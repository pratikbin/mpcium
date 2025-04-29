package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/urfave/cli/v3"
)

// Identity structure (for identity.json)
type Identity struct {
	NodeName  string `json:"node_name"`
	NodeID    string `json:"node_id"`
	PublicKey string `json:"public_key"` // Hex-encoded
	CreatedAt string `json:"created_at"`
}

// GPG Key represents a user's GPG key
type GPGKey struct {
	ID          string
	UserID      string
	KeyType     string
	Fingerprint string
}

func generateIdentity(ctx context.Context, c *cli.Command) error {
	nodeName := c.String("node")
	peersPath := c.String("peers")
	identityDir := c.String("output-dir")
	encryptKey := c.Bool("encrypt")

	// Check if peers file exists
	if _, err := os.Stat(peersPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("peers file %s does not exist", peersPath)
		}
		return fmt.Errorf("error checking peers file: %w", err)
	}

	// Read peers file
	peersData, err := os.ReadFile(peersPath)
	if err != nil {
		return fmt.Errorf("failed to read peers file: %w", err)
	}

	// Parse peers JSON
	var peers map[string]string
	if err := json.Unmarshal(peersData, &peers); err != nil {
		return fmt.Errorf("failed to parse peers JSON: %w", err)
	}

	// Find the node ID
	nodeID, ok := peers[nodeName]
	if !ok {
		return fmt.Errorf("node %s not found in peers file", nodeName)
	}

	// If encryption is enabled, check for GPG
	var selectedKey GPGKey
	if encryptKey {
		// Check if GPG is installed
		gpgInstalled, err := isGPGInstalled()
		if err != nil {
			return fmt.Errorf("error checking for GPG: %w", err)
		}

		if !gpgInstalled {
			fmt.Println("GPG is not installed. Please install it:")
			printGPGInstallInstructions()
			return fmt.Errorf("GPG is required for identity encryption")
		}

		// List GPG keys
		gpgKeys, err := listGPGKeys()
		if err != nil {
			return fmt.Errorf("error listing GPG keys: %w", err)
		}

		if len(gpgKeys) == 0 {
			return fmt.Errorf("no GPG keys found. Please create a GPG key first")
		}

		// Display keys and let user choose one
		fmt.Println("Available GPG keys:")
		for i, key := range gpgKeys {
			fmt.Printf("%d) %s (%s)\n", i+1, key.UserID, key.ID)
		}

		var selectedIndex int
		fmt.Print("Select a key (1-" + fmt.Sprint(len(gpgKeys)) + "): ")
		fmt.Scanln(&selectedIndex)

		if selectedIndex < 1 || selectedIndex > len(gpgKeys) {
			return fmt.Errorf("invalid key selection")
		}

		selectedKey = gpgKeys[selectedIndex-1]
		fmt.Printf("Using key: %s\n", selectedKey.ID)
	} else {
		fmt.Println("WARNING: Private key will NOT be encrypted. This is not recommended for production environments.")
		fmt.Println("Use --encrypt flag to enable GPG encryption.")
	}

	// Create identity directory
	if err := os.MkdirAll(identityDir, 0755); err != nil {
		return fmt.Errorf("failed to create identity directory: %w", err)
	}

	// Generate identity for the node
	if err := generateNodeIdentity(nodeName, nodeID, identityDir, encryptKey, selectedKey); err != nil {
		return fmt.Errorf("failed to generate identity for %s: %w", nodeName, err)
	}

	fmt.Printf("Successfully generated identity files for %s\n", nodeName)
	return nil
}

// Check if GPG is installed
func isGPGInstalled() (bool, error) {
	_, err := exec.LookPath("gpg")
	if err != nil {
		return false, nil
	}
	return true, nil
}

// Print GPG installation instructions based on OS
func printGPGInstallInstructions() {
	switch runtime.GOOS {
	case "darwin":
		fmt.Println("For macOS:")
		fmt.Println("  brew install gnupg")
	case "linux":
		fmt.Println("For Ubuntu/Debian:")
		fmt.Println("  sudo apt-get install gnupg")
		fmt.Println("For Fedora:")
		fmt.Println("  sudo dnf install gnupg")
	case "windows":
		fmt.Println("For Windows:")
		fmt.Println("  1. Download GPG from https://gnupg.org/download/")
		fmt.Println("  2. Install Gpg4win")
	default:
		fmt.Println("Please install GPG for your operating system")
	}
}

// List available GPG keys
func listGPGKeys() ([]GPGKey, error) {
	cmd := exec.Command("gpg", "--list-keys", "--with-colons")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var keys []GPGKey
	lines := strings.Split(string(output), "\n")

	var currentKey GPGKey
	for _, line := range lines {
		parts := strings.Split(line, ":")
		if len(parts) < 10 {
			continue
		}

		switch parts[0] {
		case "pub":
			// Save the previous key if exists
			if currentKey.ID != "" {
				keys = append(keys, currentKey)
			}
			currentKey = GPGKey{
				ID:      parts[4],
				KeyType: parts[3],
			}
		case "fpr":
			if currentKey.ID != "" && currentKey.Fingerprint == "" {
				currentKey.Fingerprint = parts[9]
			}
		case "uid":
			if currentKey.ID != "" && currentKey.UserID == "" {
				currentKey.UserID = parts[9]
			}
		}
	}

	// Add the last key if exists
	if currentKey.ID != "" {
		keys = append(keys, currentKey)
	}

	return keys, nil
}

// Generate identity for a node
func generateNodeIdentity(nodeName, nodeID, identityDir string, encrypt bool, gpgKey GPGKey) error {
	// Generate Ed25519 keypair
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate keypair: %w", err)
	}

	// Prepare Identity struct
	identity := Identity{
		NodeName:  nodeName,
		NodeID:    nodeID,
		PublicKey: hex.EncodeToString(pubKey),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Save identity.json
	identityPath := filepath.Join(identityDir, fmt.Sprintf("%s_identity.json", nodeName))
	identityBytes, err := json.MarshalIndent(identity, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal identity: %w", err)
	}

	if err := os.WriteFile(identityPath, identityBytes, 0644); err != nil {
		return fmt.Errorf("failed to write identity JSON: %w", err)
	}

	// Convert private key to hex string
	privateKeyHex := hex.EncodeToString(privKey)

	if encrypt {
		// Encrypt private key with GPG - using a temporary file with secure cleanup
		privateKeyPath := filepath.Join(identityDir, fmt.Sprintf("%s_private.key.gpg", nodeName))

		// Create a temporary file with restricted permissions
		tempFile, err := os.CreateTemp(identityDir, "tmp-privkey-*.key")
		if err != nil {
			return fmt.Errorf("failed to create temporary file: %w", err)
		}
		tempKeyPath := tempFile.Name()

		// Ensure the temporary file gets deleted even if there's an error
		defer func() {
			tempFile.Close()
			os.Remove(tempKeyPath)
		}()

		// Write the private key to the temporary file
		if _, err := tempFile.WriteString(privateKeyHex); err != nil {
			return fmt.Errorf("failed to write to temporary file: %w", err)
		}

		// Close the file to ensure all data is written
		if err := tempFile.Close(); err != nil {
			return fmt.Errorf("failed to close temporary file: %w", err)
		}

		// Run GPG to encrypt the file
		cmd := exec.Command("gpg", "--batch", "--yes", "--trust-model", "always",
			"--recipient", gpgKey.ID, "--encrypt", "--output", privateKeyPath, tempKeyPath)

		// Capture any error output
		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("GPG encryption failed: %w (%s)", err, stderr.String())
		}

		fmt.Printf("Generated encrypted identity for %s: %s, %s\n", nodeName, identityPath, privateKeyPath)
	} else {
		// Save unencrypted private key
		privateKeyPath := filepath.Join(identityDir, fmt.Sprintf("%s_private.key", nodeName))
		if err := os.WriteFile(privateKeyPath, []byte(privateKeyHex), 0600); err != nil {
			return fmt.Errorf("failed to write private key: %w", err)
		}
		fmt.Printf("Generated unencrypted identity for %s: %s, %s\n", nodeName, identityPath, privateKeyPath)
	}

	return nil
}
