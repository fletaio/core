package kernel

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fletaio/core/amount"

	"github.com/dgraph-io/badger"
	"github.com/fletaio/common"
	"github.com/fletaio/common/hash"
	"github.com/fletaio/common/util"
	"github.com/fletaio/core/account"
	"github.com/fletaio/core/block"
	"github.com/fletaio/core/data"
	"github.com/fletaio/core/db"
	"github.com/fletaio/core/event"
	"github.com/fletaio/core/transaction"
	"github.com/fletaio/framework/chain"
)

// Store saves the target chain state
// All updates are executed in one transaction with FileSync option
type Store struct {
	sync.Mutex
	db         *badger.DB
	version    uint16
	accounter  *data.Accounter
	transactor *data.Transactor
	eventer    *data.Eventer
	SeqMapLock sync.Mutex
	SeqMap     map[common.Address]uint64
	cache      storeCache
	ticker     *time.Ticker
	closeLock  sync.RWMutex
	isClose    bool
}

type storeCache struct {
	cached     bool
	height     uint32
	heightHash hash.Hash256
	heightData *chain.Data
}

// NewStore returns a Store
func NewStore(path string, version uint16, act *data.Accounter, tran *data.Transactor, evt *data.Eventer, bRecover bool) (*Store, error) {
	if !act.ChainCoord().Equal(tran.ChainCoord()) {
		return nil, ErrInvalidChainCoord
	}

	opts := badger.DefaultOptions
	opts.Dir = path
	opts.ValueDir = path
	opts.Truncate = bRecover
	opts.SyncWrites = true
	lockfilePath := filepath.Join(opts.Dir, "LOCK")
	os.MkdirAll(path, os.ModeDir)

	os.Remove(lockfilePath)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	{
	again:
		if err := db.RunValueLogGC(0.7); err != nil {
		} else {
			goto again
		}
	}

	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for range ticker.C {
		again:
			if err := db.RunValueLogGC(0.7); err != nil {
			} else {
				goto again
			}
		}
	}()

	return &Store{
		db:         db,
		ticker:     ticker,
		version:    version,
		accounter:  act,
		transactor: tran,
		eventer:    evt,
		SeqMap:     map[common.Address]uint64{},
	}, nil
}

// Close terminate and clean store
func (st *Store) Close() {
	st.closeLock.Lock()
	defer st.closeLock.Unlock()

	st.isClose = true
	st.db.Close()
	st.ticker.Stop()
	st.db = nil
	st.ticker = nil
}

// CreateHeader returns a header that implements the chain header interface
func (st *Store) CreateHeader() chain.Header {
	return &block.Header{}
}

// CreateBody returns a header that implements the chain header interface
func (st *Store) CreateBody() chain.Body {
	return &block.Body{
		Tran: st.transactor,
	}
}

// Version returns the version of the target chain
func (st *Store) Version() uint16 {
	return st.version
}

// ChainCoord returns the coordinate of the target chain
func (st *Store) ChainCoord() *common.Coordinate {
	return st.accounter.ChainCoord()
}

// Accounter returns the accounter of the target chain
func (st *Store) Accounter() *data.Accounter {
	return st.accounter
}

// Transactor returns the transactor of the target chain
func (st *Store) Transactor() *data.Transactor {
	return st.transactor
}

// Provider returns the provider of the kernel
func (st *Store) Provider() chain.Provider {
	return st
}

// Eventer returns the eventer of the target chain
func (st *Store) Eventer() *data.Eventer {
	return st.eventer
}

// TargetHeight returns the target height of the target chain
func (st *Store) TargetHeight() uint32 {
	return st.Height() + 1
}

// LastHash returns the last hash of the chain
func (st *Store) LastHash() hash.Hash256 {
	h, err := st.Hash(st.Height())
	if err != nil {
		if err != ErrStoreClosed {
			// should have not reach
			panic(err)
		}
		return hash.Hash256{}
	}
	return h
}

// Hash returns the hash of the data by height
func (st *Store) Hash(height uint32) (hash.Hash256, error) {
	st.closeLock.RLock()
	defer st.closeLock.RUnlock()
	if st.isClose {
		return hash.Hash256{}, ErrStoreClosed
	}

	if st.cache.cached {
		if st.cache.height == height {
			return st.cache.heightHash, nil
		}
	}

	var h hash.Hash256
	if err := st.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(toHeightHashKey(height))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return db.ErrNotExistKey
			} else {
				return err
			}
		}
		value, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		if _, err := h.ReadFrom(bytes.NewReader(value)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return hash.Hash256{}, err
	}
	return h, nil
}

// Header returns the header of the data by height
func (st *Store) Header(height uint32) (chain.Header, error) {
	st.closeLock.RLock()
	defer st.closeLock.RUnlock()
	if st.isClose {
		return nil, ErrStoreClosed
	}

	if height < 1 {
		return nil, db.ErrNotExistKey
	}
	if st.cache.cached {
		if st.cache.height == height {
			return st.cache.heightData.Header, nil
		}
	}

	var ch chain.Header
	if err := st.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(toHeightHeaderKey(height))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return db.ErrNotExistKey
			} else {
				return err
			}
		}
		value, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		ch = st.CreateHeader()
		if _, err := ch.ReadFrom(bytes.NewReader(value)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return ch, nil
}

// Data returns the data by height
func (st *Store) Data(height uint32) (*chain.Data, error) {
	st.closeLock.RLock()
	defer st.closeLock.RUnlock()
	if st.isClose {
		return nil, ErrStoreClosed
	}

	if height < 1 {
		return nil, db.ErrNotExistKey
	}
	if st.cache.cached {
		if st.cache.height == height {
			return st.cache.heightData, nil
		}
	}

	var cd *chain.Data
	if err := st.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(toHeightDataKey(height))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return db.ErrNotExistKey
			} else {
				return err
			}
		}
		value, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		cd = &chain.Data{
			Header: st.CreateHeader(),
			Body:   st.CreateBody(),
		}
		if _, err := cd.ReadFrom(bytes.NewReader(value)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return cd, nil
}

// Height returns the current height of the target chain
func (st *Store) Height() uint32 {
	st.closeLock.RLock()
	defer st.closeLock.RUnlock()
	if st.isClose {
		return 0
	}

	if st.cache.cached {
		return st.cache.height
	}

	var height uint32
	st.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("height"))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return db.ErrNotExistKey
			} else {
				return err
			}
		}
		value, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		height = util.BytesToUint32(value)
		return nil
	})
	return height
}

// Accounts returns all accounts in the store
func (st *Store) Accounts() ([]account.Account, error) {
	st.closeLock.RLock()
	defer st.closeLock.RUnlock()
	if st.isClose {
		return nil, ErrStoreClosed
	}

	list := []account.Account{}
	if err := st.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(tagAccount); it.ValidForPrefix(tagAccount); it.Next() {
			item := it.Item()
			value, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			acc, err := st.accounter.NewByType(account.Type(value[0]))
			if err != nil {
				return err
			}
			if _, err := acc.ReadFrom(bytes.NewReader(value[1:])); err != nil {
				return err
			}
			list = append(list, acc)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return list, nil
}

// Seq returns the sequence of the transaction
func (st *Store) Seq(addr common.Address) uint64 {
	st.closeLock.RLock()
	defer st.closeLock.RUnlock()
	if st.isClose {
		return 0
	}

	st.SeqMapLock.Lock()
	defer st.SeqMapLock.Unlock()

	if seq, has := st.SeqMap[addr]; has {
		return seq
	} else {
		var seq uint64
		if err := st.db.View(func(txn *badger.Txn) error {
			item, err := txn.Get(toAccountSeqKey(addr))
			if err != nil {
				return err
			}
			value, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			seq = util.BytesToUint64(value)
			return nil
		}); err != nil {
			return 0
		}
		st.SeqMap[addr] = seq
		return seq
	}
}

// LockedBalances returns locked balances of the address
func (st *Store) LockedBalances(addr common.Address) ([]*data.LockedBalance, error) {
	st.closeLock.RLock()
	defer st.closeLock.RUnlock()
	if st.isClose {
		return nil, ErrStoreClosed
	}

	list := []*data.LockedBalance{}
	if err := st.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := toLockedBalancePrefix(addr)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			value, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			Address, UnlockHeight := fromLockedBalanceKey(item.Key())
			list = append(list, &data.LockedBalance{
				Address:      Address,
				Amount:       amount.NewAmountFromBytes(value),
				UnlockHeight: UnlockHeight,
			})
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return list, nil
}

// LockedBalancesByHeight returns locked balances of the height
func (st *Store) LockedBalancesByHeight(Height uint32) ([]*data.LockedBalance, error) {
	st.closeLock.RLock()
	defer st.closeLock.RUnlock()
	if st.isClose {
		return nil, ErrStoreClosed
	}

	list := []*data.LockedBalance{}
	if err := st.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := toLockedBalanceHeightPrefix(Height)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			value, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			Address, UnlockHeight := fromLockedBalanceHeightKey(item.Key())
			list = append(list, &data.LockedBalance{
				Address:      Address,
				Amount:       amount.NewAmountFromBytes(value),
				UnlockHeight: UnlockHeight,
			})
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return list, nil
}

// Account returns the account instance of the address from the store
func (st *Store) Account(addr common.Address) (account.Account, error) {
	st.closeLock.RLock()
	defer st.closeLock.RUnlock()
	if st.isClose {
		return nil, ErrStoreClosed
	}

	var acc account.Account
	if err := st.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(toAccountKey(addr))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return db.ErrNotExistKey
			} else {
				return err
			}
		}
		value, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		acc, err = st.accounter.NewByType(account.Type(value[0]))
		if err != nil {
			return err
		}
		if _, err := acc.ReadFrom(bytes.NewReader(value[1:])); err != nil {
			return err
		}
		return nil
	}); err != nil {
		if err == db.ErrNotExistKey {
			return nil, data.ErrNotExistAccount
		} else {
			return nil, err
		}
	}
	return acc, nil
}

// AddressByName returns the account instance of the name from the store
func (st *Store) AddressByName(Name string) (common.Address, error) {
	st.closeLock.RLock()
	defer st.closeLock.RUnlock()
	if st.isClose {
		return common.Address{}, ErrStoreClosed
	}

	var addr common.Address
	if err := st.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(toAccountNameKey(Name))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return db.ErrNotExistKey
			} else {
				return err
			}
		}
		value, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		if _, err := addr.ReadFrom(bytes.NewReader(value)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		if err == db.ErrNotExistKey {
			return common.Address{}, data.ErrNotExistAccount
		} else {
			return common.Address{}, err
		}
	}
	return addr, nil
}

// IsExistAccount checks that the account of the address is exist or not
func (st *Store) IsExistAccount(addr common.Address) (bool, error) {
	st.closeLock.RLock()
	defer st.closeLock.RUnlock()
	if st.isClose {
		return false, ErrStoreClosed
	}

	var isExist bool
	if err := st.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(toAccountKey(addr))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return db.ErrNotExistKey
			} else {
				return err
			}
		}
		isExist = !item.IsDeletedOrExpired()
		return nil
	}); err != nil {
		if err == db.ErrNotExistKey {
			return false, nil
		} else {
			return false, err
		}
	}
	return isExist, nil
}

// IsExistAccountName checks that the account of the name is exist or not
func (st *Store) IsExistAccountName(Name string) (bool, error) {
	st.closeLock.RLock()
	defer st.closeLock.RUnlock()
	if st.isClose {
		return false, ErrStoreClosed
	}

	var isExist bool
	if err := st.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(toAccountNameKey(Name))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return db.ErrNotExistKey
			} else {
				return err
			}
		}
		isExist = !item.IsDeletedOrExpired()
		return nil
	}); err != nil {
		if err == db.ErrNotExistKey {
			return false, nil
		} else {
			return false, err
		}
	}
	return isExist, nil
}

// AccountDataKeys returns all data keys of the account in the store
func (st *Store) AccountDataKeys(addr common.Address, Prefix []byte) ([][]byte, error) {
	st.closeLock.RLock()
	defer st.closeLock.RUnlock()
	if st.isClose {
		return nil, ErrStoreClosed
	}

	list := [][]byte{}
	if err := st.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		pre := toAccountDataKey(string(addr[:]))
		if len(Prefix) > 0 {
			pre = append(pre, Prefix...)
		}
		for it.Seek(pre); it.ValidForPrefix(pre); it.Next() {
			item := it.Item()
			key := item.Key()
			list = append(list, key[len(pre):])
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return list, nil
}

// AccountData returns the account data from the store
func (st *Store) AccountData(addr common.Address, name []byte) []byte {
	st.closeLock.RLock()
	defer st.closeLock.RUnlock()
	if st.isClose {
		return nil
	}

	key := string(addr[:]) + string(name)
	var data []byte
	if err := st.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(toAccountDataKey(key))
		if err != nil {
			return err
		}
		value, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		data = value
		return nil
	}); err != nil {
		return nil
	}
	return data
}

// UTXOs returns all UTXOs in the store
func (st *Store) UTXOs() ([]*transaction.UTXO, error) {
	st.closeLock.RLock()
	defer st.closeLock.RUnlock()
	if st.isClose {
		return nil, ErrStoreClosed
	}

	list := []*transaction.UTXO{}
	if err := st.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(tagUTXO); it.ValidForPrefix(tagUTXO); it.Next() {
			item := it.Item()
			value, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			utxo := &transaction.UTXO{
				TxIn:  transaction.NewTxIn(fromUTXOKey(item.Key())),
				TxOut: transaction.NewTxOut(),
			}
			if _, err := utxo.TxOut.ReadFrom(bytes.NewReader(value)); err != nil {
				return err
			}
			list = append(list, utxo)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return list, nil
}

// IsExistUTXO checks that the utxo of the id is exist or not
func (st *Store) IsExistUTXO(id uint64) (bool, error) {
	st.closeLock.RLock()
	defer st.closeLock.RUnlock()
	if st.isClose {
		return false, ErrStoreClosed
	}

	var isExist bool
	if err := st.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(toUTXOKey(id))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return db.ErrNotExistKey
			} else {
				return err
			}
		}
		isExist = !item.IsDeletedOrExpired()
		return nil
	}); err != nil {
		if err == db.ErrNotExistKey {
			return false, nil
		} else {
			return false, err
		}
	}
	return isExist, nil
}

// UTXO returns the UTXO from the top store
func (st *Store) UTXO(id uint64) (*transaction.UTXO, error) {
	st.closeLock.RLock()
	defer st.closeLock.RUnlock()
	if st.isClose {
		return nil, ErrStoreClosed
	}

	var utxo *transaction.UTXO
	if err := st.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(toUTXOKey(id))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return data.ErrNotExistUTXO
			} else {
				return err
			}
		}
		value, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		utxo = &transaction.UTXO{
			TxIn:  transaction.NewTxIn(id),
			TxOut: transaction.NewTxOut(),
		}
		if _, err := utxo.TxOut.ReadFrom(bytes.NewReader(value)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return utxo, nil
}

// CustomData returns the custom data by the key from the store
func (st *Store) CustomData(key string) []byte {
	st.closeLock.RLock()
	defer st.closeLock.RUnlock()
	if st.isClose {
		return nil
	}

	var bs []byte
	st.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(toCustomData(key))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return db.ErrNotExistKey
			} else {
				return err
			}
		}
		value, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		bs = value
		return nil
	})
	return bs
}

// SetCustomData updates the custom data
func (st *Store) SetCustomData(key string, value []byte) error {
	st.closeLock.RLock()
	defer st.closeLock.RUnlock()
	if st.isClose {
		return ErrStoreClosed
	}

	return st.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(toCustomData(key), value); err != nil {
			return err
		}
		return nil
	})
}

// DeleteCustomData deletes the custom data
func (st *Store) DeleteCustomData(key string) error {
	st.closeLock.RLock()
	defer st.closeLock.RUnlock()
	if st.isClose {
		return ErrStoreClosed
	}

	return st.db.Update(func(txn *badger.Txn) error {
		if err := txn.Delete(toCustomData(key)); err != nil {
			return err
		}
		return nil
	})
}

// Events returns all events by conditions
func (st *Store) Events(From uint32, To uint32) ([]event.Event, error) {
	st.closeLock.RLock()
	defer st.closeLock.RUnlock()
	if st.isClose {
		return nil, ErrStoreClosed
	}

	list := []event.Event{}
	if err := st.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		tagBegin := toEventKey(event.MarshalID(common.NewCoordinate(From, 0), 0))
		tagEnd := toEventKey(event.MarshalID(common.NewCoordinate(To, 65535), 65535))
		for it.Seek(tagBegin); it.ValidForPrefix(tagEnd); it.Next() {
			item := it.Item()
			value, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			acc, err := st.eventer.NewByType(event.Type(util.BytesToUint64(value[:8])))
			if err != nil {
				return err
			}
			if _, err := acc.ReadFrom(bytes.NewReader(value[8:])); err != nil {
				return err
			}
			list = append(list, acc)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return list, nil
}

// StoreGenesis stores the genesis data
func (st *Store) StoreGenesis(genHash hash.Hash256, ctd *data.ContextData, customHash map[string][]byte) error {
	st.closeLock.RLock()
	defer st.closeLock.RUnlock()
	if st.isClose {
		return ErrStoreClosed
	}

	if st.Height() > 0 {
		return chain.ErrAlreadyGenesised
	}
	if err := st.db.Update(func(txn *badger.Txn) error {
		{
			if err := txn.Set(toHeightHashKey(0), genHash[:]); err != nil {
				return err
			}
			bsHeight := util.Uint32ToBytes(0)
			if err := txn.Set(toHashHeightKey(genHash), bsHeight); err != nil {
				return err
			}
			if err := txn.Set([]byte("height"), bsHeight); err != nil {
				return err
			}
		}
		if err := applyContextData(txn, ctd); err != nil {
			return err
		}
		for k, v := range customHash {
			if err := txn.Set(toCustomData(k), v); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	st.cache.height = 0
	st.cache.heightHash = genHash
	st.cache.heightData = nil
	st.cache.cached = true
	return nil
}

// StoreData stores the data
func (st *Store) StoreData(cd *chain.Data, ctd *data.ContextData, customHash map[string][]byte) error {
	st.closeLock.RLock()
	defer st.closeLock.RUnlock()
	if st.isClose {
		return ErrStoreClosed
	}

	DataHash := cd.Header.Hash()
	if err := st.db.Update(func(txn *badger.Txn) error {
		{
			var buffer bytes.Buffer
			if _, err := cd.WriteTo(&buffer); err != nil {
				return err
			}
			if err := txn.Set(toHeightDataKey(cd.Header.Height()), buffer.Bytes()); err != nil {
				return err
			}
		}
		{
			var buffer bytes.Buffer
			if _, err := cd.Header.WriteTo(&buffer); err != nil {
				return err
			}
			if err := txn.Set(toHeightHeaderKey(cd.Header.Height()), buffer.Bytes()); err != nil {
				return err
			}
		}
		{
			if err := txn.Set(toHeightHashKey(cd.Header.Height()), DataHash[:]); err != nil {
				return err
			}
			bsHeight := util.Uint32ToBytes(cd.Header.Height())
			if err := txn.Set(toHashHeightKey(DataHash), bsHeight); err != nil {
				return err
			}
			if err := txn.Set([]byte("height"), bsHeight); err != nil {
				return err
			}
		}
		if err := applyContextData(txn, ctd); err != nil {
			return err
		}
		for k, v := range customHash {
			if err := txn.Set(toCustomData(k), v); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	st.SeqMapLock.Lock()
	for k, v := range ctd.SeqMap {
		st.SeqMap[k] = v
	}
	st.SeqMapLock.Unlock()
	st.cache.height = cd.Header.Height()
	st.cache.heightHash = DataHash
	st.cache.heightData = cd
	st.cache.cached = true
	return nil
}

func applyContextData(txn *badger.Txn, ctd *data.ContextData) error {
	for k, v := range ctd.SeqMap {
		if err := txn.Set(toAccountSeqKey(k), util.Uint64ToBytes(v)); err != nil {
			return err
		}
	}
	for _, v := range ctd.LockedBalances {
		var AmountSum *amount.Amount
		item, err := txn.Get(toLockedBalanceKey(v.Address, v.UnlockHeight))
		if err != nil {
			if err != badger.ErrKeyNotFound {
				return err
			}
			AmountSum = amount.NewCoinAmount(0, 0)
		} else {
			value, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			AmountSum = amount.NewAmountFromBytes(value)
		}
		if err := txn.Set(toLockedBalanceKey(v.Address, v.UnlockHeight), AmountSum.Add(v.Amount).Bytes()); err != nil {
			return err
		}
		if err := txn.Set(toLockedBalanceHeightKey(v.UnlockHeight, v.Address), AmountSum.Add(v.Amount).Bytes()); err != nil {
			return err
		}
	}
	for _, v := range ctd.DeletedLockedBalances {
		if err := txn.Delete(toLockedBalanceKey(v.Address, v.UnlockHeight)); err != nil {
			return err
		}
		if err := txn.Delete(toLockedBalanceHeightKey(v.UnlockHeight, v.Address)); err != nil {
			return err
		}
	}
	for k, v := range ctd.AccountMap {
		var buffer bytes.Buffer
		buffer.WriteByte(byte(v.Type()))
		if _, err := v.WriteTo(&buffer); err != nil {
			return err
		}
		if err := txn.Set(toAccountKey(k), buffer.Bytes()); err != nil {
			return err
		}
		if err := txn.Set(toAccountNameKey(v.Name()), k[:]); err != nil {
			return err
		}
	}
	for k, v := range ctd.CreatedAccountMap {
		var buffer bytes.Buffer
		buffer.WriteByte(byte(v.Type()))
		if _, err := v.WriteTo(&buffer); err != nil {
			return err
		}
		if err := txn.Set(toAccountKey(k), buffer.Bytes()); err != nil {
			return err
		}
	}
	for k := range ctd.DeletedAccountMap {
		if err := txn.Delete(toAccountKey(k)); err != nil {
			return err
		}
		if err := txn.Delete(toAccountBalanceKey(k)); err != nil {
			return err
		}
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := toAccountDataKey(string(k[:]))
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			if err := txn.Delete(item.Key()); err != nil {
				return err
			}
		}
	}
	for k, v := range ctd.AccountDataMap {
		if err := txn.Set(toAccountDataKey(k), []byte(v)); err != nil {
			return err
		}
	}
	for k := range ctd.DeletedAccountDataMap {
		if err := txn.Delete(toAccountDataKey(k)); err != nil {
			return err
		}
	}
	for k, v := range ctd.UTXOMap {
		var buffer bytes.Buffer
		if v.TxIn.ID() != k {
			return ErrInvalidTxInKey
		}
		if _, err := v.TxOut.WriteTo(&buffer); err != nil {
			return err
		}
		if err := txn.Set(toUTXOKey(k), buffer.Bytes()); err != nil {
			return err
		}
	}
	for k, v := range ctd.CreatedUTXOMap {
		var buffer bytes.Buffer
		if _, err := v.WriteTo(&buffer); err != nil {
			return err
		}
		if err := txn.Set(toUTXOKey(k), buffer.Bytes()); err != nil {
			return err
		}
	}
	for k := range ctd.DeletedUTXOMap {
		if err := txn.Delete(toUTXOKey(k)); err != nil {
			return err
		}
	}
	for _, v := range ctd.Events {
		var buffer bytes.Buffer
		if _, err := buffer.Write(util.Uint64ToBytes(uint64(v.Type()))); err != nil {
			return err
		}
		if _, err := v.WriteTo(&buffer); err != nil {
			return err
		}
		if err := txn.Set(toEventKey(event.MarshalID(v.Coord(), v.Index())), buffer.Bytes()); err != nil {
			return err
		}
	}
	return nil
}
