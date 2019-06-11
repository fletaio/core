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

var gConsensusPolicyMap = map[uint64]*ConsensusPolicy{}

func SetConsensusPolicy(chainCoord *common.Coordinate, pc *ConsensusPolicy) {
	gConsensusPolicyMap[chainCoord.ID()] = pc
}

func GetConsensusPolicy(chainCoord *common.Coordinate) (*ConsensusPolicy, error) {
	pc, has := gConsensusPolicyMap[chainCoord.ID()]
	if !has {
		return nil, ErrNotExistConsensusPolicy
	}
	return pc, nil
}

func init() {
	data.RegisterTransaction("consensus.CreateFormulation", func(t transaction.Type) transaction.Transaction {
		return &CreateFormulation{
			Base: transaction.Base{
				Type_: t,
			},
		}
	}, func(loader data.Loader, t transaction.Transaction, signers []common.PublicHash) error {
		tx := t.(*CreateFormulation)
		if len(tx.Name) < 8 || len(tx.Name) > 16 {
			return ErrInvalidAccountName
		}

		policy, has := gConsensusPolicyMap[loader.ChainCoord().ID()]
		if !has {
			return ErrNotExistConsensusPolicy
		}
		if loader.TargetHeight() < policy.FormulatorCreationLimitHeight {
			return ErrFormulatorCreationLimited
		}

		switch tx.FormulationType {
		case AlphaFormulatorType:
		case HyperFormulatorType:
		default:
			return ErrInvalidAccountType
		}

		if tx.Seq() <= loader.Seq(tx.From()) {
			return ErrInvalidSequence
		}

		fromAcc, err := loader.Account(tx.From())
		if err != nil {
			return err
		}

		if err := loader.Accounter().Validate(loader, fromAcc, signers); err != nil {
			return err
		}
		return nil
	}, func(ctx *data.Context, Fee *amount.Amount, t transaction.Transaction, coord *common.Coordinate) (ret interface{}, rerr error) {
		tx := t.(*CreateFormulation)
		if len(tx.Name) < 8 || len(tx.Name) > 16 {
			return nil, ErrInvalidAccountName
		}

		policy, has := gConsensusPolicyMap[ctx.ChainCoord().ID()]
		if !has {
			return nil, ErrNotExistConsensusPolicy
		}
		if ctx.TargetHeight() < policy.FormulatorCreationLimitHeight {
			return nil, ErrFormulatorCreationLimited
		}

		sn := ctx.Snapshot()
		defer ctx.Revert(sn)

		if tx.Seq() != ctx.Seq(tx.From())+1 {
			return nil, ErrInvalidSequence
		}
		ctx.AddSeq(tx.From())

		fromAcc, err := ctx.Account(tx.From())
		if err != nil {
			return nil, err
		}
		if err := fromAcc.SubBalance(Fee); err != nil {
			return nil, err
		}

		var Amount *amount.Amount
		switch tx.FormulationType {
		case AlphaFormulatorType:
			Amount = policy.AlphaFormulationAmount
		case HyperFormulatorType:
			Amount = policy.HyperFormulationAmount
		default:
			return nil, ErrInvalidAccountType
		}
		if err := fromAcc.SubBalance(Amount); err != nil {
			return nil, err
		}

		addr := common.NewAddress(coord, 0)
		if is, err := ctx.IsExistAccount(addr); err != nil {
			return nil, err
		} else if is {
			return nil, ErrExistAddress
		} else if isn, err := ctx.IsExistAccountName(tx.Name); err != nil {
			return nil, err
		} else if isn {
			return nil, ErrExistAccountName
		} else {
			a, err := ctx.Accounter().NewByTypeName("consensus.FormulationAccount")
			if err != nil {
				return nil, err
			}
			acc := a.(*FormulationAccount)
			acc.Address_ = addr
			acc.Name_ = tx.Name
			acc.FormulationType = tx.FormulationType
			acc.KeyHash = tx.KeyHash
			acc.Amount = Amount
			ctx.CreateAccount(acc)
		}
		ctx.Commit(sn)
		return nil, nil
	})
}

// CreateFormulation is a consensus.CreateFormulation
// It is used to make formulation account
type CreateFormulation struct {
	transaction.Base
	Seq_            uint64
	From_           common.Address
	FormulationType FormulationType
	Name            string
	KeyHash         common.PublicHash
}

// IsUTXO returns false
func (tx *CreateFormulation) IsUTXO() bool {
	return false
}

// From returns the creator of the transaction
func (tx *CreateFormulation) From() common.Address {
	return tx.From_
}

// Seq returns the sequence of the transaction
func (tx *CreateFormulation) Seq() uint64 {
	return tx.Seq_
}

// Hash returns the hash value of it
func (tx *CreateFormulation) Hash() hash.Hash256 {
	return hash.DoubleHashByWriterTo(tx)
}

// WriteTo is a serialization function
func (tx *CreateFormulation) WriteTo(w io.Writer) (int64, error) {
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
	if n, err := util.WriteUint8(w, uint8(tx.FormulationType)); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := util.WriteString(w, tx.Name); err != nil {
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

// ReadFrom is a deserialization function
func (tx *CreateFormulation) ReadFrom(r io.Reader) (int64, error) {
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
	if v, n, err := util.ReadUint8(r); err != nil {
		return read, err
	} else {
		read += n
		tx.FormulationType = FormulationType(v)
	}
	if v, n, err := util.ReadString(r); err != nil {
		return read, err
	} else {
		read += n
		tx.Name = v
	}
	if n, err := tx.KeyHash.ReadFrom(r); err != nil {
		return read, err
	} else {
		read += n
	}
	return read, nil
}

// MarshalJSON is a marshaler function
func (tx *CreateFormulation) MarshalJSON() ([]byte, error) {
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
	buffer.WriteString(`"name":`)
	if bs, err := json.Marshal(tx.Name); err != nil {
		return nil, err
	} else {
		buffer.Write(bs)
	}
	buffer.WriteString(`,`)
	buffer.WriteString(`"from":`)
	if bs, err := tx.KeyHash.MarshalJSON(); err != nil {
		return nil, err
	} else {
		buffer.Write(bs)
	}
	buffer.WriteString(`}`)
	return buffer.Bytes(), nil
}
