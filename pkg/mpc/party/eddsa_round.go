package party

var (
	eddsaMsgURL2Round = map[string]uint8{
		// DKG
		"type.googleapis.com/binance.tsslib.eddsa.keygen.KGRound1Message":  1,
		"type.googleapis.com/binance.tsslib.eddsa.keygen.KGRound2Message1": 2,
		"type.googleapis.com/binance.tsslib.eddsa.keygen.KGRound2Message2": 3,

		// Signing
		"type.googleapis.com/binance.tsslib.eddsa.signing.SignRound1Message": 4,
		"type.googleapis.com/binance.tsslib.eddsa.signing.SignRound2Message": 5,
		"type.googleapis.com/binance.tsslib.eddsa.signing.SignRound3Message": 6,

		// Resharing
		"type.googleapis.com/binance.tsslib.eddsa.resharing.DGRound1Message":  7,
		"type.googleapis.com/binance.tsslib.eddsa.resharing.DGRound2Message":  8,
		"type.googleapis.com/binance.tsslib.eddsa.resharing.DGRound3Message1": 9,
		"type.googleapis.com/binance.tsslib.eddsa.resharing.DGRound3Message2": 10,
		"type.googleapis.com/binance.tsslib.eddsa.resharing.DGRound4Message":  11,
	}

	eddsaBroadcastMessages = map[string]struct{}{
		// DKG
		"type.googleapis.com/binance.tsslib.eddsa.keygen.KGRound1Message":  {},
		"type.googleapis.com/binance.tsslib.eddsa.keygen.KGRound2Message2": {},

		// Signing
		"type.googleapis.com/binance.tsslib.eddsa.signing.SignRound1Message": {},
		"type.googleapis.com/binance.tsslib.eddsa.signing.SignRound2Message": {},
		"type.googleapis.com/binance.tsslib.eddsa.signing.SignRound3Message": {},

		// Resharing
		"type.googleapis.com/binance.tsslib.eddsa.resharing.DGRound1Message": {},
		"type.googleapis.com/binance.tsslib.eddsa.resharing.DGRound2Message": {},
		"type.googleapis.com/binance.tsslib.eddsa.resharing.DGRound4Message": {},
	}
)
