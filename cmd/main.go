package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"os"
	"os/signal"
	"syscall"

	"github.com/cryptoniumX/mpcium/pkg/config"
	"github.com/cryptoniumX/mpcium/pkg/kvstore"
	"github.com/cryptoniumX/mpcium/pkg/logger"
	"github.com/cryptoniumX/mpcium/pkg/messaging"
	"github.com/cryptoniumX/mpcium/pkg/mpc"
	"github.com/hashicorp/consul/api"
	"github.com/nats-io/nats.go"
)

func main() {
	logger.Init("dev")
	nodeName := flag.String("name", "", "Provide node name")
	flag.Parse()

	if *nodeName == "" {
		logger.Fatal("Node name is required", nil)
	}

	// Create a new Consul client
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		logger.Fatal("Failed to create consul client", err)
	}
	logger.Info("Connected to consul!", "configuration", api.DefaultConfig())

	dbPath := fmt.Sprintf("./db/%s", *nodeName)
	badgerKv, err := kvstore.NewBadgerKVStore(
		dbPath,
		[]byte("1JwFmsc9lxlLfkPl"),
	)

	defer badgerKv.Close()
	if err != nil {
		logger.Fatal("Failed to create badger kv store", err)
	}
	logger.Info("Connected to badger kv store", "path", dbPath)

	// Create a Key-Value store client
	kv := client.KV()
	peers, err := config.LoadPeersFromConsul(kv, "mpc-peers/")
	if err != nil {
		logger.Fatal("Failed to load peers from Consul", err)
	}
	logger.Info("Loaded peers from consul", "peers", peers)
	nodeID := config.GetNodeID(*nodeName, peers)
	if nodeID == "" {
		logger.Error("Node ID not found", nil, "node", *nodeName)
		return
	}

	natsConn, err := nats.Connect(nats.DefaultURL, nats.Name("Nats NoEcho"), nats.NoEcho())
	if err != nil {
		logger.Fatal("Failed to connect to nats", err)
	}

	natsPubSub := messaging.NewNATSPubSub(natsConn)
	directMessaging := messaging.NewNatsDirectMessaging(natsConn)
	defer natsConn.Close()

	logger.Info("Node is running", "nodeID", nodeID, "name", *nodeName)

	var peersIDs []string
	for _, peer := range peers {
		peersIDs = append(peersIDs, peer.ID)
	}

	mpcNode := mpc.NewNode(
		nodeID,
		peersIDs,
		natsPubSub,
		directMessaging,
		badgerKv,
	)
	mpcNode.WaitPeersReady()

	handler(natsPubSub, mpcNode)

	go func() {
		select {}

	}()

	// Create a channel to receive signals

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// Block the execution until a signal is received
	<-signals

}

func handler(pubsub messaging.PubSub, mpcNode *mpc.Node) {
	logger.Info("Starting handler, subscribe to topic", "topic", "mpc:generate")
	go pubsub.Subscribe("mpc:generate", func(msg []byte) {
		walletID := string(msg)
		// TODO: threshold is configurable
		threshold := 2
		session, err := mpcNode.CreateKeyGenSession(walletID, threshold)
		if err != nil {
			fmt.Println(err)
			return
		}

		session.Init()
		go func() {
			for {
				select {
				case err := <-session.ErrCh:
					logger.Error("Keygen session error", err)
				}
			}

		}()

		go session.GenerateKey()
		// TODO -> done and close channel
		session.ListenToIncomingMessage()

	})

	logger.Info("Subscribed to topic", "topic", "mpc:sign")
	go pubsub.Subscribe("mpc:sign", func(raw []byte) {

		var msg SignTxMessage
		err := json.Unmarshal(raw, &msg)
		if err != nil {
			logger.Error("Failed to unmarshal message", err)
			return
		}

		logger.Info("Received signing event", "waleltID", msg.WalletID, "tx", msg.Tx)
		threshold := 2
		session, err := mpcNode.CreateSigningSession(msg.WalletID, msg.TxID, msg.NetworkInternalCode, threshold)
		if err != nil {
			logger.Error("Failed to create signing session", err)
			return
		}

		txBigInt := new(big.Int).SetBytes(msg.Tx)
		session.Init(txBigInt)

		go func() {
			for {
				select {
				case err := <-session.ErrCh:
					logger.Error("Signing session error", err)
				}
			}

		}()

		go session.Sign()
		// TODO -> done and close channel
		session.ListenToIncomingMessage()

	})
}

type SignTxMessage struct {
	WalletID            string `json:"wallet_id"`
	NetworkInternalCode string `json:"network_internal_code"`
	TxID                string `json:"tx_id"`
	Tx                  []byte `json:"tx"`
}
