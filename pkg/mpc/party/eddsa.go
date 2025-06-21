package party

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/bnb-chain/tss-lib/v2/common"
	"github.com/bnb-chain/tss-lib/v2/eddsa/keygen"
	"github.com/bnb-chain/tss-lib/v2/eddsa/resharing"
	"github.com/bnb-chain/tss-lib/v2/eddsa/signing"
	"github.com/bnb-chain/tss-lib/v2/tss"
	"github.com/fystack/mpcium/pkg/logger"
	"github.com/fystack/mpcium/pkg/monitoring"
	"github.com/golang/protobuf/ptypes/any"
	"google.golang.org/protobuf/proto"
)

type EDDSAParty struct {
	party
	reshareParams    *tss.ReSharingParameters
	saveData         *keygen.LocalPartySaveData
	KeygenStart      time.Time
	KeygenCompletion time.Time
}

func NewEDDSAParty(walletID string, partyID *tss.PartyID, partyIDs []*tss.PartyID, threshold int,
	reshareParams *tss.ReSharingParameters, saveData *keygen.LocalPartySaveData, errCh chan error) *EDDSAParty {
	return &EDDSAParty{
		party:         *NewParty(walletID, partyID, partyIDs, threshold, errCh),
		reshareParams: reshareParams,
		saveData:      saveData,
	}
}

func (s *EDDSAParty) GetSaveData() []byte {
	saveData, err := json.Marshal(s.saveData)
	if err != nil {
		s.ErrCh() <- fmt.Errorf("failed serializing shares: %w", err)
		return nil
	}
	return saveData
}

func (s *EDDSAParty) SetSaveData(shareData []byte) {
	var localSaveData keygen.LocalPartySaveData
	err := json.Unmarshal(shareData, &localSaveData)
	if err != nil {
		s.ErrCh() <- fmt.Errorf("failed deserializing shares: %w", err)
		return
	}
	s.saveData = &localSaveData
}

func (s *EDDSAParty) ClassifyMsg(msgBytes []byte) (uint8, bool, error) {
	msg := &any.Any{}
	if err := proto.Unmarshal(msgBytes, msg); err != nil {
		return 0, false, err
	}

	_, isBroadcast := eddsaBroadcastMessages[msg.TypeUrl]

	round := eddsaMsgURL2Round[msg.TypeUrl]
	if round > 4 {
		round = round - 4
	}
	return round, isBroadcast, nil
}

func (s *EDDSAParty) GetMsgType(msgBytes []byte) (string, error) {
	msg := &any.Any{}
	if err := proto.Unmarshal(msgBytes, msg); err != nil {
		return "", err
	}
	return msg.TypeUrl, nil
}

func (s *EDDSAParty) StartKeygen(ctx context.Context, send func(tss.Message), finish func([]byte)) {
	end := make(chan *keygen.LocalPartySaveData, 1)

	// Measure time to initialize the party
	s.KeygenStart = time.Now()
	params := tss.NewParameters(tss.Edwards(), tss.NewPeerContext(s.partyIDs), s.partyID, len(s.partyIDs), s.threshold)
	party := keygen.NewLocalParty(params, s.outCh, end)
	initElapsed := time.Since(s.KeygenStart)

	logger.Info("[Starting EDDSA] key generation",
		"walletID", s.walletID,
		"initElapsed", initElapsed.Milliseconds(),
		"startTime", s.KeygenStart.Format(time.RFC3339),
	)

	// Measure time to run the party
	runStart := time.Now()
	runParty(s, ctx, party, send, end, finish)
	s.KeygenCompletion = time.Now()
	runElapsed := time.Since(runStart)

	logger.Info("[Finished EDDSA] key generation run",
		"walletID", s.walletID,
		"runElapsed", runElapsed.Milliseconds(),
		"completionTime", s.KeygenCompletion.Format(time.RFC3339),
	)

	// Record the completion event
	monitoring.RecordKeygenCompletion(monitoring.KeygenTimestamps{
		WalletID:       s.walletID,
		NodeID:         s.partyID.Id,
		KeyType:        "EDDSA",
		StartTime:      s.KeygenStart,
		CompletionTime: s.KeygenCompletion,
		InitDurationMs: initElapsed.Milliseconds(),
		RunDurationMs:  runElapsed.Milliseconds(),
	})
}

func (s *EDDSAParty) StartSigning(ctx context.Context, msg *big.Int, send func(tss.Message), finish func([]byte)) {
	if s.saveData == nil {
		s.ErrCh() <- errors.New("save data is nil")
		return
	}
	end := make(chan *common.SignatureData, 1)
	params := tss.NewParameters(tss.Edwards(), tss.NewPeerContext(s.partyIDs), s.partyID, len(s.partyIDs), s.threshold)
	party := signing.NewLocalParty(msg, params, *s.saveData, s.outCh, end)
	runParty(s, ctx, party, send, end, finish)
}

func (s *EDDSAParty) StartResharing(ctx context.Context, oldPartyIDs, newPartyIDs []*tss.PartyID,
	oldThreshold, newThreshold int, send func(tss.Message), finish func([]byte)) {
	if s.saveData == nil {
		s.ErrCh() <- errors.New("save data is nil")
		return
	}
	end := make(chan *keygen.LocalPartySaveData, 1)
	params := tss.NewReSharingParameters(
		tss.Edwards(),
		tss.NewPeerContext(oldPartyIDs),
		tss.NewPeerContext(newPartyIDs),
		s.partyID,
		len(oldPartyIDs),
		oldThreshold,
		len(newPartyIDs),
		newThreshold,
	)
	party := resharing.NewLocalParty(params, *s.saveData, s.outCh, end)
	runParty(s, ctx, party, send, end, finish)
}
