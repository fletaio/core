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
	data.RegisterTransaction("consensus.Staking", func(t transaction.Type) transaction.Transaction {
		return &Staking{
			Base: transaction.Base{
				Type_: t,
			},
			Amount: amount.NewCoinAmount(0, 0),
		}
	}, func(loader data.Loader, t transaction.Transaction, signers []common.PublicHash) error {
		tx := t.(*Staking)
		if tx.Seq() <= loader.Seq(tx.From()) {
			return ErrInvalidSequence
		}

		if tx.Amount.Less(amount.COIN.DivC(10)) {
			return ErrInvalidStakingAmount
		}

		acc, err := loader.Account(tx.HyperFormulator)
		if err != nil {
			return err
		}
		frAcc, is := acc.(*FormulationAccount)
		if !is {
			return ErrInvalidAccountType
		}
		if frAcc.FormulationType != HyperFormulatorType {
			return ErrInvalidAccountType
		}
		if !frAcc.Policy.MinimumStaking.IsZero() && tx.Amount.Less(frAcc.Policy.MinimumStaking) {
			return ErrInvalidStakingAmount
		}
		if !frAcc.Policy.MaximumStaking.IsZero() && frAcc.Policy.MaximumStaking.Less(tx.Amount) {
			return ErrInvalidStakingAmount
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
		tx := t.(*Staking)
		sn := ctx.Snapshot()
		defer ctx.Revert(sn)

		if tx.Seq() != ctx.Seq(tx.From())+1 {
			return nil, ErrInvalidSequence
		}
		ctx.AddSeq(tx.From())

		if tx.Amount.Less(amount.COIN.DivC(10)) {
			return nil, ErrInvalidStakingAmount
		}

		acc, err := ctx.Account(tx.HyperFormulator)
		if err != nil {
			return nil, err
		}
		frAcc, is := acc.(*FormulationAccount)
		if !is {
			return nil, ErrInvalidAccountType
		}
		if frAcc.FormulationType != HyperFormulatorType {
			return nil, ErrInvalidAccountType
		}
		if !frAcc.Policy.MinimumStaking.IsZero() && tx.Amount.Less(frAcc.Policy.MinimumStaking) {
			return nil, ErrInsufficientStakingAmount
		}
		if !frAcc.Policy.MaximumStaking.IsZero() && frAcc.Policy.MaximumStaking.Less(tx.Amount) {
			return nil, ErrExceedStakingAmount
		}

		fromAcc, err := ctx.Account(tx.From())
		if err != nil {
			return nil, err
		}
		if err := fromAcc.SubBalance(Fee); err != nil {
			return nil, err
		}
		if err := fromAcc.SubBalance(tx.Amount); err != nil {
			return nil, err
		}

		var fromStakingAmount *amount.Amount
		if bs := ctx.AccountData(tx.HyperFormulator, ToStakingKey(tx.From())); len(bs) > 0 {
			fromStakingAmount = amount.NewAmountFromBytes(bs)
		} else {
			fromStakingAmount = amount.NewCoinAmount(0, 0)
		}
		fromStakingAmount.Add(tx.Amount)
		ctx.SetAccountData(tx.HyperFormulator, ToStakingKey(tx.From()), fromStakingAmount.Bytes())
		frAcc.StakingAmount = frAcc.StakingAmount.Add(tx.Amount)

		ctx.Commit(sn)
		return nil, nil
	})
}

// Staking is a consensus.Staking
// It is used to make formulation account
type Staking struct {
	transaction.Base
	Seq_            uint64
	From_           common.Address
	HyperFormulator common.Address
	Amount          *amount.Amount
}

// IsUTXO returns false
func (tx *Staking) IsUTXO() bool {
	return false
}

// From returns the creator of the transaction
func (tx *Staking) From() common.Address {
	return tx.From_
}

// Seq returns the sequence of the transaction
func (tx *Staking) Seq() uint64 {
	return tx.Seq_
}

// Hash returns the hash value of it
func (tx *Staking) Hash() hash.Hash256 {
	return hash.DoubleHashByWriterTo(tx)
}

// WriteTo is a serialization function
func (tx *Staking) WriteTo(w io.Writer) (int64, error) {
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
	if n, err := tx.HyperFormulator.WriteTo(w); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := tx.Amount.WriteTo(w); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	return wrote, nil
}

// ReadFrom is a deserialization function
func (tx *Staking) ReadFrom(r io.Reader) (int64, error) {
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
	if n, err := tx.HyperFormulator.ReadFrom(r); err != nil {
		return read, err
	} else {
		read += n
	}
	if n, err := tx.Amount.ReadFrom(r); err != nil {
		return read, err
	} else {
		read += n
	}
	return read, nil
}

// MarshalJSON is a marshaler function
func (tx *Staking) MarshalJSON() ([]byte, error) {
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
	buffer.WriteString(`"Hyper_formulator":`)
	if bs, err := tx.HyperFormulator.MarshalJSON(); err != nil {
		return nil, err
	} else {
		buffer.Write(bs)
	}
	buffer.WriteString(`,`)
	buffer.WriteString(`"amount":`)
	if bs, err := tx.Amount.MarshalJSON(); err != nil {
		return nil, err
	} else {
		buffer.Write(bs)
	}
	buffer.WriteString(`}`)
	return buffer.Bytes(), nil
}
