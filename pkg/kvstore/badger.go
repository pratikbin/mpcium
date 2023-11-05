package kvstore

import (
	"errors"

	"github.com/cryptoniumX/mpcium/pkg/logger"
	"github.com/dgraph-io/badger/v4"
	"github.com/dgraph-io/badger/v4/options"
)

var (
	ErrEncryptionKeyNotProvided = errors.New("encryption key not provided")
)

// BadgerKVStore is an implementation of the KVStore interface using BadgerDB.
type BadgerKVStore struct {
	db *badger.DB
}

// NewBadgerKVStore creates a new BadgerKVStore instance.
func NewBadgerKVStore(dbPath string, encryptionKey []byte) (*BadgerKVStore, error) {
	// must ensure encryption key is provided
	if len(encryptionKey) == 0 {
		return nil, ErrEncryptionKeyNotProvided
	}

	opts := badger.DefaultOptions(dbPath).WithCompression(options.ZSTD).WithEncryptionKey(encryptionKey).WithIndexCacheSize(100 << 20) // 100MB
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	logger.Info("Connected to BadgerDB successfully!", "path", dbPath)

	return &BadgerKVStore{db: db}, nil
}

// Put stores a key-value pair in the BadgerDB.
func (b *BadgerKVStore) Put(key string, value []byte) error {
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), value)
	})
}

// Get retrieves the value associated with a key from BadgerDB.
func (b *BadgerKVStore) Get(key string) ([]byte, error) {
	var result []byte
	err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err == nil {
			return item.Value(func(val []byte) error {
				result = append([]byte{}, val...)
				return nil
			})
		}
		return err
	})

	return result, err
}

// Delete removes a key-value pair from BadgerDB.
func (b *BadgerKVStore) Delete(key string) error {
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

// Close closes the BadgerDB.
func (b *BadgerKVStore) Close() error {
	return b.db.Close()
}
