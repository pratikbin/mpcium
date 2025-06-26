package party

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/bnb-chain/tss-lib/v2/tss"
	"github.com/fystack/mpcium/pkg/types"
)

type Party interface {
	StartKeygen(ctx context.Context, send func(tss.Message), onComplete func([]byte))
	StartSigning(ctx context.Context, msg *big.Int, send func(tss.Message), onComplete func([]byte))
	StartResharing(
		ctx context.Context,
		oldPartyIDs,
		newPartyIDs []*tss.PartyID,
		oldThreshold,
		newThreshold int,
		send func(tss.Message),
		onComplete func([]byte),
	)

	WalletID() string
	PartyID() *tss.PartyID
	PartyIDs() []*tss.PartyID
	GetSaveData() []byte
	SetSaveData(saveData []byte)
	ClassifyMsg(msgBytes []byte) (uint8, bool, error)
	InCh() chan types.TssMessage
	OutCh() chan tss.Message
	ErrCh() chan error
}

type party struct {
	walletID  string
	threshold int
	partyID   *tss.PartyID
	partyIDs  []*tss.PartyID
	inCh      chan types.TssMessage
	outCh     chan tss.Message
	errCh     chan error

	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
}

func NewParty(
	walletID string,
	partyID *tss.PartyID,
	partyIDs []*tss.PartyID,
	threshold int,
	errCh chan error,
) *party {
	return &party{
		walletID:  walletID,
		threshold: threshold,
		partyID:   partyID,
		partyIDs:  partyIDs,
		inCh:      make(chan types.TssMessage, 1000),
		outCh:     make(chan tss.Message, 1000),
		errCh:     errCh,
	}
}

func (p *party) WalletID() string {
	return p.walletID
}

func (p *party) PartyID() *tss.PartyID {
	return p.partyID
}

func (p *party) PartyIDs() []*tss.PartyID {
	return p.partyIDs
}

func (p *party) InCh() chan types.TssMessage { return p.inCh }
func (p *party) OutCh() chan tss.Message     { return p.outCh }
func (p *party) ErrCh() chan error           { return p.errCh }

// runParty handles the common party execution loop
// startPartyLoop runs a TSS party, handling messages, errors, and completion.
func runParty[T any](
	s Party,
	ctx context.Context,
	party tss.Party,
	send func(tss.Message),
	endCh <-chan T,
	onComplete func([]byte),
) {
	// safe error reporter
	safeErr := func(err error) {
		select {
		case s.ErrCh() <- err:
		case <-ctx.Done():
		}
	}

	// start the tss party logic
	go func() {
		defer func() {
			if r := recover(); r != nil {
				safeErr(fmt.Errorf("panic in party.Start: %v", r))
			}
		}()
		if err := party.Start(); err != nil {
			safeErr(err)
		}
	}()

	// main handling loop
	for {
		select {
		case <-ctx.Done():
			if ctx.Err() != context.Canceled {
				safeErr(fmt.Errorf("party timed out: %w", ctx.Err()))
			}
			return

		case inMsg, ok := <-s.InCh():
			if !ok {
				return
			}
			ok2, err := party.UpdateFromBytes(inMsg.MsgBytes, inMsg.From, inMsg.IsBroadcast)
			if err != nil || !ok2 {
				safeErr(errors.New("UpdateFromBytes failed"))
				return
			}

		case outMsg, ok := <-s.OutCh():
			if !ok {
				return
			}
			// respect cancellation before invoking callback
			if ctx.Err() != nil {
				return
			}
			send(outMsg)

		case result, ok := <-endCh:
			if !ok {
				return
			}
			bts, err := json.Marshal(result)
			if err != nil {
				safeErr(err)
				return
			}
			onComplete(bts)
			return
		}
	}
}
