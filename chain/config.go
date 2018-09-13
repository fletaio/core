package chain

import (
	"git.fleta.io/fleta/core/amount"
)

// Config TODO
type Config struct {
	Version             uint16
	FormulationCost     *amount.Amount
	MultiSigAccountCost *amount.Amount
	DustAmount          *amount.Amount
}
