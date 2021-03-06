package observer

import (
	"io"

	"github.com/fletaio/common"
	"github.com/fletaio/common/hash"
	"github.com/fletaio/common/util"
	"github.com/fletaio/framework/chain"
)

type RoundVote struct {
	ChainCoord           *common.Coordinate
	LastHash             hash.Hash256
	VoteTargetHeight     uint32
	TimeoutCount         uint32
	Formulator           common.Address
	FormulatorPublicHash common.PublicHash
	Timestamp            uint64
	IsReply              bool
}

// Hash returns the hash value of it
func (vt *RoundVote) Hash() hash.Hash256 {
	return hash.DoubleHashByWriterTo(vt)
}

// WriteTo is a serialization function
func (vt *RoundVote) WriteTo(w io.Writer) (int64, error) {
	var wrote int64
	if n, err := vt.ChainCoord.WriteTo(w); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := vt.LastHash.WriteTo(w); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := util.WriteUint32(w, vt.VoteTargetHeight); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := util.WriteUint32(w, vt.TimeoutCount); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := vt.Formulator.WriteTo(w); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := vt.FormulatorPublicHash.WriteTo(w); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := util.WriteUint64(w, vt.Timestamp); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := util.WriteBool(w, vt.IsReply); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	return wrote, nil
}

// ReadFrom is a deserialization function
func (vt *RoundVote) ReadFrom(r io.Reader) (int64, error) {
	var read int64
	if n, err := vt.ChainCoord.ReadFrom(r); err != nil {
		return read, err
	} else {
		read += n
	}
	if n, err := vt.LastHash.ReadFrom(r); err != nil {
		return read, err
	} else {
		read += n
	}
	if v, n, err := util.ReadUint32(r); err != nil {
		return read, err
	} else {
		read += n
		vt.VoteTargetHeight = v
	}
	if v, n, err := util.ReadUint32(r); err != nil {
		return read, err
	} else {
		read += n
		vt.TimeoutCount = v
	}
	if n, err := vt.Formulator.ReadFrom(r); err != nil {
		return read, err
	} else {
		read += n
	}
	if n, err := vt.FormulatorPublicHash.ReadFrom(r); err != nil {
		return read, err
	} else {
		read += n
	}
	if v, n, err := util.ReadUint64(r); err != nil {
		return read, err
	} else {
		read += n
		vt.Timestamp = v
	}
	if v, n, err := util.ReadBool(r); err != nil {
		return read, err
	} else {
		read += n
		vt.IsReply = v
	}
	return read, nil
}

// RoundVoteAck is a message for a round vote ack
type RoundVoteAck struct {
	VoteTargetHeight     uint32
	TimeoutCount         uint32
	Formulator           common.Address
	FormulatorPublicHash common.PublicHash
	PublicHash           common.PublicHash
	Timestamp            uint64
	IsReply              bool
}

// Hash returns the hash value of it
func (vt *RoundVoteAck) Hash() hash.Hash256 {
	return hash.DoubleHashByWriterTo(vt)
}

// WriteTo is a serialization function
func (vt *RoundVoteAck) WriteTo(w io.Writer) (int64, error) {
	var wrote int64
	if n, err := util.WriteUint32(w, vt.VoteTargetHeight); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := util.WriteUint32(w, vt.TimeoutCount); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := vt.Formulator.WriteTo(w); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := vt.FormulatorPublicHash.WriteTo(w); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := vt.PublicHash.WriteTo(w); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := util.WriteUint64(w, vt.Timestamp); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := util.WriteBool(w, vt.IsReply); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	return wrote, nil
}

// ReadFrom is a deserialization function
func (vt *RoundVoteAck) ReadFrom(r io.Reader) (int64, error) {
	var read int64
	if v, n, err := util.ReadUint32(r); err != nil {
		return read, err
	} else {
		read += n
		vt.VoteTargetHeight = v
	}
	if v, n, err := util.ReadUint32(r); err != nil {
		return read, err
	} else {
		read += n
		vt.TimeoutCount = v
	}
	if n, err := vt.Formulator.ReadFrom(r); err != nil {
		return read, err
	} else {
		read += n
	}
	if n, err := vt.FormulatorPublicHash.ReadFrom(r); err != nil {
		return read, err
	} else {
		read += n
	}
	if n, err := vt.PublicHash.ReadFrom(r); err != nil {
		return read, err
	} else {
		read += n
	}
	if v, n, err := util.ReadUint64(r); err != nil {
		return read, err
	} else {
		read += n
		vt.Timestamp = v
	}
	if v, n, err := util.ReadBool(r); err != nil {
		return read, err
	} else {
		read += n
		vt.IsReply = v
	}
	return read, nil
}

// BlockVote is message for a block vote
type BlockVote struct {
	VoteTargetHeight   uint32
	Header             chain.Header
	GeneratorSignature common.Signature
	ObserverSignature  common.Signature
	IsReply            bool
}

// Hash returns the hash value of it
func (vt *BlockVote) Hash() hash.Hash256 {
	return hash.DoubleHashByWriterTo(vt)
}

// WriteTo is a serialization function
func (vt *BlockVote) WriteTo(w io.Writer) (int64, error) {
	var wrote int64
	if n, err := util.WriteUint32(w, vt.VoteTargetHeight); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := vt.Header.WriteTo(w); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := vt.GeneratorSignature.WriteTo(w); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := vt.ObserverSignature.WriteTo(w); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := util.WriteBool(w, vt.IsReply); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	return wrote, nil
}

// ReadFrom is a deserialization function
func (vt *BlockVote) ReadFrom(r io.Reader) (int64, error) {
	var read int64
	if v, n, err := util.ReadUint32(r); err != nil {
		return read, err
	} else {
		read += n
		vt.VoteTargetHeight = v
	}
	if n, err := vt.Header.ReadFrom(r); err != nil {
		return read, err
	} else {
		read += n
	}
	if n, err := vt.GeneratorSignature.ReadFrom(r); err != nil {
		return read, err
	} else {
		read += n
	}
	if n, err := vt.ObserverSignature.ReadFrom(r); err != nil {
		return read, err
	} else {
		read += n
	}
	if v, n, err := util.ReadBool(r); err != nil {
		return read, err
	} else {
		read += n
		vt.IsReply = v
	}
	return read, nil
}
