package kernel

import (
	"encoding/binary"

	"github.com/fletaio/common/util"

	"github.com/fletaio/common"
	"github.com/fletaio/common/hash"
)

var (
	tagHeightHash          = []byte{1, 0}
	tagHeightHeader        = []byte{1, 2}
	tagHeightData          = []byte{1, 3}
	tagHashHeight          = []byte{1, 4}
	tagAccount             = []byte{2, 0}
	tagAccountName         = []byte{2, 1}
	tagAccountSeq          = []byte{2, 2}
	tagAccountBalance      = []byte{2, 3}
	tagAccountData         = []byte{2, 4}
	tagUTXO                = []byte{3, 0}
	tagCustomData          = []byte{4, 0}
	tagEvent               = []byte{5, 0}
	tagLockedBalance       = []byte{6, 0}
	tagLockedBalanceHeight = []byte{6, 1}
)

func toHeightDataKey(height uint32) []byte {
	bs := make([]byte, 6)
	copy(bs, tagHeightData)
	binary.LittleEndian.PutUint32(bs[2:], height)
	return bs
}

func toHeightHeaderKey(height uint32) []byte {
	bs := make([]byte, 6)
	copy(bs, tagHeightHeader)
	binary.LittleEndian.PutUint32(bs[2:], height)
	return bs
}

func toHeightHashKey(height uint32) []byte {
	bs := make([]byte, 6)
	copy(bs, tagHeightHash)
	binary.LittleEndian.PutUint32(bs[2:], height)
	return bs
}

func toHashHeightKey(h hash.Hash256) []byte {
	bs := make([]byte, 34)
	copy(bs, tagHashHeight)
	copy(bs[2:], h[:])
	return bs
}

func toAccountKey(addr common.Address) []byte {
	bs := make([]byte, 2+common.AddressSize)
	copy(bs, tagAccount)
	copy(bs[2:], addr[:])
	return bs
}

func toAccountNameKey(Name string) []byte {
	bs := make([]byte, 2+len(Name))
	copy(bs, tagAccountName)
	copy(bs[2:], []byte(Name))
	return bs
}

func toAccountSeqKey(addr common.Address) []byte {
	bs := make([]byte, 2+common.AddressSize)
	copy(bs, tagAccountSeq)
	copy(bs[2:], addr[:])
	return bs
}

func toAccountBalanceKey(addr common.Address) []byte {
	bs := make([]byte, 2+common.AddressSize)
	copy(bs, tagAccountBalance)
	copy(bs[2:], addr[:])
	return bs
}

func toAccountDataKey(key string) []byte {
	bs := make([]byte, 2+len(key))
	copy(bs, tagAccountData)
	copy(bs[2:], []byte(key))
	return bs
}

func toUTXOKey(id uint64) []byte {
	bs := make([]byte, 10)
	copy(bs, tagUTXO)
	binary.LittleEndian.PutUint64(bs[2:], id)
	return bs
}

func fromUTXOKey(bs []byte) uint64 {
	return binary.LittleEndian.Uint64(bs[2:])
}

func toCustomData(key string) []byte {
	bs := make([]byte, 2+len(key))
	copy(bs, tagCustomData)
	copy(bs[2:], []byte(key))
	return bs
}

func toEventKey(id uint64) []byte {
	bs := make([]byte, 10)
	copy(bs, tagEvent)
	binary.LittleEndian.PutUint64(bs[2:], id)
	return bs
}

func toLockedBalancePrefix(Address common.Address) []byte {
	bs := make([]byte, 2+common.AddressSize)
	copy(bs, tagLockedBalance)
	copy(bs[2:], Address[:])
	return bs
}

func toLockedBalanceKey(Address common.Address, UnlockHeight uint32) []byte {
	bs := make([]byte, 6+common.AddressSize)
	copy(bs, tagLockedBalance)
	copy(bs[2:], Address[:])
	binary.LittleEndian.PutUint32(bs[2+common.AddressSize:], UnlockHeight)
	return bs
}

func fromLockedBalanceKey(bs []byte) (common.Address, uint32) {
	var addr common.Address
	copy(addr[:], bs[2:])
	return addr, util.BytesToUint32(bs[2+common.AddressSize:])
}

func toLockedBalanceHeightPrefix(Height uint32) []byte {
	bs := make([]byte, 6)
	copy(bs, tagLockedBalance)
	binary.LittleEndian.PutUint32(bs[2:], Height)
	return bs
}

func toLockedBalanceHeightKey(UnlockHeight uint32, Address common.Address) []byte {
	bs := make([]byte, 6+common.AddressSize)
	copy(bs, tagLockedBalance)
	binary.LittleEndian.PutUint32(bs[2:], UnlockHeight)
	copy(bs[6:], Address[:])
	return bs
}

func fromLockedBalanceHeightKey(bs []byte) (common.Address, uint32) {
	var addr common.Address
	copy(addr[:], bs[6:])
	return addr, util.BytesToUint32(bs[2:])
}
