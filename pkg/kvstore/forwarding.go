package kvstore

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/bnb-chain/tss-lib/v2/ecdsa/keygen"
	"github.com/fystack/mpcium/pkg/logger"
	"golang.org/x/crypto/ripemd160"
)

// ForwardingKVStore forwards keyshares to a relayer service instead of storing them locally.
// It implements the KVStore interface.
type ForwardingKVStore struct {
	// Underlying store for non-keyshare data
	baseStore KVStore
	// URL of the relayer service to forward keyshares to
	relayerURL string
	// In-memory storage for keyshares (for debugging purposes)
	keyshareCache map[string][]byte
}

// NewForwardingKVStore creates a new ForwardingKVStore
func NewForwardingKVStore(baseStore KVStore, relayerURL string) *ForwardingKVStore {
	return &ForwardingKVStore{
		baseStore:     baseStore,
		relayerURL:    relayerURL,
		keyshareCache: make(map[string][]byte),
	}
}

// Put forwards keyshares to the relayer service or stores other data in the base store
func (f *ForwardingKVStore) Put(key string, value []byte) error {
	// Log every key being stored to help with debugging
	logger.Info("ForwardingKVStore.Put called", "key", key, "value_size", len(value), "is_user_node", true, "relayer_url", f.relayerURL)

	// Check if this is a keyshare (keys prefixed with "ecdsa:" or "eddsa:")
	if strings.HasPrefix(key, "ecdsa:") || strings.HasPrefix(key, "eddsa:") {
		logger.Info("KEYSHARE DETECTED - Forwarding keyshare to relayer", "key", key, "value_size", len(value))

		// DEBUG STEP 1: Store the keyshare in memory before forwarding to relayer
		f.keyshareCache[key] = make([]byte, len(value))
		copy(f.keyshareCache[key], value)
		logger.Info("DEBUG STEP 1: STORED KEYSHARE IN MEMORY", "key", key, "value_size", len(value))

		// Debug: Derive and log the Ethereum address for ECDSA keyshares
		if strings.HasPrefix(key, "ecdsa:") {
			// Extract wallet ID from key (format: "ecdsa:wallet_id")
			parts := strings.SplitN(key, ":", 2)
			walletID := parts[1]
			ethereumAddress := deriveEthereumAddressFromKeyshare(value, walletID)
			logger.Info("DEBUG: Derived Ethereum address during wallet creation",
				"wallet_id", walletID,
				"ethereum_address", ethereumAddress)
		}

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

// Get retrieves a value from the base store or relayer service
func (f *ForwardingKVStore) Get(key string) ([]byte, error) {
	// Check if this is a keyshare (keys prefixed with "ecdsa:" or "eddsa:")
	if strings.HasPrefix(key, "ecdsa:") || strings.HasPrefix(key, "eddsa:") {
		logger.Info("KEYSHARE DETECTED - Retrieving keyshare from relayer", "key", key)
		value, err := f.retrieveFromRelayer(key)
		if err != nil {
			logger.Error("Failed to retrieve keyshare from relayer", err, "key", key)
			return nil, err
		}
		logger.Info("Successfully retrieved keyshare from relayer", "key", key)
		return value, nil
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

	keyType := parts[0]  // ecdsa or eddsa
	walletID := parts[1] // wallet identifier

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

	// Construct the full URL by appending the endpoint path
	endpoint := f.relayerURL + "/mpc/key-share"

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

// retrieveFromRelayer fetches the keyshare from the relayer service
func (f *ForwardingKVStore) retrieveFromRelayer(key string) ([]byte, error) {
	// Extract wallet ID and key type from key
	parts := strings.SplitN(key, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid key format: %s", key)
	}

	keyType := parts[0]  // ecdsa or eddsa
	walletID := parts[1] // wallet identifier

	logger.Info("Preparing to retrieve keyshare", "key_type", keyType, "wallet_id", walletID, "relayer_url", f.relayerURL)

	// Set up HTTP client with reasonable timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Prepare payload for the reconstruct-key-share API
	payload := map[string]interface{}{
		"wallet_id": walletID,
		"key_type":  keyType,
		"node_id":   "node2",
		"threshold": 2, // Default threshold
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("error marshaling payload: %w", err)
	}

	// Construct the full URL for reconstructing the keyshare
	endpoint := f.relayerURL + "/mpc/reconstruct-key-share"

	logger.Info("Reconstructing keyshare from relayer endpoint", "endpoint", endpoint, "wallet_id", walletID, "key_type", keyType)

	resp, err := client.Post(endpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error reconstructing keyshare from relayer: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("relayer returned non-success status: %d", resp.StatusCode)
	}

	// Read the raw response body for debugging
	rawResponse, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	// Debug log the raw response
	logger.Info("Raw relayer response", "response", string(rawResponse), "status", resp.StatusCode, "content_length", len(rawResponse))

	// Parse the response
	var response struct {
		Status           string `json:"status"`
		Message          string `json:"message"`
		ReconstructedKey string `json:"reconstructed_key"`
		RequestID        string `json:"request_id"`
	}

	if err := json.Unmarshal(rawResponse, &response); err != nil {
		logger.Error("Failed to unmarshal relayer response", err, "raw_response", string(rawResponse))
		return nil, fmt.Errorf("error decoding relayer response: %w", err)
	}

	// Check if reconstruction was successful
	if response.Status != "success" {
		return nil, fmt.Errorf("relayer reconstruction failed: %s", response.Message)
	}

	// Decode the base64 reconstructed key
	reconstructedKeyBytes, err := base64.StdEncoding.DecodeString(response.ReconstructedKey)
	if err != nil {
		return nil, fmt.Errorf("error decoding base64 reconstructed key: %w", err)
	}

	logger.Info("Successfully reconstructed keyshare from relayer",
		"wallet_id", walletID,
		"key_type", keyType,
		"status", resp.StatusCode,
		"request_id", response.RequestID)

	// DEBUG STEP 2: Compare received keyshare with stored one
	if storedKeyshare, exists := f.keyshareCache[key]; exists {
		if len(storedKeyshare) == len(reconstructedKeyBytes) && bytes.Equal(storedKeyshare, reconstructedKeyBytes) {
			logger.Info("DEBUG STEP 2: KEYSHARE COMPARISON SUCCESS - SAME KEYSHARE USED",
				"key", key,
				"stored_size", len(storedKeyshare),
				"received_size", len(reconstructedKeyBytes))
		} else {
			logger.Error("DEBUG STEP 2: KEYSHARE COMPARISON FAILED - DIFFERENT KEYSHARES",
				nil,
				"key", key,
				"stored_size", len(storedKeyshare),
				"received_size", len(reconstructedKeyBytes),
				"stored_hash", fmt.Sprintf("%x", sha256.Sum256(storedKeyshare)),
				"received_hash", fmt.Sprintf("%x", sha256.Sum256(reconstructedKeyBytes)))
		}
	} else {
		logger.Warn("DEBUG STEP 2: NO STORED KEYSHARE FOUND FOR COMPARISON", "key", key)
	}

	// Debug: Derive and log the Ethereum address that will be used for signing
	if keyType == "ecdsa" {
		ethereumAddress := deriveEthereumAddressFromKeyshare(reconstructedKeyBytes, walletID)
		logger.Info("DEBUG: Derived Ethereum address for signing",
			"wallet_id", walletID,
			"ethereum_address", ethereumAddress)
	}

	return reconstructedKeyBytes, nil
}

// deriveEthereumAddressFromKeyshare derives an Ethereum address from TSS keyshare data
func deriveEthereumAddressFromKeyshare(keyshareBytes []byte, walletID string) string {
	// Try to unmarshal the keyshare data
	var data keygen.LocalPartySaveData
	err := json.Unmarshal(keyshareBytes, &data)
	if err != nil {
		logger.Error("Failed to unmarshal keyshare for address derivation", err, "wallet_id", walletID)
		return "ERROR: Could not parse keyshare"
	}

	// Extract the public key
	if data.ECDSAPub == nil {
		logger.Error("No ECDSA public key found in keyshare", nil, "wallet_id", walletID)
		return "ERROR: No public key in keyshare"
	}

	publicKey := *data.ECDSAPub
	pk := ecdsa.PublicKey{
		Curve: publicKey.Curve(),
		X:     publicKey.X(),
		Y:     publicKey.Y(),
	}

	// Derive Ethereum address
	address := deriveEthereumAddress(&pk)
	return address
}

// deriveEthereumAddress derives an Ethereum address from an ECDSA public key
func deriveEthereumAddress(pubKey *ecdsa.PublicKey) string {
	// Concatenate X and Y coordinates
	pubBytes := append(pubKey.X.Bytes(), pubKey.Y.Bytes()...)

	// SHA256 hash
	sha256Hash := sha256.Sum256(pubBytes)

	// RIPEMD160 hash
	ripemd160Hasher := ripemd160.New()
	ripemd160Hasher.Write(sha256Hash[:])
	ripemd160Hash := ripemd160Hasher.Sum(nil)

	// Add version byte (0x00 for mainnet)
	versionedHash := append([]byte{0x00}, ripemd160Hash...)

	// Double SHA256 for checksum
	checksum1 := sha256.Sum256(versionedHash)
	checksum2 := sha256.Sum256(checksum1[:])

	// Take first 4 bytes of checksum
	checksum := checksum2[:4]

	// Combine version + hash + checksum
	addressBytes := append(versionedHash, checksum...)

	// Base58 encode
	address := base58Encode(addressBytes)

	return address
}

// base58Encode encodes bytes to Base58 string
func base58Encode(input []byte) string {
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

	var result []byte
	x := new(big.Int).SetBytes(input)
	base := big.NewInt(58)
	zero := big.NewInt(0)

	for x.Cmp(zero) > 0 {
		mod := new(big.Int)
		x.DivMod(x, base, mod)
		result = append([]byte{alphabet[mod.Int64()]}, result...)
	}

	// Add leading zeros
	for _, b := range input {
		if b == 0 {
			result = append([]byte{'1'}, result...)
		} else {
			break
		}
	}

	return string(result)
}
