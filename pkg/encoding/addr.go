package encoding

import (
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcutil"
)

func PublicKeyToBech32Address(publicKey *btcec.PublicKey, network *chaincfg.Params) (string, error) {
	// Create a P2WPKH (Pay-to-Witness-Public-Key-Hash) address from the public key
	address, err := btcutil.NewAddressWitnessPubKeyHash(btcutil.Hash160(publicKey.SerializeCompressed()), network)
	if err != nil {
		return "", err
	}
	return address.EncodeAddress(), nil
}

func PublicKeyToP2SHSegWitAddress(publicKey *btcec.PublicKey, network *chaincfg.Params) (string, error) {
	// Create a P2WSH (Pay-to-Witness-Script-Hash) script
	pubKeyHash := btcutil.Hash160(publicKey.SerializeCompressed())
	segwitAddress, err := btcutil.NewAddressWitnessPubKeyHash(pubKeyHash, network)

	if err != nil {
		return "", err
	}

	payToAddrScript, err := txscript.PayToAddrScript(segwitAddress)
	if err != nil {
		return "", err
	}

	// TODO:  Redeem script, have no idea about it
	address, err := btcutil.NewAddressScriptHash(payToAddrScript, network)
	if err != nil {
		return "", err
	}
	return address.String(), nil
}
