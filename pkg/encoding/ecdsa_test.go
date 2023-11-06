package encoding

import (
	"log"
	"testing"

	"github.com/btcsuite/btcd/btcec"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeKey(t *testing.T) {
	// generate public key with ecdsa
	privateKey, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		log.Fatal("Error generating private key:", err)
	}

	// Extract the public key from the private key
	publicKey := &privateKey.PublicKey

	encodedKey, err := EncodeS256PubKey(publicKey)
	assert.NoError(t, err)
	assert.NotNil(t, encodedKey)

	decodedPubKey, err := DecodeECDSAPubKey(encodedKey)
	assert.NoError(t, err)
	assert.Equal(t, publicKey.X, decodedPubKey.X)
	assert.Equal(t, publicKey.Y, decodedPubKey.Y)
}
