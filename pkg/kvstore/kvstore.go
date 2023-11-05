package kvstore

// KVStore defines the interface for a key-value store.
type KVStore interface {
	// Put stores a key-value pair in the store.
	Put(key string, value []byte) error

	// Get retrieves the value associated with a key. If the key is not found, it returns an empty slice.
	Get(key string) ([]byte, error)

	// Delete removes a key-value pair from the store.
	Delete(key string) error

	// Close closes the key-value store.
	Close() error
}
