package event

const (
	KeygenSuccessEventTopic    = "mpc.keygen.success.*"
	ResharingSuccessEventTopic = "mpc.resharing.success.*"
)

type KeygenSuccessEvent struct {
	WalletID    string `json:"wallet_id"`
	ECDSAPubKey []byte `json:"ecdsa_pub_key"`
	EDDSAPubKey []byte `json:"eddsa_pub_key"`
}

type SigningResultEvent struct {
	ResultType          SigningResultType `json:"result_type"`
	ErrorReason         string            `json:"error_reason"`
	IsTimeout           bool              `json:"is_timeout"`
	NetworkInternalCode string            `json:"network_internal_code"`
	WalletID            string            `json:"wallet_id"`
	TxID                string            `json:"tx_id"`
	R                   []byte            `json:"r"`
	S                   []byte            `json:"s"`
	SignatureRecovery   []byte            `json:"signature_recovery"`

	// TODO: define two separate events for eddsa and ecdsa
	Signature []byte `json:"signature"`
}

type SigningResultSuccessEvent struct {
	NetworkInternalCode string `json:"network_internal_code"`
	WalletID            string `json:"wallet_id"`
	TxID                string `json:"tx_id"`
	R                   []byte `json:"r"`
	S                   []byte `json:"s"`
	SignatureRecovery   []byte `json:"signature_recovery"`

	// TODO: define two separate events for eddsa and ecdsa
	Signature []byte `json:"signature"`
}

type SigningResultErrorEvent struct {
	NetworkInternalCode string `json:"network_internal_code"`
	WalletID            string `json:"wallet_id"`
	TxID                string `json:"tx_id"`
	ErrorReason         string `json:"error_reason"`
	IsTimeout           bool   `json:"is_timeout"`
}

type ResharingSuccessEvent struct {
	WalletID    string `json:"wallet_id"`
	ECDSAPubKey []byte `json:"ecdsa_pub_key"`
	EDDSAPubKey []byte `json:"eddsa_pub_key"`
}
