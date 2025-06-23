package kvstore

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/fystack/mpcium/pkg/logger"
)

// ForwardingKVStore forwards keyshares to a relayer service instead of storing them locally.
// It implements the KVStore interface.
type ForwardingKVStore struct {
	// Underlying store for non-keyshare data
	baseStore KVStore
	// URL of the relayer service to forward keyshares to
	relayerURL string
}

// NewForwardingKVStore creates a new ForwardingKVStore
func NewForwardingKVStore(baseStore KVStore, relayerURL string) *ForwardingKVStore {
	return &ForwardingKVStore{
		baseStore:  baseStore,
		relayerURL: relayerURL,
	}
}

// Put forwards keyshares to the relayer service or stores other data in the base store
func (f *ForwardingKVStore) Put(key string, value []byte) error {
	// Log every key being stored to help with debugging
	logger.Info("ForwardingKVStore.Put called", "key", key, "value_size", len(value), "is_user_node", true, "relayer_url", f.relayerURL)

	// Check if this is a keyshare (keys prefixed with "ecdsa:" or "eddsa:")
	if strings.HasPrefix(key, "ecdsa:") || strings.HasPrefix(key, "eddsa:") {
		logger.Info("KEYSHARE DETECTED - Forwarding keyshare to relayer", "key", key, "value_size", len(value))
		err := f.forwardToRelayer(key, value)
		if err != nil {
			logger.Error("Failed to forward keyshare to relayer", err, "key", key)
			return err
		}
		logger.Info("Successfully forwarded keyshare", "key", key)
		return nil
	}
	
	// For non-keyshare data, store in the base store
	return f.baseStore.Put(key, value)
}

// Get retrieves a value from the base store
func (f *ForwardingKVStore) Get(key string) ([]byte, error) {
	// Key shares are not stored locally, so for those keys we should return an error
	if strings.HasPrefix(key, "ecdsa:") || strings.HasPrefix(key, "eddsa:") {
		return nil, fmt.Errorf("keyshare not stored locally (forwarded to relayer): %s", key)
	}
	
	// Regular data comes from the base store
	return f.baseStore.Get(key)
}

// Delete deletes a key from the base store
func (f *ForwardingKVStore) Delete(key string) error {
	return f.baseStore.Delete(key)
}

// Note: This KVStore implementation doesn't need a Keys method as it's not part of the KVStore interface

// Close closes the base store
func (f *ForwardingKVStore) Close() error {
	return f.baseStore.Close()
}

// forwardToRelayer sends the keyshare to the relayer service
func (f *ForwardingKVStore) forwardToRelayer(key string, value []byte) error {
	// Extract wallet ID and key type from key
	parts := strings.SplitN(key, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid key format: %s", key)
	}
	
	keyType := parts[0]   // ecdsa or eddsa
	walletID := parts[1]  // wallet identifier
	
	logger.Info("Preparing to forward keyshare", "key_type", keyType, "wallet_id", walletID, "relayer_url", f.relayerURL)
	
	// Prepare payload - match the relayer's expected format
	payload := map[string]interface{}{
		"wallet_id": walletID,
		"key_type":  keyType,
		"keyshare":  base64.StdEncoding.EncodeToString(value),
		"node_id":   "node2", // Required by the relayer API
	}
	
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshaling payload: %w", err)
	}
	
	// Set up HTTP client with reasonable timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	// Send to relayer
	// Don't append path if relayerURL already contains a path
	endpoint := f.relayerURL
	
	// Debug log the actual API endpoint being used
	logger.Info("Sending keyshare to relayer endpoint", "endpoint", endpoint, "wallet_id", walletID, "key_type", keyType)
	
	resp, err := client.Post(endpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error forwarding keyshare to relayer: %w", err)
	}
	defer resp.Body.Close()
	
	// Check response
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("relayer returned non-success status: %d", resp.StatusCode)
	}
	
	logger.Info("Successfully forwarded keyshare to relayer", "wallet_id", walletID, "key_type", keyType, "status", resp.StatusCode)
	return nil
}
