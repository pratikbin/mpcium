package encoding

import (
	"crypto/ecdsa"
	"math/big"

	"github.com/btcsuite/btcd/btcec/v2"
)

func EncodeS256PubKey(pubKey *ecdsa.PublicKey) ([]byte, error) {
	publicKeyBytes := append(pubKey.X.Bytes(), pubKey.Y.Bytes()...)
	return publicKeyBytes, nil
}

func DecodeECDSAPubKey(encodedKey []byte) (*ecdsa.PublicKey, error) {
	xBytes := encodedKey[:32] // 32 bytes for X coordinate
	yBytes := encodedKey[32:] // 32 bytes for Y coordinate
	x := new(big.Int).SetBytes(xBytes)
	y := new(big.Int).SetBytes(yBytes)
	return &ecdsa.PublicKey{Curve: btcec.S256(), X: x, Y: y}, nil
}
