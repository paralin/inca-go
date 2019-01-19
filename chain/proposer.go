package chain

import (
	"bytes"
	"context"

	hblock "github.com/aperturerobotics/hydra/block"
	"github.com/aperturerobotics/hydra/cid"
	hobject "github.com/aperturerobotics/hydra/object"
	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/block"
	"github.com/aperturerobotics/inca-go/chain/state"
	"github.com/aperturerobotics/inca-go/peer"
	ichain "github.com/aperturerobotics/inca/chain"
	"github.com/aperturerobotics/timestamp"
	"github.com/pkg/errors"

	lpeer "github.com/aperturerobotics/bifrost/peer"
	"github.com/libp2p/go-libp2p-crypto"

	"github.com/golang/protobuf/proto"
	"github.com/sirupsen/logrus"
)

// Proposer controls proposing new blocks on a Chain.
type Proposer struct {
	ctx         context.Context
	state       ichain.ProposerState
	ch          *Chain
	le          *logrus.Entry
	privKey     crypto.PrivKey
	validatorID lpeer.ID
	pubKeyBytes []byte
	store       hobject.ObjectStore

	msgSender Node
	peerStore *peer.PeerStore
}

// NewProposer builds a new Proposer.
func NewProposer(
	ctx context.Context,
	le *logrus.Entry,
	privKey crypto.PrivKey,
	store hobject.ObjectStore,
	ch *Chain,
	sender Node,
	peerStore *peer.PeerStore,
) (*Proposer, error) {
	le = le.WithField("c", "proposer")
	pid, err := lpeer.IDFromPrivateKey(privKey)
	if err != nil {
		return nil, err
	}

	store = hobject.NewPrefixer(store, "proposer/"+pid.Pretty()+"/")
	p := &Proposer{
		ctx:       ctx,
		ch:        ch,
		le:        le,
		privKey:   privKey,
		msgSender: sender,
		peerStore: peerStore,
		store:     store,
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

	return p, nil
}

// readState reads the state from the database.
// Note: the state object must be allocated, and the ID set.
// If the key does not exist nothing happens.
func (p *Proposer) readState(ctx context.Context) error {
	dat, datOk, err := p.store.GetObject("state")
	if err != nil {
		return err
	}

	if !datOk {
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

	return p.store.SetObject("state", dat)
}

// makeProposal makes a proposal for the given state.
func (p *Proposer) makeProposal(ctx context.Context, state *state.ChainStateSnapshot) (*cid.BlockRef, *inca.BlockHeader, error) {
	nowTs := timestamp.Now()
	blockProposer := p.ch.GetBlockProposer()

	if blockProposer == nil {
		p.le.Debug("proposer is nil, abstaining")
		return nil, nil, nil
	}

	_, lastBlockCursor := p.ch.rootCursor.BuildTransactionAtRef(nil, state.LastBlockRef)
	lastBlock, err := block.GetBlock(
		ctx,
		lastBlockCursor,
		p.ch.GetBlockDbm(),
	)
	if err != nil {
		return nil, nil, err
	}

	chainConfRef := lastBlock.GetHeader().GetChainConfigRef()
	chainConfCursor, err := lastBlockCursor.FollowRef(2, chainConfRef)
	if err != nil {
		return nil, nil, err
	}
	chainConfi, err := chainConfCursor.Unmarshal(func() hblock.Block {
		return &inca.ChainConfig{}
	})
	if err != nil {
		return nil, nil, err
	}
	chainConf := chainConfi.(*inca.ChainConfig)
	vsetRef := chainConf.GetValidatorSetRef()
	vsetCursor, err := chainConfCursor.FollowRef(2, vsetRef)
	if err != nil {
		return nil, nil, err
	}
	vseti, err := vsetCursor.Unmarshal(func() hblock.Block {
		return &inca.ValidatorSet{}
	})
	if err != nil {
		return nil, nil, err
	}
	vset := vseti.(*inca.ValidatorSet)
	vsetIndex := -1
	for vi, validator := range vset.GetValidators() {
		pubKey, err := crypto.UnmarshalPublicKey(validator.GetPubKey())
		if err != nil {
			return nil, nil, err
		}
		if bytes.Compare(p.pubKeyBytes, validator.GetPubKey()) == 0 {
			vsetIndex = vi
		}
	}
	if vsetIndex == -1 {
		return nil, nil, errors.New("peer id not found in validator set")
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
		PrevBlockRef:       state.LastBlockRef,
		RoundInfo: &inca.BlockRoundInfo{
			Height: state.BlockRoundInfo.GetHeight(),
			Round:  state.BlockRoundInfo.GetRound(),
		},
		BlockTs:        &nowTs,
		ValidatorIndex: uint32(vsetIndex),
		StateRef:       proposedState,
	}

	tx, blkHeaderCursor := p.ch.rootCursor.BuildTransaction(nil)
	blkHeaderCursor.SetBlock(blockHeader)
	eves, blkHeaderCursor, err := tx.Write()
	if err != nil {
		return nil, nil, err
	}
	blockHeaderRef := eves[len(eves)-1].GetPutBlock().GetBlockCommon().GetBlockRef()

	return blockHeaderRef, blockHeader, err
}

// makeVoe sends the vote for a block.
func (p *Proposer) makeVote(ctx context.Context, blockHeaderRef *cid.BlockRef) error {
	vote := &inca.Vote{BlockHeaderRef: blockHeaderRef}
	voteTx, voteCursor := p.ch.rootCursor.BuildTransaction(nil)
	voteCursor.SetBlock(vote)
	eves, voteCursor, err := voteTx.Write()
	if err != nil {
		return err
	}
	voteStorageRef := eves[len(eves)-1].GetPutBlock().GetBlockCommon().GetBlockRef()
	return p.msgSender.SendMessage(
		p.ctx,
		inca.NodeMessageType_NodeMessageType_VOTE,
		0,
		voteStorageRef,
	)
}

type confirmedVote struct {
	peer    *peer.Peer
	vote    *inca.Vote
	voteRef *cid.BlockRef
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
		if err != nil && err != context.Canceled {
			p.le.WithError(err).Error("unable to make proposal")
		}

		if blockHeaderRef == nil {
			p.le.
				WithField("round-height", nextState.BlockRoundInfo.String()).
				Debug("proposer abstains this round")
			continue
		}

		// Wait for the proposal to be signed by enough voting power.
		requiredVotingPower := int32(float32(nextState.TotalVotingPower) * block.BlockCommitRatio)
		p.le.
			WithField("height-round", nextState.BlockRoundInfo.String()).
			WithField("total-voting-power", nextState.TotalVotingPower).
			WithField("required-voting-power", requiredVotingPower).
			Debug("waiting for votes")

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
			// Note: waitForValidatorVote starts a goroutine.
			p.waitForValidatorVote(
				stateCtx,
				peer,
				blockHeaderRef,
				int32(validator.GetVotingPower()),
				votingPowerAccum,
			)
		}

		if err := p.makeVote(stateCtx, blockHeaderRef); err != nil {
			p.le.WithError(err).Error("unable to make vote for own proposal")
			return err
		}

		var confirmedVotes []*cid.BlockRef
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
		var blockStorageRef *cid.BlockRef
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
func (p *Proposer) waitForValidatorVote(
	ctx context.Context,
	pr *peer.Peer,
	blockHeaderRef *cid.BlockRef,
	power int32,
	votingPowerAccum chan<- *confirmedVote,
) {
	le := p.le.WithField("peer", pr.GetPeerID().Pretty())
	nodeMessages, nodeMessagesCancel := pr.SubscribeMessages()

	go func() {
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
				// innerRef.FollowRef(subCtx, innerRef.GetObjectDigest(), vote
				// TODO

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
	}()
}
