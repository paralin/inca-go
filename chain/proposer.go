package chain

import (
	"bytes"
	"context"
	"fmt"

	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/block"
	"github.com/aperturerobotics/inca-go/chain/state"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/inca-go/peer"
	ichain "github.com/aperturerobotics/inca/chain"
	"github.com/aperturerobotics/objstore"
	"github.com/aperturerobotics/objstore/db"
	"github.com/aperturerobotics/pbobject"
	"github.com/aperturerobotics/storageref"
	"github.com/aperturerobotics/timestamp"

	"github.com/libp2p/go-libp2p-crypto"
	lpeer "github.com/libp2p/go-libp2p-peer"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Proposer controls proposing new blocks on a Chain.
type Proposer struct {
	ctx         context.Context
	state       ichain.ProposerState
	ch          *Chain
	le          *logrus.Entry
	dbm         db.Db
	privKey     crypto.PrivKey
	validatorID lpeer.ID
	pubKeyBytes []byte
	objStore    *objstore.ObjectStore

	msgSender Node
	peerStore *peer.PeerStore
}

// NewProposer builds a new Proposer.
func NewProposer(
	ctx context.Context,
	privKey crypto.PrivKey,
	dbm db.Db,
	ch *Chain,
	sender Node,
	peerStore *peer.PeerStore,
) (*Proposer, error) {
	ch.GetBlockValidator()
	le := logctx.GetLogEntry(ctx).WithField("c", "proposer")
	pid, err := lpeer.IDFromPrivateKey(privKey)
	if err != nil {
		return nil, err
	}

	dbm = db.WithPrefix(dbm, []byte(fmt.Sprintf("/proposer/%s", pid.Pretty())))
	p := &Proposer{
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
	p.validatorID, err = lpeer.IDFromPrivateKey(privKey)
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
func (p *Proposer) readState(ctx context.Context) error {
	dat, err := p.dbm.Get(ctx, []byte("/state"))
	if err != nil {
		return err
	}

	if len(dat) == 0 {
		return nil
	}

	return proto.Unmarshal(dat, &p.state)
}

// writeState writes the state to the database.
func (p *Proposer) writeState(ctx context.Context) error {
	dat, err := proto.Marshal(&p.state)
	if err != nil {
		return err
	}

	return p.dbm.Set(ctx, []byte("/state"), dat)
}

// makeProposal makes a proposal for the given state.
func (p *Proposer) makeProposal(ctx context.Context, state *state.ChainStateSnapshot) (*storageref.StorageRef, *inca.BlockHeader, error) {
	nowTs := timestamp.Now()
	blockProposer := p.ch.GetBlockProposer()

	if blockProposer == nil {
		p.le.Debug("proposer is nil, abstaining")
		return nil, nil, nil
	}

	lastBlock, err := block.GetBlock(
		ctx,
		p.ch.GetEncryptionStrategy(),
		p.ch.GetBlockValidator(),
		p.ch.GetBlockDbm(),
		state.LastBlockRef,
	)
	if err != nil {
		return nil, nil, err
	}

	proposedState, err := blockProposer.ProposeBlock(ctx, lastBlock, state)
	if err != nil {
		return nil, nil, err
	}

	if proposedState == nil {
		return nil, nil, nil
	}

	blockHeader := &inca.BlockHeader{
		GenesisRef:         p.ch.GetGenesisRef(),
		ChainConfigRef:     state.LastBlockHeader.GetChainConfigRef(),
		NextChainConfigRef: state.LastBlockHeader.GetChainConfigRef(),
		LastBlockRef:       state.LastBlockRef,
		RoundInfo: &inca.BlockRoundInfo{
			Height: state.BlockRoundInfo.GetHeight(),
			Round:  state.BlockRoundInfo.GetRound(),
		},
		BlockTs:    &nowTs,
		ProposerId: p.validatorID.Pretty(),
		StateRef:   proposedState,
	}

	var blockHeaderRef *storageref.StorageRef
	{
		encConf := p.ch.GetEncryptionStrategy().GetBlockEncryptionConfig()
		subCtx := pbobject.WithEncryptionConf(ctx, &encConf)
		sr, _, err := p.objStore.StoreObject(subCtx, blockHeader, encConf)
		if err != nil {
			return nil, nil, err
		}
		blockHeaderRef = sr
	}

	vote := &inca.Vote{BlockHeaderRef: blockHeaderRef}
	var voteStorageRef *storageref.StorageRef
	{
		encConf := p.ch.GetEncryptionStrategy().GetNodeMessageEncryptionConfig(p.privKey)
		subCtx := pbobject.WithEncryptionConf(ctx, &encConf)
		sr, _, err := p.objStore.StoreObject(subCtx, vote, encConf)
		if err != nil {
			return nil, nil, err
		}
		voteStorageRef = sr
	}

	err = p.msgSender.SendMessage(
		p.ctx,
		inca.NodeMessageType_NodeMessageType_VOTE,
		0,
		voteStorageRef,
	)
	return blockHeaderRef, blockHeader, err
}

type confirmedVote struct {
	peer    *peer.Peer
	vote    *inca.Vote
	voteRef *storageref.StorageRef
	power   int32
}

// ManageProposer manages the proposer lifecycle.
func (p *Proposer) ManageProposer(ctx context.Context) error {
	p.le.Debug("proposer running")
	defer p.le.Debug("proposer shut down")

	chStateCh, chStateCancel := p.ch.SubscribeState()
	defer chStateCancel()

StateLoop:
	for {
		var nextState state.ChainStateSnapshot
		select {
		case <-ctx.Done():
			return ctx.Err()
		case nextState = <-chStateCh:
		}

		stateCtx := nextState.Context
		if nextState.CurrentProposer == nil {
			continue
		}

		if !nextState.RoundStarted {
			continue
		}

		if bytes.Compare(nextState.CurrentProposer.GetPubKey(), p.pubKeyBytes) != 0 {
			continue
		}

		lastHeight := p.state.GetLastProposal().GetHeight()
		currHeight := nextState.BlockRoundInfo.GetHeight()
		if p.state.GetLastProposal() != nil && lastHeight > currHeight {
			continue
		}

		lastRound := p.state.GetLastProposal().GetRound()
		currRound := nextState.BlockRoundInfo.GetRound()
		if p.state.GetLastProposal() != nil && lastHeight == currHeight && lastRound >= currRound {
			continue
		}

		p.state.LastProposal = nextState.BlockRoundInfo
		if err := p.writeState(stateCtx); err != nil {
			p.le.WithError(err).Error("unable to write state, skipping proposal")
			continue
		}

		p.le.
			WithField("round-height", nextState.BlockRoundInfo.String()).
			Debug("proposing this round")

		// make the proposal
		blockHeaderRef, _, err := p.makeProposal(stateCtx, &nextState)
		if err != nil {
			p.le.WithError(err).Error("unable to make proposal")
		}

		if blockHeaderRef == nil {
			p.le.
				WithField("round-height", nextState.BlockRoundInfo.String()).
				Debug("proposer decided to abstain this round")
			continue
		}

		// Wait for the proposal to be signed by enough voting power.
		requiredVotingPower := int32(float32(nextState.TotalVotingPower) * block.BlockCommitRatio)
		p.le.
			WithField("height-round", nextState.BlockRoundInfo.String()).
			WithField("total-voting-power", nextState.TotalVotingPower).
			WithField("required-voting-power", requiredVotingPower).
			Info("waiting for votes")

		votingPowerAccum := make(chan *confirmedVote)
		for _, validator := range nextState.ValidatorSet.GetValidators() {
			if validator.GetOperationMode() != inca.Validator_OperationMode_OPERATING {
				continue
			}

			validatorID, validatorPubKey, err := validator.ParsePeerID()
			le := p.le.WithField("peer", validatorID.Pretty())
			if err != nil {
				le.WithError(err).Error("validator public key failed to parse")
				continue
			}

			peer, err := p.peerStore.GetPeerWithPubKey(validatorPubKey)
			if err != nil {
				le.WithError(err).Error("unable to get peer for validator")
				continue
			}

			// Wait for the vote from the validator.
			go p.waitForValidatorVote(stateCtx, peer, blockHeaderRef, int32(validator.GetVotingPower()), votingPowerAccum)
		}

		var confirmedVotes []*storageref.StorageRef
		for {
			var currVoteStanding int32
			select {
			case <-stateCtx.Done():
				continue StateLoop
			case confVote := <-votingPowerAccum:
				currVoteStanding += confVote.power
				confirmedVotes = append(confirmedVotes, confVote.voteRef)
			}

			if currVoteStanding >= requiredVotingPower {
				break
			}
		}

		// Mint the final block
		blk := &inca.Block{BlockHeaderRef: blockHeaderRef, VoteRefs: confirmedVotes}
		var blockStorageRef *storageref.StorageRef
		{
			encConf := p.ch.GetEncryptionStrategy().GetBlockEncryptionConfig()
			subCtx := pbobject.WithEncryptionConf(stateCtx, &encConf)
			sr, _, err := p.objStore.StoreObject(subCtx, blk, encConf)
			if err != nil {
				p.le.WithError(err).Error("cannot store final block object")
				continue StateLoop
			}
			blockStorageRef = sr
		}

		if err := p.msgSender.SendMessage(
			stateCtx,
			inca.NodeMessageType_NodeMessageType_BLOCK_COMMIT,
			0,
			blockStorageRef,
		); err != nil {
			p.le.WithError(err).Error("unable to send block commit")
		}

		p.le.Info("committed block")
	}
}

// waitForValidatorVote waits for a validator vote from a peer.
func (p *Proposer) waitForValidatorVote(ctx context.Context, pr *peer.Peer, blockHeaderRef *storageref.StorageRef, power int32, votingPowerAccum chan<- *confirmedVote) {
	le := p.le.WithField("peer", pr.GetPeerID().Pretty())
	nodeMessages, nodeMessagesCancel := pr.SubscribeMessages()
	defer nodeMessagesCancel()

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-nodeMessages:
			if msg.GetMessageType() != inca.NodeMessageType_NodeMessageType_VOTE {
				continue
			}

			vote := &inca.Vote{}
			innerRef := msg.GetInnerRef()
			encConf := p.ch.GetEncryptionStrategy().GetBlockEncryptionConfigWithDigest(innerRef.GetObjectDigest())
			subCtx := pbobject.WithEncryptionConf(ctx, &encConf)
			if err := innerRef.FollowRef(subCtx, innerRef.GetObjectDigest(), vote, nil); err != nil {
				le.WithError(err).Warn("unable to follow inner message ref")
				continue
			}

			conf := &confirmedVote{
				peer:    pr,
				vote:    vote,
				power:   power,
				voteRef: innerRef,
			}
			select {
			case <-ctx.Done():
			case votingPowerAccum <- conf:
			}
			return
		}
	}
}
