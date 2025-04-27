package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Peers structure (from peers.json)
type Peers map[string]string

// Identity structure (for identity.json)
type Identity struct {
	NodeName  string `json:"node_name"`
	NodeID    string `json:"node_id"`
	PublicKey string `json:"public_key"` // Hex-encoded
	CreatedAt string `json:"created_at"`
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("Usage: go run main.go <node_name>")
	}
	nodeName := os.Args[1]

	// Load peers.json
	peersData, err := os.ReadFile("peers.json")
	if err != nil {
		log.Fatalf("failed to read peers.json: %v", err)
	}
	var peers Peers
	if err := json.Unmarshal(peersData, &peers); err != nil {
		log.Fatalf("failed to parse peers.json: %v", err)
	}

	// Lookup node UUID
	nodeUUID, ok := peers[nodeName]
	if !ok {
		log.Fatalf("node %s not found in peers.json", nodeName)
	}

	// Generate Ed25519 keypair
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatalf("failed to generate keypair: %v", err)
	}

	// Prepare Identity struct
	identity := Identity{
		NodeName:  nodeName,
		NodeID:    nodeUUID,
		PublicKey: hex.EncodeToString(pubKey),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Create output folder
	outputDir := "identity"
	err = os.MkdirAll(outputDir, 0700)
	if err != nil {
		log.Fatalf("failed to create identity directory: %v", err)
	}

	// Save private key
	privateKeyPath := filepath.Join(outputDir, fmt.Sprintf("%s_private.key", nodeName))
	privateKeyHex := hex.EncodeToString(privKey)
	err = os.WriteFile(privateKeyPath, []byte(privateKeyHex), 0600)
	if err != nil {
		log.Fatalf("failed to write private key: %v", err)
	}

	// Save identity.json
	identityPath := filepath.Join(outputDir, fmt.Sprintf("%s_identity.json", nodeName))
	identityBytes, err := json.MarshalIndent(identity, "", "  ")
	if err != nil {
		log.Fatalf("failed to marshal identity: %v", err)
	}
	err = os.WriteFile(identityPath, identityBytes, 0644)
	if err != nil {
		log.Fatalf("failed to write identity.json: %v", err)
	}

	fmt.Println("Identity generation complete!")
	fmt.Println("- Private key saved to:", privateKeyPath)
	fmt.Println("- Identity JSON saved to:", identityPath)
}
