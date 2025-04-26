package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/cryptoniumX/mpcium/pkg/config"
	"github.com/cryptoniumX/mpcium/pkg/constant"
	"github.com/cryptoniumX/mpcium/pkg/event"
	"github.com/cryptoniumX/mpcium/pkg/eventconsumer"
	"github.com/cryptoniumX/mpcium/pkg/identity"
	"github.com/cryptoniumX/mpcium/pkg/infra"
	"github.com/cryptoniumX/mpcium/pkg/keyinfo"
	"github.com/cryptoniumX/mpcium/pkg/kvstore"
	"github.com/cryptoniumX/mpcium/pkg/logger"
	"github.com/cryptoniumX/mpcium/pkg/messaging"
	"github.com/cryptoniumX/mpcium/pkg/mpc"
	"github.com/hashicorp/consul/api"
	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
)

const (
	ENVIRONMENT = "ENVIRONMENT"
)

func main() {
	environment := os.Getenv(ENVIRONMENT)
	config.InitViperConfig(environment)
	logger.Init(environment)

	nodeName := flag.String("name", "", "Provide node name")
	flag.Parse()
	if *nodeName == "" {
		logger.Fatal("Node name is required", nil)
	}

	appConfig := config.LoadConfig()
	logger.Info("App config", "config", appConfig.MarshalJSONMask())

	consulClient := infra.GetConsulClient(environment)
	badgerKV := NewBadgerKV(*nodeName)
	defer badgerKV.Close()

	keyinfoStore := keyinfo.NewStore(consulClient.KV())
	peers := LoadPeersFromConsul(consulClient)
	nodeID := GetIDFromName(*nodeName, peers)

	identityStore, err := identity.NewFileStore("identity", *nodeName)
	if err != nil {
		logger.Fatal("Failed to create identity store", err)
	}

	natsConn, err := GetNATSConnection(environment)
	if err != nil {
		logger.Fatal("Failed to connect to NATS", err)
	}
	defer natsConn.Close()

	pubsub := messaging.NewNATSPubSub(natsConn)
	signingStream, err := messaging.NewJetStreamPubSub(natsConn, event.SigningPublisherStream, []string{
		event.SigningRequestTopic,
	})
	if err != nil {
		logger.Fatal("Failed to create JetStream PubSub", err)
	}

	directMessaging := messaging.NewNatsDirectMessaging(natsConn)
	mqManager := messaging.NewNATsMessageQueueManager("mpc", []string{
		"mpc.mpc_keygen_success.*",
		event.SigningResultTopic,
	}, natsConn)

	genKeySuccessQueue := mqManager.NewMessageQueue("mpc_keygen_success")
	defer genKeySuccessQueue.Close()
	singingResultQueue := mqManager.NewMessageQueue("signing_result")
	defer singingResultQueue.Close()

	logger.Info("Node is running", "peerID", nodeID, "name", *nodeName)

	peerNodeIDs := GetPeerIDs(peers)
	peerRegistry := mpc.NewRegistry(nodeID, peerNodeIDs, consulClient.KV())

	mpcNode := mpc.NewNode(
		nodeID,
		peerNodeIDs,
		pubsub,
		directMessaging,
		badgerKV,
		keyinfoStore,
		peerRegistry,
		identityStore,
	)
	defer mpcNode.Close()

	eventConsumer := eventconsumer.NewEventConsumer(
		mpcNode,
		pubsub,
		genKeySuccessQueue,
		singingResultQueue,
	)
	eventConsumer.Run()
	defer eventConsumer.Close()

	timeoutConsumer := eventconsumer.NewTimeOutConsumer(
		natsConn,
		singingResultQueue,
	)

	timeoutConsumer.Run()
	defer timeoutConsumer.Close()
	signingConsumer := eventconsumer.NewSigningConsumer(natsConn, signingStream, pubsub)

	// Make the node ready before starting the signing consumer
	peerRegistry.Ready()

	ctx, cancel := context.WithCancel(context.Background())
	// Setup signal handling to cancel context on termination signals.
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		logger.Warn("Shutdown signal received, canceling context...")
		cancel()
	}()

	if err := signingConsumer.Run(ctx); err != nil {
		logger.Error("error running consumer:", err)
	}
}

func NewConsulClient(addr string) *api.Client {
	// Create a new Consul client
	consulConfig := api.DefaultConfig()
	consulConfig.Address = addr
	consulClient, err := api.NewClient(consulConfig)
	if err != nil {
		logger.Fatal("Failed to create consul client", err)
	}
	logger.Info("Connected to consul!")
	return consulClient
}

func LoadPeersFromConsul(consulClient *api.Client) []config.Peer { // Create a Consul Key-Value store client
	kv := consulClient.KV()
	peers, err := config.LoadPeersFromConsul(kv, "mpc-peers/")
	if err != nil {
		logger.Fatal("Failed to load peers from Consul", err)
	}
	logger.Info("Loaded peers from consul", "peers", peers)

	return peers
}

func GetPeerIDs(peers []config.Peer) []string {
	var peersIDs []string
	for _, peer := range peers {
		peersIDs = append(peersIDs, peer.ID)
	}
	return peersIDs
}

// Given node name, loop through peers and find the matching ID
func GetIDFromName(name string, peers []config.Peer) string {
	// Get nodeID from node name
	nodeID := config.GetNodeID(name, peers)
	if nodeID == "" {
		logger.Fatal("Empty Node ID", fmt.Errorf("Node ID not found for name %s", name))
	}

	return nodeID
}

func NewBadgerKV(nodeName string) *kvstore.BadgerKVStore {
	// Badger KV DB
	dbPath := fmt.Sprintf("./db/%s", nodeName)
	badgerKv, err := kvstore.NewBadgerKVStore(
		dbPath,
		[]byte(viper.GetString("badger_password")),
	)
	if err != nil {
		logger.Fatal("Failed to create badger kv store", err)
	}
	logger.Info("Connected to badger kv store", "path", dbPath)
	return badgerKv
}

func GetNATSConnection(environment string) (*nats.Conn, error) {
	if environment != constant.EnvProduction {
		return nats.Connect(viper.GetString("nats.url"))
	}
	clientCert := "./certs/client-cert.pem"
	clientKey := "./certs/client-key.pem"
	caCert := "./certs/rootCA.pem"

	return nats.Connect(viper.GetString("nats.url"),
		nats.ClientCert(clientCert, clientKey),
		nats.RootCAs(caCert),
		nats.UserInfo(viper.GetString("nats.username"), viper.GetString("nats.password")),
	)
}
