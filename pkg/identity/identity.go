package identity

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bnb-chain/tss-lib/v2/tss"
	"github.com/cryptoniumX/mpcium/pkg/types"
)

// NodeIdentity represents a node's identity information
type NodeIdentity struct {
	NodeName  string `json:"node_name"`
	NodeID    string `json:"node_id"`
	PublicKey string `json:"public_key"`
	CreatedAt string `json:"created_at"`
}

// Store manages node identities
type Store interface {
	// GetPublicKey retrieves a node's public key by its ID
	GetPublicKey(nodeID string) ([]byte, error)

	SignMessage(msg *types.TssMessage) ([]byte, error)
	VerifyMessage(msg *types.TssMessage) error
}

// fileStore implements the Store interface using the filesystem
type fileStore struct {
	identityDir     string
	currentNodeName string

	// Cache for public keys by node_id
	publicKeys map[string][]byte
	mu         sync.RWMutex

	// Cached private key
	privateKey []byte
}

// NewFileStore creates a new identity store
func NewFileStore(identityDir, nodeName string) (*fileStore, error) {
	if err := os.MkdirAll(identityDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create identity directory: %w", err)
	}

	// Load private key immediately
	privateKeyPath := filepath.Join(identityDir, fmt.Sprintf("%s_private.key", nodeName))
	privateKeyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	privateKey, err := hex.DecodeString(string(privateKeyData))
	if err != nil {
		return nil, fmt.Errorf("invalid private key format: %w", err)
	}

	store := &fileStore{
		identityDir:     identityDir,
		currentNodeName: nodeName,
		publicKeys:      make(map[string][]byte),
		privateKey:      privateKey,
	}

	// Load all identity files
	files, err := os.ReadDir(identityDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read identity directory: %w", err)
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), "_identity.json") {
			path := filepath.Join(identityDir, file.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("failed to read identity file %s: %w", file.Name(), err)
			}

			var identity NodeIdentity
			if err := json.Unmarshal(data, &identity); err != nil {
				return nil, fmt.Errorf("failed to parse identity file %s: %w", file.Name(), err)
			}

			key, err := hex.DecodeString(identity.PublicKey)
			if err != nil {
				return nil, fmt.Errorf("invalid public key format in %s: %w", file.Name(), err)
			}

			store.publicKeys[identity.NodeID] = key
		}
	}

	return store, nil
}

// GetPublicKey retrieves a node's public key by its ID
func (s *fileStore) GetPublicKey(nodeID string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if key, exists := s.publicKeys[nodeID]; exists {
		return key, nil
	}

	return nil, fmt.Errorf("public key not found for node ID: %s", nodeID)
}

func (s *fileStore) SignMessage(msg *types.TssMessage) ([]byte, error) {
	// Get deterministic bytes for signing
	msgBytes, err := msg.MarshalForSigning()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message for signing: %w", err)
	}

	signature := ed25519.Sign(s.privateKey, msgBytes)
	return signature, nil
}

// VerifyMessage verifies a TSS message's signature using the sender's public key
func (s *fileStore) VerifyMessage(msg *types.TssMessage) error {
	if msg.Signature == nil {
		return fmt.Errorf("message has no signature")
	}

	// Get the sender's NodeID
	senderNodeID := partyIDToNodeID(msg.From)

	// Get the sender's public key
	publicKey, err := s.GetPublicKey(senderNodeID)
	if err != nil {
		return fmt.Errorf("failed to get sender's public key: %w", err)
	}

	// Get deterministic bytes for verification
	msgBytes, err := msg.MarshalForSigning()
	if err != nil {
		return fmt.Errorf("failed to marshal message for verification: %w", err)
	}

	// Verify the signature
	if !ed25519.Verify(publicKey, msgBytes, msg.Signature) {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

func partyIDToNodeID(partyID *tss.PartyID) string {
	return string(partyID.KeyInt().Bytes())
}
