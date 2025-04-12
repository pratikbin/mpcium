package event

const (
	SigningPublisherStream     = "mpc-signing"
	SigningConsumerStream      = "mpc-signing-consumer"
	SigningRequestTopic        = "mpc.signing_request.*"
	SigningResultTopic         = "mpc.signing_result.*"
	SigningResultCompleteTopic = "mpc.signing_result.complete"
	MPCSigningEventTopic       = "mpc:sign"
	SigningRequestEventTopic   = "mpc.signing_request.event"
)

type SigningResultType int

const (
	SigningResultTypeUnknown SigningResultType = iota
	SigningResultTypeSuccess
	SigningResultTypeError
)

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
