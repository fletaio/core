package key

import (
	"io"

	"github.com/fletaio/common"
	"github.com/fletaio/common/hash"
)

// Key is an interface that defines crypto key functions
type Key interface {
	io.ReaderFrom
	io.WriterTo
	Sign(h hash.Hash256) (common.Signature, error)
	SignWithPassphrase(h hash.Hash256, passphrase []byte) (common.Signature, error)
	Verify(h hash.Hash256, sig common.Signature) bool
	PublicKey() common.PublicKey
}
