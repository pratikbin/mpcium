package party

import (
	"context"
	"errors"
	"math/big"

	"github.com/bnb-chain/tss-lib/v2/common"
	"github.com/bnb-chain/tss-lib/v2/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/v2/ecdsa/resharing"
	"github.com/bnb-chain/tss-lib/v2/ecdsa/signing"
	"github.com/bnb-chain/tss-lib/v2/tss"
)

type ECDSAParty struct {
	party
	prepareParams keygen.LocalPreParams
	reshareParams *tss.ReSharingParameters
	saveData      *keygen.LocalPartySaveData
	outCh         chan tss.Message
}

func NewECDSAParty(walletID string, partyID *tss.PartyID, partyIDs []*tss.PartyID, threshold int,
	prepareParams keygen.LocalPreParams, reshareParams *tss.ReSharingParameters, errCh chan error) *ECDSAParty {
	return &ECDSAParty{
		party:         *NewParty(walletID, partyID, partyIDs, threshold, errCh),
		prepareParams: prepareParams,
		reshareParams: reshareParams,
		outCh:         make(chan tss.Message, 1000),
	}
}

func (s *ECDSAParty) StartKeygen(ctx context.Context, send func(tss.Message), finish func([]byte)) {
	end := make(chan *keygen.LocalPartySaveData)
	params := tss.NewParameters(tss.S256(), tss.NewPeerContext(s.partyIDs), s.partyID, len(s.partyIDs), s.threshold)
	party := keygen.NewLocalParty(params, s.outCh, end, s.prepareParams)
	runParty(s, ctx, party, send, end, finish)
}

func (s *ECDSAParty) StartSigning(ctx context.Context, msg *big.Int, send func(tss.Message), finish func([]byte)) {
	if s.saveData == nil {
		s.ErrCh() <- errors.New("save data is nil")
		return
	}
	end := make(chan *common.SignatureData)
	params := tss.NewParameters(tss.S256(), tss.NewPeerContext(s.partyIDs), s.partyID, len(s.partyIDs), s.threshold)
	party := signing.NewLocalParty(msg, params, *s.saveData, s.outCh, end)
	runParty(s, ctx, party, send, end, finish)
}

func (s *ECDSAParty) StartReshare(ctx context.Context, oldPartyIDs, newPartyIDs []*tss.PartyID,
	oldThreshold, newThreshold int, send func(tss.Message), finish func([]byte)) {
	if s.saveData == nil {
		s.ErrCh() <- errors.New("save data is nil")
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
