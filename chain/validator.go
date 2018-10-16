package chain

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/chain/state"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/inca-go/peer"
	ichain "github.com/aperturerobotics/inca/chain"

	"github.com/aperturerobotics/objstore"
	"github.com/aperturerobotics/objstore/db"
	"github.com/aperturerobotics/pbobject"
	"github.com/aperturerobotics/storageref"

	"github.com/golang/protobuf/proto"
	"github.com/libp2p/go-libp2p-crypto"
	lpeer "github.com/libp2p/go-libp2p-peer"
	"github.com/sirupsen/logrus"
)

// Validator validates incoming block commit messages.
type Validator struct {
	ctx context.Context

	state       ichain.ValidatorState
	ch          *Chain
	le          *logrus.Entry
	dbm         db.Db
	privKey     crypto.PrivKey
	pubKeyBytes []byte
	objStore    *objstore.ObjectStore

	msgSender Node
	peerStore *peer.PeerStore
}

// NewValidator builds a new Validator.
func NewValidator(
	ctx context.Context,
	privKey crypto.PrivKey,
	dbm db.Db,
	ch *Chain,
	sender Node,
	peerStore *peer.PeerStore,
) (*Validator, error) {
	le := logctx.GetLogEntry(ctx).WithField("c", "validator")
	pid, err := lpeer.IDFromPrivateKey(privKey)
	if err != nil {
		return nil, err
	}

	dbm = db.WithPrefix(dbm, []byte(fmt.Sprintf("/validator/%s", pid.Pretty())))
	p := &Validator{
		ctx:       ctx,
		ch:        ch,
		le:        le,
		dbm:       dbm,
		privKey:   privKey,
		msgSender: sender,
		peerStore: peerStore,
	}
	p.pubKeyBytes, err = privKey.GetPublic().Bytes()
	if err != nil {
		return nil, err
	}
	if err := p.readState(ctx); err != nil {
		return nil, err
	}
	if err := p.writeState(ctx); err != nil {
		return nil, err
	}

	p.objStore = objstore.GetObjStore(ctx)
	if p.objStore == nil {
		return nil, errors.New("object store must be specified in ctx")
	}

	return p, nil
}

// readState reads the state from the database.
// Note: the state object must be allocated, and the ID set.
// If the key does not exist nothing happens.
func (p *Validator) readState(ctx context.Context) error {
	dat, datOk, err := p.dbm.Get(ctx, []byte("/state"))
	if err != nil || !datOk {
		return err
	}

	return proto.Unmarshal(dat, &p.state)
}

// writeState writes the state to the database.
func (p *Validator) writeState(ctx context.Context) error {
	dat, err := proto.Marshal(&p.state)
	if err != nil {
		return err
	}

	return p.dbm.Set(ctx, []byte("/state"), dat)
}

// makeVote makes a vote for the given state.
func (p *Validator) makeVote(ctx context.Context, state *state.ChainStateSnapshot) (*storageref.StorageRef, error) {
	proposer := state.CurrentProposer
	_, pubKey, err := proposer.ParsePeerID()
	if err != nil {
		return nil, err
	}

	proposerPeer, err := p.peerStore.GetPeerWithPubKey(pubKey)
	if err != nil {
		return nil, err
	}

	msg, msgCancel := proposerPeer.SubscribeMessages()
	defer msgCancel()

	var blockHeaderRef *storageref.StorageRef
	var peerMsg *inca.NodeMessage
	for {
		select {
		case <-ctx.Done():
			return nil, nil
		case peerMsg = <-msg:
		}

		if peerMsg.GetMessageType() != inca.NodeMessageType_NodeMessageType_VOTE {
			continue
		}

		vote := &inca.Vote{}
		voteRef := peerMsg.GetInnerRef()
		encConf := p.ch.
			GetEncryptionStrategy().
			GetBlockEncryptionConfigWithDigest(voteRef.GetObjectDigest())
		encCtx := pbobject.WithEncryptionConf(ctx, &encConf)
		if err := voteRef.FollowRef(encCtx, nil, vote, nil); err != nil {
			return nil, err
		}

		blockHeaderRef = vote.GetBlockHeaderRef()
		blockHeader := &inca.BlockHeader{}
		encConf = p.ch.
			GetEncryptionStrategy().
			GetBlockEncryptionConfigWithDigest(blockHeaderRef.GetObjectDigest())
		encCtx = pbobject.WithEncryptionConf(ctx, &encConf)
		if err := blockHeaderRef.FollowRef(encCtx, nil, blockHeader, nil); err != nil {
			return nil, err
		}

		if blockHeader.GetRoundInfo().GetHeight() != state.BlockRoundInfo.GetHeight() ||
			blockHeader.GetRoundInfo().GetRound() != state.BlockRoundInfo.GetRound() {
			p.le.
				WithField("round", blockHeader.GetRoundInfo().String()).
				Warn("skipping vote for wrong round")
			continue
		}

		if !blockHeader.GetLastBlockRef().Equals(state.LastBlockRef) {
			continue
		}

		// TODO: validate block header
		break
	}

	var voteStorageRef *storageref.StorageRef
	{
		vote := &inca.Vote{BlockHeaderRef: blockHeaderRef}
		{
			encConf := p.ch.GetEncryptionStrategy().GetNodeMessageEncryptionConfig(p.privKey)
			subCtx := pbobject.WithEncryptionConf(ctx, &encConf)
			sr, _, err := p.objStore.StoreObject(subCtx, vote, encConf)
			if err != nil {
				return nil, err
			}
			voteStorageRef = sr
		}

		err := p.msgSender.SendMessage(
			p.ctx,
			inca.NodeMessageType_NodeMessageType_VOTE,
			0,
			voteStorageRef,
		)
		if err != nil {
			return nil, err
		}
	}

	p.le.WithField("height-round", state.BlockRoundInfo.String()).Info("voted for height/round")
	return voteStorageRef, nil
}

// ManageValidator manages the validator lifecycle.
func (p *Validator) ManageValidator(ctx context.Context) error {
	p.le.Debug("validator running")
	defer p.le.Debug("validator shut down")

	chStateCh, chStateCancel := p.ch.SubscribeState()
	defer chStateCancel()

	for {
		var nextState state.ChainStateSnapshot
		select {
		case <-ctx.Done():
			return ctx.Err()
		case nextState = <-chStateCh:
		}

		if err := p.processStateSnapshot(nextState.Context, &nextState); err != nil {
			p.le.WithError(err).Error("skipping round")
		}
	}
}

// processStateSnapshot processes a state.
func (p *Validator) processStateSnapshot(ctx context.Context, nextState *state.ChainStateSnapshot) error {
	if nextState.CurrentProposer == nil {
		return nil
	}

	now := time.Now()
	if nextState.RoundStartTime.After(now) {
		return nil
	}

	lastHeight := p.state.GetLastVote().GetHeight()
	currHeight := nextState.BlockRoundInfo.GetHeight()
	if p.state.GetLastVote() != nil && lastHeight > currHeight {
		return nil
	}

	lastRound := p.state.GetLastVote().GetRound()
	currRound := nextState.BlockRoundInfo.GetRound()
	if p.state.GetLastVote() != nil && lastRound >= currRound {
		return nil
	}

	p.state.LastVote = nextState.BlockRoundInfo
	if err := p.writeState(ctx); err != nil {
		p.le.WithError(err).Error("unable to write state, skipping round")
		return nil
	}

	proposerPubKey, err := nextState.CurrentProposer.ParsePublicKey()
	if err != nil {
		p.le.WithError(err).Error("proposer public key failed to parse")
		return nil
	}

	if !proposerPubKey.Equals(p.privKey.GetPublic()) {
		// Check if we are a validator
		weAreValidator := false
		for _, validator := range nextState.ValidatorSet.GetValidators() {
			if bytes.Compare(p.pubKeyBytes, validator.GetPubKey()) == 0 {
				weAreValidator = true
				break
			}
		}

		if weAreValidator {
			_, err := p.makeVote(ctx, nextState)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
