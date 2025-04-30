package client

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fystack/mpcium/pkg/event"
	"github.com/fystack/mpcium/pkg/eventconsumer"
	"github.com/fystack/mpcium/pkg/logger"
	"github.com/fystack/mpcium/pkg/messaging"
	"github.com/fystack/mpcium/pkg/mpc"
	"github.com/fystack/mpcium/pkg/types"
	"github.com/nats-io/nats.go"
)

const (
	GenerateWalletSuccessTopic = "mpc.mpc_keygen_success.*" // wildcard to listen to all success events
)

type MPCClient interface {
	CreateWallet(walletID string) error
	OnWalletCreationResult(callback func(event mpc.KeygenSuccessEvent)) error

	SignTransaction(msg *types.SignTxMessage) error
	OnSignResult(callback func(event event.SigningResultEvent)) error
}

type mpcClient struct {
	signingStream      messaging.StreamPubsub
	pubsub             messaging.PubSub
	genKeySuccessQueue messaging.MessageQueue
	signResultQueue    messaging.MessageQueue
	privKey            ed25519.PrivateKey
}

// NewMPCClient reads the Ed25519 private key from disk and
// sets up a JetStream subscriber/publisher on "mpc.signing_request.*".
func NewMPCClient(nc *nats.Conn) MPCClient {
	// 1) Load seed or full private key bytes
	privHexBytes, err := os.ReadFile(filepath.Join(".", "event_initiator.key"))
	if err != nil {
		logger.Fatal("Failed to read private key file", err)
	}

	privHex := string(privHexBytes)
	// Decode private key from hex
	privSeed, err := hex.DecodeString(privHex)
	if err != nil {
		fmt.Println("Failed to decode private key hex:", err)
		os.Exit(1)
	}

	// Reconstruct full Ed25519 private key from seed
	priv := ed25519.NewKeyFromSeed(privSeed)

	// 2) Create the PubSub for both publish & subscribe
	signingStream, err := messaging.NewJetStreamPubSub(nc, "mpc-signing", []string{
		"mpc.signing_request.*",
	})
	if err != nil {
		logger.Fatal("Failed to create JetStream PubSub", err)
	}

	pubsub := messaging.NewNATSPubSub(nc)

	manager := messaging.NewNATsMessageQueueManager("mpc", []string{
		"mpc.mpc_keygen_success.*",
		"mpc.signing_result.*",
	}, nc)

	genKeySuccessQueue := manager.NewMessageQueue("mpc_keygen_success")
	signResultQueue := manager.NewMessageQueue("signing_result")

	return &mpcClient{
		signingStream:      signingStream,
		pubsub:             pubsub,
		genKeySuccessQueue: genKeySuccessQueue,
		signResultQueue:    signResultQueue,
		privKey:            priv,
	}
}

// CreateWallet generates a GenerateKeyMessage, signs it, and publishes it.
func (c *mpcClient) CreateWallet(walletID string) error {
	// build the message
	msg := &types.GenerateKeyMessage{
		WalletID: walletID,
	}
	// compute the canonical raw bytes
	raw, err := msg.Raw()
	if err != nil {
		return fmt.Errorf("CreateWallet: raw payload error: %w", err)
	}
	// sign
	msg.Signature = ed25519.Sign(c.privKey, raw)

	bytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("CreateWallet: marshal error: %w", err)
	}

	if err := c.pubsub.Publish(eventconsumer.MPCGenerateEvent, bytes); err != nil {
		return fmt.Errorf("CreateWallet: publish error: %w", err)
	}
	return nil
}

// The callback will be invoked whenever a wallet creation result is received.
func (c *mpcClient) OnWalletCreationResult(callback func(event mpc.KeygenSuccessEvent)) error {
	err := c.genKeySuccessQueue.Dequeue(GenerateWalletSuccessTopic, func(msg []byte) error {
		var event mpc.KeygenSuccessEvent
		err := json.Unmarshal(msg, &event)
		if err != nil {
			return err
		}
		callback(event)
		return nil
	})

	if err != nil {
		return fmt.Errorf("OnWalletCreationResult: subscribe error: %w", err)
	}

	return nil
}

// SignTransaction builds a SignTxMessage, signs it, and publishes it.
func (c *mpcClient) SignTransaction(msg *types.SignTxMessage) error {
	// compute the canonical raw bytes (omitting Signature field)
	raw, err := msg.Raw()
	if err != nil {
		return fmt.Errorf("SignTransaction: raw payload error: %w", err)
	}
	// sign
	msg.Signature = ed25519.Sign(c.privKey, raw)

	bytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("SignTransaction: marshal error: %w", err)
	}

	if err := c.signingStream.Publish(event.SigningRequestEventTopic, bytes); err != nil {
		return fmt.Errorf("SignTransaction: publish error: %w", err)
	}
	return nil
}

func (c *mpcClient) OnSignResult(callback func(event event.SigningResultEvent)) error {
	err := c.signResultQueue.Dequeue(event.SigningResultCompleteTopic, func(msg []byte) error {
		var event event.SigningResultEvent
		err := json.Unmarshal(msg, &event)
		if err != nil {
			return err
		}
		callback(event)
		return nil
	})

	if err != nil {
		return fmt.Errorf("OnSignResult: subscribe error: %w", err)
	}

	return nil
}
