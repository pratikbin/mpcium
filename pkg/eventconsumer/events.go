package eventconsumer

type SignTxMessage struct {
	KeyType             string `json:"key_type"`
	WalletID            string `json:"wallet_id"`
	NetworkInternalCode string `json:"network_internal_code"`
	TxID                string `json:"tx_id"`
	Tx                  []byte `json:"tx"`
}
