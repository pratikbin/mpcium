package encoding

import (
	"crypto/ecdsa"
)

func EncodeS256PubKey(pubKey *ecdsa.PublicKey) ([]byte, error) {
	publicKeyBytes := append(pubKey.X.Bytes(), pubKey.Y.Bytes()...)
	return publicKeyBytes, nil
}
