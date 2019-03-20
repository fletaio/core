package observer

import (
	"github.com/fletaio/common"
	"github.com/fletaio/core/data"
	"github.com/fletaio/core/message_def"
)

// consts
const (
	EmptyState        = iota
	RoundVoteState    = iota
	RoundVoteAckState = iota
	BlockVoteState    = iota
)

// VoteRound is data for the voting round
type VoteRound struct {
	RoundState                 int
	VoteTargetHeight           uint32
	VoteFailCount              int
	HasForwardVote             bool
	RoundVoteMessageMap        map[common.PublicHash]*RoundVoteMessage
	PublicHash                 common.PublicHash
	RoundVoteAckMessageMap     map[common.PublicHash]*RoundVoteAckMessage
	MinRoundVoteAck            *RoundVoteAck
	RoundVoteWaitMap           map[common.PublicHash]*RoundVoteMessage
	RoundVoteAckMessageWaitMap map[common.PublicHash]*RoundVoteAckMessage
	BlockRounds                []*BlockRound
	ClosedBlockRounds          []*BlockRound
}

// NewVoteRound returns a VoteRound
func NewVoteRound(TargetHeight uint32, MaxBlocksPerFormulator uint32) *VoteRound {
	vr := &VoteRound{
		RoundState:                 RoundVoteState,
		VoteTargetHeight:           TargetHeight,
		RoundVoteMessageMap:        map[common.PublicHash]*RoundVoteMessage{},
		RoundVoteAckMessageMap:     map[common.PublicHash]*RoundVoteAckMessage{},
		RoundVoteWaitMap:           map[common.PublicHash]*RoundVoteMessage{},
		RoundVoteAckMessageWaitMap: map[common.PublicHash]*RoundVoteAckMessage{},
		BlockRounds:                make([]*BlockRound, 0, MaxBlocksPerFormulator),
		ClosedBlockRounds:          make([]*BlockRound, 0, MaxBlocksPerFormulator),
	}
	for i := uint32(0); i < MaxBlocksPerFormulator; i++ {
		vr.BlockRounds = append(vr.BlockRounds, NewBlockRound(TargetHeight+i))
	}
	return vr
}

// CloseBlockRound closes a block round
func (vr *VoteRound) CloseBlockRound() {
	vr.ClosedBlockRounds = append(vr.ClosedBlockRounds, vr.BlockRounds[0])
	vr.BlockRounds = vr.BlockRounds[1:]
}

// RemoveBlockRound removes a block round
func (vr *VoteRound) RemoveBlockRound(br *BlockRound) {
	BlockRounds := make([]*BlockRound, 0, len(vr.BlockRounds))
	for _, v := range vr.BlockRounds {
		if br != v {
			BlockRounds = append(BlockRounds, v)
		}
	}
	vr.BlockRounds = BlockRounds
}

// BlockRoundCount returns a number of remained block rounds
func (vr *VoteRound) BlockRoundCount() int {
	return len(vr.BlockRounds)
}

// BlockRound is data for the block round
type BlockRound struct {
	TargetHeight            uint32
	BlockVoteMap            map[common.PublicHash]*BlockVote
	BlockGenMessage         *message_def.BlockGenMessage
	Context                 *data.Context
	BlockVoteMessageWaitMap map[common.PublicHash]*BlockVoteMessage
	BlockGenMessageWait     *message_def.BlockGenMessage
}

// NewBlockRound returns a VoteRound
func NewBlockRound(TargetHeight uint32) *BlockRound {
	vr := &BlockRound{
		TargetHeight:            TargetHeight,
		BlockVoteMap:            map[common.PublicHash]*BlockVote{},
		BlockVoteMessageWaitMap: map[common.PublicHash]*BlockVoteMessage{},
	}
	return vr
}

type voteSortItem struct {
	PublicHash common.PublicHash
	Priority   uint64
}

type voteSorter []*voteSortItem

func (s voteSorter) Len() int {
	return len(s)
}

func (s voteSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s voteSorter) Less(i, j int) bool {
	a := s[i]
	b := s[j]
	if a.Priority == b.Priority {
		return a.PublicHash.Less(b.PublicHash)
	} else {
		return a.Priority < b.Priority
	}
}
