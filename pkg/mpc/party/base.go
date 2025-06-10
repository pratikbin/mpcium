package party

import (
	"context"
	"encoding/json"
	"math/big"

	"github.com/bnb-chain/tss-lib/v2/tss"
	"github.com/fystack/mpcium/pkg/types"
)

type party struct {
	walletID   string
	threshold  int
	localParty tss.Party
	partyID    *tss.PartyID
	partyIDs   []*tss.PartyID
	inCh       chan types.TssMessage
	outCh      chan tss.Message
	errCh      chan error
}

type PartyInterface interface {
	StartKeygen(ctx context.Context, send func(tss.Message), finish func([]byte))
	StartSigning(ctx context.Context, msg *big.Int, send func(tss.Message), finish func([]byte))
	StartReshare(ctx context.Context, oldPartyIDs, newPartyIDs []*tss.PartyID, oldThreshold, newThreshold int, send func(tss.Message), finish func([]byte))

	PartyID() *tss.PartyID
	Party() tss.Party
	InCh() chan types.TssMessage
	OutCh() chan tss.Message
	ErrCh() chan error
}

func NewParty(walletID string, partyID *tss.PartyID, partyIDs []*tss.PartyID, threshold int, errCh chan error) *party {
	inCh := make(chan types.TssMessage, 1000)
	outCh := make(chan tss.Message, 1000)
	return &party{walletID, threshold, nil, partyID, partyIDs, inCh, outCh, errCh}
}

func (p *party) PartyID() *tss.PartyID {
	return p.partyID
}

func (p *party) Party() tss.Party {
	return p.localParty
}

func (p *party) InCh() chan types.TssMessage {
	return p.inCh
}

func (p *party) OutCh() chan tss.Message {
	return p.outCh
}

func (p *party) ErrCh() chan error {
	return p.errCh
}

// runParty handles the common party execution loop
func runParty[T any](s PartyInterface, ctx context.Context, party tss.Party, send func(tss.Message), endCh <-chan T, finish func([]byte)) {
	go func() {
		if err := party.Start(); err != nil {
			s.ErrCh() <- err
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case in := <-s.InCh():
			ok, err := party.UpdateFromBytes(in.MsgBytes, in.From, in.IsBroadcast)
			if !ok || err != nil {
				s.ErrCh() <- err
				return
			}
		case msg := <-s.OutCh():
			send(msg)
		case result := <-endCh:
			bz, err := json.Marshal(result)
			if err != nil {
				s.ErrCh() <- err
				return
			}
			finish(bz)
			return
		}
	}
}
