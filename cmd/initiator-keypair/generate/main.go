package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"runtime"
	"time"
)

// Identity struct to store node metadata
type Identity struct {
	NodeName    string `json:"node_name"`
	PublicKey   string `json:"public_key"`
	CreatedAt   string `json:"created_at"`
	CreatedBy   string `json:"created_by"`
	MachineOS   string `json:"machine_os"`
	MachineName string `json:"machine_name"`
}

func main() {
	nodeName := "event_initiator" // Or load from CLI args/env var if needed

	// Generate Ed25519 keypair
	pubKey, privKeyFull, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Println("Failed to generate Ed25519 keypair:", err)
		os.Exit(1)
	}

	// Extract 32-byte seed
	privKeySeed := privKeyFull.Seed()

	// Save private key in hex format
	privHex := hex.EncodeToString(privKeySeed)
	err = os.WriteFile(fmt.Sprintf("%s.key", nodeName), []byte(privHex), 0600)
	if err != nil {
		fmt.Println("Failed to save private key:", err)
		os.Exit(1)
	}

	// Get current user
	currentUser, err := user.Current()
	if err != nil {
		fmt.Println("Failed to get current user:", err)
		os.Exit(1)
	}

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Create Identity object
	identity := Identity{
		NodeName:    nodeName,
		PublicKey:   hex.EncodeToString(pubKey),
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		CreatedBy:   currentUser.Username,
		MachineOS:   runtime.GOOS,
		MachineName: hostname,
	}

	// Save identity JSON
	identityBytes, err := json.MarshalIndent(identity, "", "  ")
	if err != nil {
		fmt.Println("Failed to marshal identity JSON:", err)
		os.Exit(1)
	}

	err = os.WriteFile(fmt.Sprintf("%s.identity.json", nodeName), identityBytes, 0644)
	if err != nil {
		fmt.Println("Failed to save identity file:", err)
		os.Exit(1)
	}

	fmt.Println("✅ Successfully generated:")
	fmt.Println("- Private Key:", fmt.Sprintf("%s.key", nodeName))
	fmt.Println("- Identity JSON:", fmt.Sprintf("%s.identity.json", nodeName))
}
