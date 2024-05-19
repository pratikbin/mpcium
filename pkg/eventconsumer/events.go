package eventconsumer

type KeyType string

const (
	KeyTypeSecp256k1 KeyType = "secp256k1"
	KeyTypeEd25519           = "ed25519"
)

type SignTxMessage struct {
	KeyType             KeyType `json:"key_type"`
	WalletID            string  `json:"wallet_id"`
	NetworkInternalCode string  `json:"network_internal_code"`
	TxID                string  `json:"tx_id"`
	Tx                  []byte  `json:"tx"`
}
