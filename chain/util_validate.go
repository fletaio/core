package chain

import (
	"git.fleta.io/fleta/common"
	"git.fleta.io/fleta/common/store"
	"git.fleta.io/fleta/core/amount"
	"git.fleta.io/fleta/core/block"
	"git.fleta.io/fleta/core/chain/account"
	"git.fleta.io/fleta/core/transaction"
	"git.fleta.io/fleta/core/transaction/advanced"
)

// ValidateBlockGeneratorSignature TODO
func ValidateBlockGeneratorSignature(b *block.Block, GeneratorSignature common.Signature, ExpectedPublicKey common.PublicKey) error {
	h, err := b.Header.Hash()
	if err != nil {
		return err
	}
	{
		pubkey, err := common.RecoverPubkey(h, GeneratorSignature)
		if err != nil {
			return err
		}
		if !pubkey.Equal(ExpectedPublicKey) {
			return ErrInvalidGeneratorAddress
		}
	}
	return nil
}

// ValidationContext TODO
type ValidationContext struct {
	AccountHash       map[string]*account.Account
	DeleteAccountHash map[string]*account.Account
}

// NewValidationContext TODO
func NewValidationContext() *ValidationContext {
	ctx := &ValidationContext{
		AccountHash:       map[string]*account.Account{},
		DeleteAccountHash: map[string]*account.Account{},
	}
	return ctx
}

// LoadAccount TODO
func (ctx *ValidationContext) LoadAccount(cn Provider, addr common.Address) (*account.Account, error) {
	if _, has := ctx.DeleteAccountHash[string(addr[:])]; has {
		return nil, ErrDeletedAccount
	}

	targetAcc, has := ctx.AccountHash[string(addr[:])]
	if !has {
		acc, err := cn.Account(addr)
		if err != nil {
			return nil, err
		}
		targetAcc = acc
		ctx.AccountHash[string(addr[:])] = targetAcc
	}
	return targetAcc, nil
}

// ValidateTransaction TODO
func ValidateTransaction(cn Chain, tx transaction.Transaction, signers []common.Address) error {
	ctx := NewValidationContext()
	return validateTransaction(ctx, cn, tx, signers, 0, false)
}

// validateTransactionWithResult TODO
func validateTransactionWithResult(ctx *ValidationContext, cn Chain, tx transaction.Transaction, signers []common.Address, idx uint16) error {
	return validateTransaction(ctx, cn, tx, signers, idx, true)
}

// validateTransaction TODO
func validateTransaction(ctx *ValidationContext, cn Provider, t transaction.Transaction, signers []common.Address, idx uint16, bResult bool) error {
	Fee := cn.Fee(t)
	switch tx := t.(type) {
	case *advanced.Trade:
		fromAcc, err := ctx.LoadAccount(cn, tx.From)
		if err != nil {
			return err
		}
		if t.Seq() != fromAcc.Seq+1 {
			return ErrInvalidSequence
		}
		if err := ValidateSigners(fromAcc, signers); err != nil {
			return err
		}

		if fromAcc.Balance.Less(Fee) {
			return ErrInsuffcientBalance
		}
		fromAcc.Balance = fromAcc.Balance.Sub(Fee)
		fromAcc.Seq++

		for _, vout := range tx.Vout {
			if vout.Amount.IsZero() {
				return ErrInvalidAmount
			}
			if vout.Amount.Less(cn.Config().DustAmount) {
				return ErrTooSmallAmount
			}

			if fromAcc.Balance.Less(vout.Amount) {
				return ErrInsuffcientBalance
			}
			fromAcc.Balance = fromAcc.Balance.Sub(vout.Amount)

			toAcc, err := ctx.LoadAccount(cn, vout.Address)
			if err != nil {
				return err
			}
			toAcc.Balance = toAcc.Balance.Add(vout.Amount)
		}
	case *advanced.Formulation:
		fromAcc, err := ctx.LoadAccount(cn, tx.From)
		if err != nil {
			return err
		}
		if t.Seq() != fromAcc.Seq+1 {
			return ErrInvalidSequence
		}
		if err := ValidateSigners(fromAcc, signers); err != nil {
			return err
		}

		if fromAcc.Balance.Less(Fee) {
			return ErrInsuffcientBalance
		}
		fromAcc.Balance = fromAcc.Balance.Sub(Fee)
		fromAcc.Seq++

		TxHash, err := tx.Hash()
		if err != nil {
			return err
		}
		addr := common.AddressFromHash(cn.Coordinate(), FormulationAccountType, TxHash, common.ChecksumFromAddresses(signers))
		if _, err := ctx.LoadAccount(cn, addr); err != nil {
			if err != store.ErrNotExistKey {
				return err
			} else {
				acc := &account.Account{
					Address:      addr,
					ChainCoord:   cn.Coordinate(),
					Type:         FormulationAccountType,
					Balance:      amount.NewCoinAmount(0, 0),
					KeyAddresses: signers,
				}
				ctx.AccountHash[string(addr[:])] = acc
			}
		} else {
			return ErrExistAddress
		}
	case *advanced.RevokeFormulation:
		fromAcc, err := ctx.LoadAccount(cn, tx.From)
		if err != nil {
			return err
		}
		if t.Seq() != fromAcc.Seq+1 {
			return ErrInvalidSequence
		}
		if err := ValidateSigners(fromAcc, signers); err != nil {
			return err
		}

		formulationAcc, err := ctx.LoadAccount(cn, tx.FormulationAddress)
		if err != nil {
			return err
		}
		if err := ValidateSigners(formulationAcc, signers); err != nil {
			return err
		}

		if fromAcc.Balance.Less(Fee) {
			return ErrInsuffcientBalance
		}
		fromAcc.Balance = fromAcc.Balance.Sub(Fee)
		fromAcc.Seq++
		fromAcc.Balance = fromAcc.Balance.Add(formulationAcc.Balance).Add(cn.Config().FormulationCost)

		ctx.DeleteAccountHash[string(tx.FormulationAddress[:])] = formulationAcc
	case *advanced.MultiSigAccount:
		fromAcc, err := ctx.LoadAccount(cn, tx.From)
		if err != nil {
			return err
		}
		if t.Seq() != fromAcc.Seq+1 {
			return ErrInvalidSequence
		}
		if err := ValidateSigners(fromAcc, signers); err != nil {
			return err
		}

		if fromAcc.Balance.Less(Fee) {
			return ErrInsuffcientBalance
		}
		fromAcc.Balance = fromAcc.Balance.Sub(Fee)
		fromAcc.Seq++

		TxHash, err := tx.Hash()
		if err != nil {
			return err
		}
		addr := common.AddressFromHash(cn.Coordinate(), MultiSigAccountType, TxHash, common.ChecksumFromAddresses(tx.Addresses))
		if _, err := ctx.LoadAccount(cn, addr); err != nil {
			if err != store.ErrNotExistKey {
				return err
			} else {
				acc := &account.Account{
					Address:      addr,
					ChainCoord:   cn.Coordinate(),
					Type:         MultiSigAccountType,
					Balance:      amount.NewCoinAmount(0, 0),
					KeyAddresses: tx.Addresses,
				}
				ctx.AccountHash[string(addr[:])] = acc
			}
		} else {
			return ErrExistAddress
		}
	}
	return nil
}

// ValidateSigners TODO
func ValidateSigners(acc *account.Account, addrs []common.Address) error {
	if len(addrs) != len(acc.KeyAddresses) {
		return ErrMismatchSignaturesCount
	}
	for i, addr := range addrs {
		if addr.Type() != KeyAccountType {
			return ErrInvalidAccountType
		}
		if !addr.Equal(acc.KeyAddresses[i]) {
			return ErrInvalidTransactionSignature
		}
	}
	return nil
}
