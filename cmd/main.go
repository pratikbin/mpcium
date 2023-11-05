package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"

	"github.com/cryptoniumX/mpcium/pkg/config"
	"github.com/cryptoniumX/mpcium/pkg/kvstore"
	"github.com/cryptoniumX/mpcium/pkg/logger"
	"github.com/cryptoniumX/mpcium/pkg/messaging"
	"github.com/cryptoniumX/mpcium/pkg/mpc"
	"github.com/hashicorp/consul/api"
	"github.com/nats-io/nats.go"
)

func main() {
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	logger.Init("dev")
	nodeName := flag.String("name", "", "Node name")
	flag.Parse()

	if *nodeName == "" {
		log.Fatal().Msg("Node name is required")
	}

	// Create a new Consul client
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		log.Error().Err(err)
	}

	badgerKv, err := kvstore.NewBadgerKVStore(
		fmt.Sprintf("./db/%s", *nodeName),
		[]byte("1JwFmsc9lxlLfkPl"),
	)

	defer badgerKv.Close()

	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create badger kv store")
	}

	// Create a Key-Value store client
	kv := client.KV()
	peers, err := config.LoadPeersFromConsul(kv, "mpc-peers/")
	if err != nil {
		logger.Error("Failed to load peers from Consul", err, "node", *nodeName)
		// log.Error().Stack().Err(err).Msg("Failed to load peers from Consul")
	}

	nodeID := config.GetNodeID(*nodeName, peers)
	if nodeID == "" {
		log.Error().Err(errors.New("Node not found"))
	}

	logger.Info("Node is running", "nodeID", nodeID)

	fmt.Printf("NODE ID is running = %+v\n", nodeID)
	natsConn, err := nats.Connect(nats.DefaultURL, nats.Name("Nats NoEcho"), nats.NoEcho())
	if err != nil {
		log.Error().Err(err)
	}

	natsPubSub := messaging.NewNATSPubSub(natsConn)
	directMessaging := messaging.NewNatsDirectMessaging(natsConn)

	defer natsConn.Close()

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
	fmt.Println("Subscribing to topic 'mpc:generate'")
	pubsub.Subscribe("mpc:generate", func(msg []byte) {
		fmt.Printf("msg = %+v\n", string(msg))

		walletID := string(msg)
		threshold := 3
		session, err := mpcNode.CreateKeyGenSession(walletID, threshold)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("session = %+v\n", session)

		err = session.Init()
		if err != nil {
			log.Error().Err(err)
		}
		go func() {
			for {
				select {
				case err := <-session.ErrCh:
					fmt.Printf("err = %+v\n", err)

				}
			}

		}()

		go session.GenerateKey()
		session.ListenToIncomingMessage()

	})
}
