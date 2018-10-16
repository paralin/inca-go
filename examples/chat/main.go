package main

import (
	"context"
	"os"

	"github.com/aperturerobotics/inca-go/examples/common"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/inca-go/utils/transaction"
	"github.com/aperturerobotics/inca-go/utils/transaction/mempool"
	"github.com/aperturerobotics/inca-go/utils/transaction/txdb"
	"github.com/aperturerobotics/inca-go/validators"
	"github.com/aperturerobotics/objstore"
	"github.com/aperturerobotics/objstore/db"
	"github.com/aperturerobotics/pbobject"
	"github.com/aperturerobotics/storageref"
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
		func(ctx context.Context) (*storageref.StorageRef, error) {
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

	// Construct the app (chat app state).
	app, err := NewChat(ctx, conf.LocalDbm)
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

	<-nod.GetProcess().Closed()
	return nil
}
