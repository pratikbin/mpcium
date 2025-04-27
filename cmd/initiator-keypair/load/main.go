package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
)

// Identity struct must match your identity file format
type Identity struct {
	NodeName  string `json:"node_name"`
	PublicKey string `json:"public_key"`
	CreatedAt string `json:"created_at"`
}

func main() {
	nodeName := "event_initiator" // Or load from CLI args/env var if needed

	// Load private key hex
	privHexBytes, err := os.ReadFile(fmt.Sprintf("%s.key", nodeName))
	if err != nil {
		fmt.Println("Failed to read private key file:", err)
		os.Exit(1)
	}

	privHex := string(privHexBytes)
	privHex = trimWhitespace(privHex)

	// Decode private key from hex
	privSeed, err := hex.DecodeString(privHex)
	if err != nil {
		fmt.Println("Failed to decode private key hex:", err)
		os.Exit(1)
	}

	// Reconstruct full Ed25519 private key from seed
	privateKey := ed25519.NewKeyFromSeed(privSeed)

	// Load identity JSON
	identityBytes, err := os.ReadFile(fmt.Sprintf("%s.identity.json", nodeName))
	if err != nil {
		fmt.Println("Failed to read identity JSON file:", err)
		os.Exit(1)
	}

	var identity Identity
	if err := json.Unmarshal(identityBytes, &identity); err != nil {
		fmt.Println("Failed to unmarshal identity JSON:", err)
		os.Exit(1)
	}

	// Decode public key from hex
	publicKeyBytes, err := hex.DecodeString(identity.PublicKey)
	if err != nil {
		fmt.Println("Failed to decode public key from identity JSON:", err)
		os.Exit(1)
	}
	publicKey := ed25519.PublicKey(publicKeyBytes)

	fmt.Println("✅ Successfully loaded identity and keys!")
	fmt.Println("Node Name:", identity.NodeName)
	fmt.Println("Created At:", identity.CreatedAt)
	fmt.Printf("Private Key: %x\n", privateKey.Seed())
	fmt.Printf("Public Key:  %x\n", publicKey)

	// Optional: verify that private key matches public key
	if !privateMatchesPublic(privateKey, publicKey) {
		fmt.Println("❌ WARNING: private key does NOT match public key!")
	} else {
		fmt.Println("✅ Private key matches public key!")
	}
}

func privateMatchesPublic(priv ed25519.PrivateKey, pub ed25519.PublicKey) bool {
	return string(priv.Public().(ed25519.PublicKey)) == string(pub)
}

func trimWhitespace(s string) string {
	return string([]byte(s))
}
