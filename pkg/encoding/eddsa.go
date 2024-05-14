package encoding

import (
	"github.com/decred/dcrd/dcrec/edwards/v2"
)

func EncodeEDDSAPubKey(pubKey *edwards.PublicKey) ([]byte, error) {
	bytes := pubKey.SerializeCompressed()
	return bytes, nil
}

func DecodeEDDSAPubKey(encodedKey []byte) (*edwards.PublicKey, error) {
	pubKey, err := edwards.ParsePubKey(encodedKey)
	if err != nil {
		return nil, err
	}
	return pubKey, nil
}
