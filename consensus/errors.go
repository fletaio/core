package consensus

import "errors"

// consensus errors
var (
	ErrInvalidSignerCount                 = errors.New("invalid signer count")
	ErrInvalidAccountSigner               = errors.New("invalid account signer")
	ErrInvalidAccountType                 = errors.New("invalid account type")
	ErrInvalidKeyHashCount                = errors.New("invalid key hash count")
	ErrInvalidSequence                    = errors.New("invalid sequence")
	ErrInsuffcientBalance                 = errors.New("insufficient balance")
	ErrInvalidToAddress                   = errors.New("invalid to address")
	ErrInvalidBlockHash                   = errors.New("invalid block hash")
	ErrInvalidPhase                       = errors.New("invalid phase")
	ErrExistAddress                       = errors.New("exist address")
	ErrExistAccountName                   = errors.New("exist account name")
	ErrInvalidAccountName                 = errors.New("invaild account name")
	ErrExceedCandidateCount               = errors.New("exceed candidate count")
	ErrInsufficientCandidateCount         = errors.New("insufficient candidate count")
	ErrInvalidMaxBlocksPerFormulator      = errors.New("invalid max blocks per formulator")
	ErrInvalidCommunityFormulationAddress = errors.New("invalid community formulator address")
	ErrInsufficientStakingAmount          = errors.New("insufficient staking amount")
	ErrExceedStakingAmount                = errors.New("exceed staking amount")
	ErrCriticalStakingAmount              = errors.New("critical staking amount")
	ErrInvalidStakingAddress              = errors.New("invalid staking address")
)
