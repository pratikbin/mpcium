package mpc

import (
	"crypto/ecdsa"
	"fmt"
	"time"

	"github.com/bnb-chain/tss-lib/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/tss"
	"github.com/cryptoniumX/mpcium/pkg/messaging"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/fatih/color"

	"github.com/rs/zerolog/log"
)

type Session struct {
	walletID string
	pubSub   messaging.PubSub
	direct   messaging.DirectMessaging

	threshold   int
	selfPartyID *tss.PartyID

	// IDs of all parties in the session including self
	mapPartyIdToNodeId map[string]string
	partyIDs           []*tss.PartyID
	outCh              chan tss.Message
	endCh              chan keygen.LocalPartySaveData
	ErrCh              chan error

	party tss.Party

	// its ready when alls party emit ready
	readyCh chan string
}

func NewSession(
	walletID string,
	pubSub messaging.PubSub,
	direct messaging.DirectMessaging,
	selfID *tss.PartyID,
	partyIDs []*tss.PartyID,
	threshold int,
	mapPartyIdToNodeId map[string]string,
) *Session {
	fmt.Printf("mapPartyIdToNodeId = %+v\n", mapPartyIdToNodeId)
	return &Session{
		walletID:           walletID,
		pubSub:             pubSub,
		direct:             direct,
		threshold:          threshold,
		selfPartyID:        selfID,
		partyIDs:           partyIDs,
		outCh:              make(chan tss.Message),
		endCh:              make(chan keygen.LocalPartySaveData),
		ErrCh:              make(chan error),
		readyCh:            make(chan string, 3),
		mapPartyIdToNodeId: mapPartyIdToNodeId,
	}
}

func (s *Session) PartyID() *tss.PartyID {
	return s.selfPartyID
}

func (s *Session) PartyIDs() []*tss.PartyID {
	return s.partyIDs
}

func (s *Session) PartyCount() int {
	return len(s.partyIDs)
}

func (s *Session) composeReadyTopic() string {
	return fmt.Sprintf("%s-%s", s.walletID, "ready")
}

func (s *Session) Init() error {
	go func() {
		topic := s.composeReadyTopic()
		fmt.Println("Subscribing to topic", topic)
		s.pubSub.Subscribe(topic, func(data []byte) {
			fmt.Printf("string(data) = %+v\n", string(data))
			log.Info().Msgf("Received ready message for %s from %s", topic, string(data))
			s.readyCh <- string(data)
		})
	}()

	preParams, err := keygen.GeneratePreParams(1 * time.Minute)
	if err != nil {
		return err
	}

	log.Info().Msgf("Initializing session with partyID: %s, peerIDs %s", s.selfPartyID, s.partyIDs)
	ctx := tss.NewPeerContext(s.partyIDs)
	params := tss.NewParameters(tss.S256(), ctx, s.selfPartyID, len(s.partyIDs), s.threshold)
	s.party = keygen.NewLocalParty(params, s.outCh, s.endCh, *preParams)
	log.Info().Msg("Initialized session successfully")

	s.pubSub.Publish(s.composeReadyTopic(), []byte(PartyIDToNodeID(s.selfPartyID)))
	s.readyCh <- PartyIDToNodeID(s.selfPartyID)

	for i := 0; i < len(s.partyIDs); i++ {
		nodeID := <-s.readyCh
		fmt.Printf("nodeID is ready = %+v\n", nodeID)
	}

	color.Cyan("All parties are ready")

	return nil
}

func (s *Session) composeTargetID(nodeID string) string {
	return fmt.Sprintf("%s/%s", s.walletID, nodeID)
}

func (s *Session) handleMessage(keyshare tss.Message) {
	data, routing, err := keyshare.WireBytes()
	fmt.Printf("routing = %+v\n", routing)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal message")
		s.ErrCh <- err
		return
	}

	msg, err := MarshalTssMessage(s.walletID, data, routing.IsBroadcast, routing.From, routing.To)
	if err != nil {
		s.ErrCh <- err
		return
	}

	fmt.Printf("PREPARE TO SEND MESSAGE TO %v, is broadcast %v", routing.To, routing.IsBroadcast)
	fmt.Printf("walletID: %s\n", s.walletID)

	if routing.IsBroadcast && len(routing.To) == 0 {
		fmt.Println("[Routing] to == 0", routing)
		fmt.Println("Publishing to topic", s.walletID)
		err := s.pubSub.Publish(s.walletID, msg)
		if err != nil {
			s.ErrCh <- err
			return
		}
	} else {
		fmt.Println("[Routing] to > 0", routing)
		for _, to := range routing.To {
			nodeID := PartyIDToNodeID(to)
			fmt.Printf("[Topic]: %s\n", s.composeTargetID(nodeID))
			color.Cyan("Sending message to topic", s.composeTargetID(nodeID))
			err := s.direct.Send(s.composeTargetID(nodeID), msg)
			if err != nil {
				color.Red(err.Error())
				s.ErrCh <- err
			}

		}

	}
}

func (s *Session) GenerateKey() {
	fmt.Println("Waiting all parties to be ready")
	fmt.Println("All parties are ready, start generating key")

	go func() {
		if err := s.party.Start(); err != nil {
			s.ErrCh <- err
		}

	}()

	for {
		select {
		case msg := <-s.outCh:
			blue := color.New(color.FgBlue)
			blue.Printf("Received message from %v\n", msg)
			s.handleMessage(msg)
		case saveData := <-s.endCh:

			publicKey := saveData.ECDSAPub

			pubKey := &ecdsa.PublicKey{
				Curve: publicKey.Curve(),
				X:     publicKey.X(),
				Y:     publicKey.Y(),
			}

			address := crypto.PubkeyToAddress(*pubKey)
			fmt.Printf("address = %+v\n", address)

		}

	}

}

func (s *Session) ListenToIncomingMessage() {
	go func() {
		s.pubSub.Subscribe(s.walletID, func(msg []byte) {
			s.receiveMessage(msg)
		})

	}()

	nodeID := PartyIDToNodeID(s.selfPartyID)
	fmt.Println("Listening on target topic", s.composeTargetID(nodeID))

	targetID := s.composeTargetID(nodeID)
	s.direct.Listen(targetID, func(msg []byte) {
		color.Cyan("Received message from direct channel", targetID)
		s.receiveMessage(msg)
	})

}

func (s *Session) receiveMessage(rawMsg []byte) {
	fmt.Println("receive message")
	blue := color.New(color.FgGreen)
	msg, err := UnmarshalTssMessage(rawMsg)
	if err != nil {
		log.Error().Err(err).Msg("failed to unmarshal message")
		s.ErrCh <- err
		return
	}

	blue.Printf("from = %+v, to = %+v\n", msg.From, msg.To)
	blue.Printf("Party ID %v\n", s.party)

	isBroadcast := msg.IsBroadcast && len(msg.To) == 0
	isToSelf := len(msg.To) == 1 && ArePartyIDsEqual(msg.To[0], s.selfPartyID)

	fmt.Printf("isBroadcast = %+v, isToSelf =%+v\n", isBroadcast, isToSelf)

	if isBroadcast || isToSelf {
		go func() {
			if isBroadcast {
				fmt.Println("UPDATE BROADCAST MESSAGE", msg.To)
			} else if isToSelf {
				fmt.Println("UPDATE MESSAGE FOR ME", msg.To)

			}
			ok, err := s.party.UpdateFromBytes(msg.MsgBytes, msg.From, msg.IsBroadcast)
			if !ok || err != nil {
				fmt.Printf("err = %+v\n", err)
				log.Error().Err(err).Msg("failed to update party")
				s.ErrCh <- err
				return
			}

		}()
	}
}

// Close and clean up
func (s *Session) Close() {
	return

}
