package party

var (
	ecdsaMsgURL2Round = map[string]uint8{
		// DKG
		"type.googleapis.com/binance.tsslib.ecdsa.keygen.KGRound1Message":  1,
		"type.googleapis.com/binance.tsslib.ecdsa.keygen.KGRound2Message1": 2,
		"type.googleapis.com/binance.tsslib.ecdsa.keygen.KGRound2Message2": 3,
		"type.googleapis.com/binance.tsslib.ecdsa.keygen.KGRound3Message":  4,

		// Signing
		"type.googleapis.com/binance.tsslib.ecdsa.signing.SignRound1Message1": 5,
		"type.googleapis.com/binance.tsslib.ecdsa.signing.SignRound1Message2": 6,
		"type.googleapis.com/binance.tsslib.ecdsa.signing.SignRound2Message":  7,
		"type.googleapis.com/binance.tsslib.ecdsa.signing.SignRound3Message":  8,
		"type.googleapis.com/binance.tsslib.ecdsa.signing.SignRound4Message":  9,
		"type.googleapis.com/binance.tsslib.ecdsa.signing.SignRound5Message":  10,
		"type.googleapis.com/binance.tsslib.ecdsa.signing.SignRound6Message":  11,
		"type.googleapis.com/binance.tsslib.ecdsa.signing.SignRound7Message":  12,
		"type.googleapis.com/binance.tsslib.ecdsa.signing.SignRound8Message":  13,
		"type.googleapis.com/binance.tsslib.ecdsa.signing.SignRound9Message":  14,

		// Resharing
		"type.googleapis.com/binance.tsslib.ecdsa.resharing.DGRound1Message":  15,
		"type.googleapis.com/binance.tsslib.ecdsa.resharing.DGRound2Message1": 16,
		"type.googleapis.com/binance.tsslib.ecdsa.resharing.DGRound2Message2": 17,
		"type.googleapis.com/binance.tsslib.ecdsa.resharing.DGRound3Message1": 18,
		"type.googleapis.com/binance.tsslib.ecdsa.resharing.DGRound3Message2": 19,
		"type.googleapis.com/binance.tsslib.ecdsa.resharing.DGRound4Message1": 20,
		"type.googleapis.com/binance.tsslib.ecdsa.resharing.DGRound4Message2": 21,
	}

	ecdsaBroadcastMessages = map[string]struct{}{
		// DKG
		"type.googleapis.com/binance.tsslib.ecdsa.keygen.KGRound1Message":  {},
		"type.googleapis.com/binance.tsslib.ecdsa.keygen.KGRound2Message2": {},
		"type.googleapis.com/binance.tsslib.ecdsa.keygen.KGRound3Message":  {},

		// Signing
		"type.googleapis.com/binance.tsslib.ecdsa.signing.SignRound1Message2": {},
		"type.googleapis.com/binance.tsslib.ecdsa.signing.SignRound3Message":  {},
		"type.googleapis.com/binance.tsslib.ecdsa.signing.SignRound4Message":  {},
		"type.googleapis.com/binance.tsslib.ecdsa.signing.SignRound5Message":  {},
		"type.googleapis.com/binance.tsslib.ecdsa.signing.SignRound6Message":  {},
		"type.googleapis.com/binance.tsslib.ecdsa.signing.SignRound7Message":  {},
		"type.googleapis.com/binance.tsslib.ecdsa.signing.SignRound8Message":  {},
		"type.googleapis.com/binance.tsslib.ecdsa.signing.SignRound9Message":  {},

		// Resharing
		"type.googleapis.com/binance.tsslib.ecdsa.resharing.DGRound1Message":  {},
		"type.googleapis.com/binance.tsslib.ecdsa.resharing.DGRound2Message1": {},
		"type.googleapis.com/binance.tsslib.ecdsa.resharing.DGRound2Message2": {},
		"type.googleapis.com/binance.tsslib.ecdsa.resharing.DGRound3Message1": {},
		"type.googleapis.com/binance.tsslib.ecdsa.resharing.DGRound4Message2": {},
	}
)
