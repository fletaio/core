package tx_account

import (
	"bytes"
	"io"

	"git.fleta.io/fleta/common"
	"git.fleta.io/fleta/common/hash"
	"git.fleta.io/fleta/common/util"
	"git.fleta.io/fleta/core/transaction"
)

// SingleAccount TODO
type SingleAccount struct {
	transaction.Base
	Seq     uint64
	From    common.Address //MAXLEN : 255
	KeyHash common.PublicHash
}

// NewSingleAccount TODO
func NewSingleAccount(coord *common.Coordinate, timestamp uint64, seq uint64) *SingleAccount {
	return &SingleAccount{
		Base: transaction.Base{
			Coordinate_: coord.Clone(),
			Timestamp_:  timestamp,
		},
		Seq: seq,
	}
}

// Hash TODO
func (tx *SingleAccount) Hash() (hash.Hash256, error) {
	var buffer bytes.Buffer
	if _, err := tx.WriteTo(&buffer); err != nil {
		return hash.Hash256{}, err
	}
	return hash.DoubleHash(buffer.Bytes()), nil
}

// WriteTo TODO
func (tx *SingleAccount) WriteTo(w io.Writer) (int64, error) {
	var wrote int64
	if n, err := tx.Base.WriteTo(w); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := util.WriteUint64(w, tx.Seq); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := tx.From.WriteTo(w); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := tx.KeyHash.WriteTo(w); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	return wrote, nil
}

// ReadFrom TODO
func (tx *SingleAccount) ReadFrom(r io.Reader) (int64, error) {
	var read int64
	if n, err := tx.Base.ReadFrom(r); err != nil {
		return read, err
	} else {
		read += n
	}
	if v, n, err := util.ReadUint64(r); err != nil {
		return read, err
	} else {
		read += n
		tx.Seq = v
	}
	if n, err := tx.From.ReadFrom(r); err != nil {
		return read, err
	} else {
		read += n
	}
	if n, err := tx.KeyHash.ReadFrom(r); err != nil {
		return read, err
	} else {
		read += n
	}
	return read, nil
}