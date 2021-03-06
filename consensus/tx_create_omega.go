package consensus

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/fletaio/common"
	"github.com/fletaio/common/hash"
	"github.com/fletaio/common/util"
	"github.com/fletaio/core/amount"
	"github.com/fletaio/core/data"
	"github.com/fletaio/core/transaction"
)

func init() {
	data.RegisterTransaction("consensus.CreateOmega", func(t transaction.Type) transaction.Transaction {
		return &CreateOmega{
			Base: transaction.Base{
				Type_: t,
			},
		}
	}, func(loader data.Loader, t transaction.Transaction, signers []common.PublicHash) error {
		tx := t.(*CreateOmega)
		if tx.Seq() <= loader.Seq(tx.From()) {
			return ErrInvalidSequence
		}

		acc, err := loader.Account(tx.From())
		if err != nil {
			return err
		}
		frAcc, is := acc.(*FormulationAccount)
		if !is {
			return ErrInvalidAccountType
		}
		if frAcc.FormulationType != SigmaFormulatorType {
			return ErrInvalidAccountType
		}

		if err := loader.Accounter().Validate(loader, frAcc, signers); err != nil {
			return err
		}

		hasFrom := false
		for _, addr := range tx.SigmaFormulators {
			if addr.Equal(tx.From()) {
				hasFrom = true
			}
			if acc, err := loader.Account(addr); err != nil {
				return err
			} else if facc, is := acc.(*FormulationAccount); !is {
				return ErrInvalidAccountType
			} else if facc.FormulationType != SigmaFormulatorType {
				return ErrInvalidAccountType
			} else {
				if err := loader.Accounter().Validate(loader, facc, signers); err != nil {
					return err
				}
			}
		}
		if !hasFrom {
			return ErrInvalidFormulatorCount
		}
		return nil
	}, func(ctx *data.Context, Fee *amount.Amount, t transaction.Transaction, coord *common.Coordinate) (ret interface{}, rerr error) {
		tx := t.(*CreateOmega)

		policy, has := gConsensusPolicyMap[ctx.ChainCoord().ID()]
		if !has {
			return nil, ErrNotExistConsensusPolicy
		}
		if len(tx.SigmaFormulators) != int(policy.OmegaRequiredSigmaCount) {
			return nil, ErrInvalidFormulatorCount
		}

		sn := ctx.Snapshot()
		defer ctx.Revert(sn)

		if tx.Seq() != ctx.Seq(tx.From())+1 {
			return nil, ErrInvalidSequence
		}
		ctx.AddSeq(tx.From())

		acc, err := ctx.Account(tx.From())
		if err != nil {
			return nil, err
		}
		frAcc, is := acc.(*FormulationAccount)
		if !is {
			return nil, ErrInvalidAccountType
		}
		if frAcc.FormulationType != SigmaFormulatorType {
			return nil, ErrInvalidAccountType
		}

		hasFrom := true
		for _, addr := range tx.SigmaFormulators {
			if addr.Equal(tx.From()) {
				hasFrom = true
			}
			if acc, err := ctx.Account(addr); err != nil {
				return nil, err
			} else if subAcc, is := acc.(*FormulationAccount); !is {
				return nil, ErrInvalidAccountType
			} else if subAcc.FormulationType != SigmaFormulatorType {
				return nil, ErrInvalidAccountType
			} else {
				if ctx.TargetHeight() < addr.Coordinate().Height+policy.OmegaRequiredSigmaBlocks {
					return nil, ErrInsufficientFormulatorBlocks
				}
				if !addr.Equal(frAcc.Address()) {
					frAcc.Amount = frAcc.Amount.Add(subAcc.Amount)
					frAcc.AddBalance(subAcc.Balance())
					ctx.DeleteAccount(subAcc)
				}
			}
		}
		if !hasFrom {
			return nil, ErrInvalidFormulatorCount
		}

		ctx.Commit(sn)
		return nil, nil
	})
}

// CreateOmega is a consensus.CreateOmega
// It is used to make formulation account
type CreateOmega struct {
	transaction.Base
	Seq_             uint64
	From_            common.Address
	SigmaFormulators []common.Address
}

// IsUTXO returns false
func (tx *CreateOmega) IsUTXO() bool {
	return false
}

// From returns the creator of the transaction
func (tx *CreateOmega) From() common.Address {
	return tx.From_
}

// Seq returns the sequence of the transaction
func (tx *CreateOmega) Seq() uint64 {
	return tx.Seq_
}

// Hash returns the hash value of it
func (tx *CreateOmega) Hash() hash.Hash256 {
	return hash.DoubleHashByWriterTo(tx)
}

// WriteTo is a serialization function
func (tx *CreateOmega) WriteTo(w io.Writer) (int64, error) {
	var wrote int64
	if n, err := tx.Base.WriteTo(w); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := util.WriteUint64(w, tx.Seq_); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := tx.From_.WriteTo(w); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := util.WriteUint8(w, uint8(len(tx.SigmaFormulators))); err != nil {
		return wrote, err
	} else {
		wrote += n
		for _, addr := range tx.SigmaFormulators {
			if n, err := addr.WriteTo(w); err != nil {
				return wrote, err
			} else {
				wrote += n
			}
		}
	}
	return wrote, nil
}

// ReadFrom is a deserialization function
func (tx *CreateOmega) ReadFrom(r io.Reader) (int64, error) {
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
		tx.Seq_ = v
	}
	if n, err := tx.From_.ReadFrom(r); err != nil {
		return read, err
	} else {
		read += n
	}
	if Len, n, err := util.ReadUint8(r); err != nil {
		return read, err
	} else {
		read += n
		tx.SigmaFormulators = make([]common.Address, 0, Len)
		for i := 0; i < int(Len); i++ {
			var addr common.Address
			if n, err := addr.ReadFrom(r); err != nil {
				return read, err
			} else {
				read += n
			}
			tx.SigmaFormulators = append(tx.SigmaFormulators, addr)
		}
	}
	return read, nil
}

// MarshalJSON is a marshaler function
func (tx *CreateOmega) MarshalJSON() ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteString(`{`)
	buffer.WriteString(`"type":`)
	if bs, err := json.Marshal(tx.Type_); err != nil {
		return nil, err
	} else {
		buffer.Write(bs)
	}
	buffer.WriteString(`,`)
	buffer.WriteString(`"timestamp":`)
	if bs, err := json.Marshal(tx.Timestamp_); err != nil {
		return nil, err
	} else {
		buffer.Write(bs)
	}
	buffer.WriteString(`,`)
	buffer.WriteString(`"seq":`)
	if bs, err := json.Marshal(tx.Seq_); err != nil {
		return nil, err
	} else {
		buffer.Write(bs)
	}
	buffer.WriteString(`,`)
	buffer.WriteString(`"from":`)
	if bs, err := tx.From_.MarshalJSON(); err != nil {
		return nil, err
	} else {
		buffer.Write(bs)
	}
	buffer.WriteString(`,`)
	buffer.WriteString(`"sigma_formulators":[`)
	for i, addr := range tx.SigmaFormulators {
		if i > 0 {
			buffer.WriteString(`,`)
		}
		if bs, err := addr.MarshalJSON(); err != nil {
			return nil, err
		} else {
			buffer.Write(bs)
		}
	}
	buffer.WriteString(`]`)
	buffer.WriteString(`}`)
	return buffer.Bytes(), nil
}
