package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aperturerobotics/hydra/cid"
	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/examples/common"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/inca-go/utils/transaction"
	"github.com/aperturerobotics/inca-go/utils/transaction/mempool"
	"github.com/aperturerobotics/inca-go/utils/transaction/txdb"
	"github.com/aperturerobotics/inca-go/validators"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)

	app := cli.NewApp()
	app.Name = "chat"
	app.Usage = "inca chat example"
	app.HideVersion = true
	app.Action = runChatExample
	app.Flags = common.Flags

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err.Error())
	}
}

func runChatExample(c *cli.Context) error {
	// Initially set proposer and validator to nil.
	conf, err := common.Build(
		context.Background(),
		nil, nil,
		func(ctx context.Context) (*cid.BlockRef, error) {
			objStore := objstore.GetObjStore(ctx)
			le := logctx.GetLogEntry(ctx)
			// Construct the initial state.
			var state ChatState
			ref, _, err := objStore.StoreObject(ctx, &state, pbobject.EncryptionConfig{})
			if err != nil {
				le.WithError(err).Error("unable to store initial state")
			} else {
				le.Info("stored initial state: %v", ref)
			}

			// Construct the mempool state wrapper
			blockState := &transaction.BlockState{
				ApplicationStateRef: ref,
			}
			ref, _, err = objStore.StoreObject(ctx, blockState, pbobject.EncryptionConfig{})
			return ref, err
		},
	)
	if err != nil {
		return err
	}

	nod := conf.Node
	ctx := conf.Context
	le := conf.LogEntry
	ch := nod.GetChain()

	// Construct a tx database.
	txdb, err := txdb.NewTxDatabase(ctx, db.WithPrefix(conf.LocalDbm, []byte("/txdb")))
	if err != nil {
		return err
	}

	// Construct the mempool.
	memPool, err := mempool.NewMempool(
		ctx,
		conf.LocalDbm,
		txdb,
		mempool.Opts{},
	)
	if err != nil {
		return err
	}

	appEncConf := &pbobject.EncryptionConfig{
		CompressionType: objectenc.CompressionType_CompressionType_SNAPPY,
	}
	stateCtx := pbobject.WithEncryptionConf(ctx, appEncConf)

	// Construct the app (chat app state).
	app, err := NewChat(stateCtx, conf.LocalDbm)
	if err != nil {
		return err
	}

	// Construct the proposer.
	proposer, err := mempool.NewProposer(
		ctx,
		memPool,
		mempool.ProposerOpts{},
		app,
	)
	if err != nil {
		return err
	}

	// Attach the proposer to the chain.
	ch.SetBlockProposer(proposer)

	// TODO: attach a "allow all" validator
	validator, err := validators.GetBuiltInValidator("allow", ch)
	if err != nil {
		return err
	}
	_ = validator

	ctx = ch.GetContext()
	chStateHandle, chStateHandleCancel := ch.SubscribeState()
	defer chStateHandleCancel()

	go func() {
		makeTxTicker := time.NewTicker(time.Second * 2)
		defer makeTxTicker.Stop()

		var msgText string
		for {
			select {
			case <-ctx.Done():
				return
			case tickTime := <-makeTxTicker.C:
				msgText = fmt.Sprintf("Hello at time %s!", tickTime.String())
			}

			ntx := &ChatTransaction{
				ChatMessage: &ChatMessage{
					MessageText: msgText,
					SenderName:  nod.GetAddrPretty(),
				},
			}

			txRef, _, err := conf.ObjStore.StoreObject(stateCtx, ntx, *appEncConf)
			if err != nil {
				le.WithError(err).Warn("unable to store transaction")
				continue
			}

			err = nod.SendMessage(
				stateCtx,
				inca.NodeMessageType_NodeMessageType_APP,
				transaction.TransactionAppMessageID,
				txRef,
			)
			if err != nil {
				le.WithError(err).Warn("unable to xmit transaction")
				continue
			}

			// TODO: process the app message
			// memPool.GetTransactionDb().Put(ctx, , nodeMessageRef *cid.BlockRef)
			// memPool.Enqueue()
		}
	}()

	var lastProcessedMessageRef *cid.BlockRef
	handleChatState := func(state *ChatVirtualState) {
		lastMessageRef := state.state.GetLastMessageRef()
		if lastMessageRef.IsEmpty() {
			return
		}

		if lastProcessedMessageRef != nil &&
			lastProcessedMessageRef.Equals(lastMessageRef) {
			return
		}

		var lastMessage ChatMessageElem
		if err := lastMessageRef.FollowRef(ctx, nil, &lastMessage, nil); err != nil {
			le.WithError(err).Warn("cannot fetch last message")
			return
		}

		le.
			WithField("total-messages", state.state.GetMessageCount()).
			WithField("last-message", lastMessage.GetChatMessage().GetMessageText()).
			WithField("last-message-sender", lastMessage.GetChatMessage().GetSenderName()).
			Info("message retrieved")
	}

	for {
		select {
		case <-nod.GetProcess().Closed():
			return nil
		case snap := <-chStateHandle:
			// Get last block.
			lastBlock, err := ch.GetBlock(snap.LastBlockRef)
			if err != nil {
				le.WithError(err).Warn("unable to get head block")
				continue
			}
			if lastBlock == nil {
				continue
			}

			// Process block.
			var blkState transaction.BlockState
			stateRef := lastBlock.GetStateRef()
			if err := stateRef.FollowRef(stateCtx, nil, &blkState, nil); err != nil {
				le.WithError(err).Warn("unable to follow state reference")
				continue
			}

			appState, err := app.GetStateAtBlock(stateCtx, lastBlock, &blkState)
			if err != nil {
				le.WithError(err).Warn("unable to resolve app state")
				continue
			}

			chatVs := appState.(*ChatVirtualState)
			handleChatState(chatVs)
		}
	}
}
