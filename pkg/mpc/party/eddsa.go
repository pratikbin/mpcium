package party

import (
	"context"
	"errors"
	"math/big"

	"github.com/bnb-chain/tss-lib/v2/common"
	"github.com/bnb-chain/tss-lib/v2/eddsa/keygen"
	"github.com/bnb-chain/tss-lib/v2/eddsa/resharing"
	"github.com/bnb-chain/tss-lib/v2/eddsa/signing"
	"github.com/bnb-chain/tss-lib/v2/tss"
	"github.com/fystack/mpcium/pkg/logger"
)

type EDDSASession struct {
	party
	reshareParams *tss.ReSharingParameters
	saveData      *keygen.LocalPartySaveData
	outCh         chan tss.Message
}

func NewEDDASession(walletID string, partyID *tss.PartyID, partyIDs []*tss.PartyID, threshold int,
	reshareParams *tss.ReSharingParameters, saveData *keygen.LocalPartySaveData) *EDDSASession {
	return &EDDSASession{
		party:         *NewParty(walletID, partyID, partyIDs, threshold),
		reshareParams: reshareParams,
		saveData:      saveData,
		outCh:         make(chan tss.Message, 1000),
	}
}

func (s *EDDSASession) PartyID() *tss.PartyID {
	return s.partyID
}

func (s *EDDSASession) GetOutCh() chan tss.Message {
	return s.outCh
}

func (s *EDDSASession) UpdateFromBytes(msgBytes []byte, from *tss.PartyID, isBroadcast bool) (bool, error) {
	ok, err := s.localParty.UpdateFromBytes(msgBytes, from, isBroadcast)
	if err != nil {
		return false, err
	}
	return ok, nil
}

func (s *EDDSASession) StartKeygen(ctx context.Context, send func(tss.Message), finish func([]byte)) {
	end := make(chan *keygen.LocalPartySaveData)
	params := tss.NewParameters(tss.S256(), tss.NewPeerContext(s.partyIDs), s.partyID, len(s.partyIDs), s.threshold)
	party := keygen.NewLocalParty(params, s.outCh, end)
	runParty(s, ctx, party, send, end, finish)
}

func (s *EDDSASession) StartSigning(ctx context.Context, msg *big.Int, send func(tss.Message), finish func([]byte)) {
	if s.saveData == nil {
		logger.Error("Save data is nil", errors.New("save data is nil"))
		return
	}
	end := make(chan *common.SignatureData)
	params := tss.NewParameters(tss.S256(), tss.NewPeerContext(s.partyIDs), s.partyID, len(s.partyIDs), s.threshold)
	party := signing.NewLocalParty(msg, params, *s.saveData, s.outCh, end)
	runParty(s, ctx, party, send, end, finish)
}

func (s *EDDSASession) StartReshare(ctx context.Context, oldPartyIDs, newPartyIDs []*tss.PartyID,
	oldThreshold, newThreshold int, send func(tss.Message), finish func([]byte)) {
	if s.saveData == nil {
		logger.Error("Save data is nil", errors.New("save data is nil"))
		return
	}
	end := make(chan *keygen.LocalPartySaveData)
	params := tss.NewReSharingParameters(
		tss.S256(),
		tss.NewPeerContext(oldPartyIDs),
		tss.NewPeerContext(newPartyIDs),
		s.partyID,
		len(oldPartyIDs),
		len(newPartyIDs),
		oldThreshold,
		newThreshold,
	)
	party := resharing.NewLocalParty(params, *s.saveData, s.outCh, end)
	runParty(s, ctx, party, send, end, finish)
}
