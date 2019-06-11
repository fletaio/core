package consensus

import (
	"bytes"

	"github.com/fletaio/common"
)

var (
	tagStaking     = []byte{1, 0}
	tagAutoStaking = []byte{1, 1}
)

func toStakingKey(addr common.Address) []byte {
	bs := make([]byte, 2+common.AddressSize)
	copy(bs, tagStaking)
	copy(bs[2:], addr[:])
	return bs
}

// FromStakingKey returns staking address if it is staking key
func FromStakingKey(bs []byte) (common.Address, bool) {
	if bytes.HasPrefix(bs, tagStaking) {
		var addr common.Address
		copy(addr[:], bs[2:])
		return addr, true
	} else {
		return common.Address{}, false
	}
}

func toAutoStakingKey(addr common.Address) []byte {
	bs := make([]byte, 2+common.AddressSize)
	copy(bs, tagAutoStaking)
	copy(bs[2:], addr[:])
	return bs
}
