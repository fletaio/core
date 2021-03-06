package data

import "errors"

// data errors
var (
	ErrExistAccount           = errors.New("exist account")
	ErrNotExistAccount        = errors.New("not exist account")
	ErrExistUTXO              = errors.New("exist utxo")
	ErrNotExistUTXO           = errors.New("not exist utxo")
	ErrNotExistEvent          = errors.New("not exist event")
	ErrDoubleSpent            = errors.New("double spent")
	ErrUnknownAccountType     = errors.New("unknown account type")
	ErrNotExistHandler        = errors.New("not exist handler")
	ErrExistHandler           = errors.New("exist handler")
	ErrNotExistAccounter      = errors.New("not exist accounter")
	ErrUnknownTransactionType = errors.New("unknown transaction type")
	ErrNotExistTransactor     = errors.New("not exist transactor")
	ErrInvalidChainCoordinate = errors.New("invalid chain coordinate")
	ErrUnknownEventType       = errors.New("unknown event type")
	ErrInvalidAccountName     = errors.New("invalid account name")
)
