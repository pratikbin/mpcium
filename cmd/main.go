package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/cryptoniumX/mpcium/pkg/config"
	"github.com/cryptoniumX/mpcium/pkg/eventconsumer"
	"github.com/cryptoniumX/mpcium/pkg/kvstore"
	"github.com/cryptoniumX/mpcium/pkg/logger"
	"github.com/cryptoniumX/mpcium/pkg/messaging"
	"github.com/cryptoniumX/mpcium/pkg/mpc"
	"github.com/hashicorp/consul/api"
	"github.com/nats-io/nats.go"
)

const (
	ENVIRONMENT = "ENVIRONMENT"
)

type AppConfig struct {
	ConsulAddr string `yaml:"consul.address"`
	NatsURL    string `yaml:"nats.url"`
}

func main() {
	config.InitViperConfig()
	logger.Init(os.Getenv(ENVIRONMENT))

	nodeName := flag.String("name", "", "Provide node name")
	flag.Parse()
	if *nodeName == "" {
		logger.Fatal("Node name is required", nil)
	}

	appConfig := config.LoadConfig()
	logger.Info("App config", "config", appConfig)

	consulClient := NewConsulClient(appConfig.Consul.Address)
	badgerKV := NewBadgerKV(*nodeName)
	defer badgerKV.Close()

	peers := LoadPeersFromConsul(consulClient)
	nodeID := GetIDFromName(*nodeName, peers)

	natsConn := NewNATsConnection(appConfig.NATs.URL)
	defer natsConn.Close()
	pubsub := messaging.NewNATSPubSub(natsConn)
	directMessaging := messaging.NewNatsDirectMessaging(natsConn)
	mqManager := messaging.NewNATsMessageQueueManager("mpc", []string{
		"mpc.mpc_keygen_success.*",
		"mpc.mpc_sign_success.*",
	}, natsConn)

	genKeySuccessQueue := mqManager.NewMessageQueue("mpc_keygen_success")
	defer genKeySuccessQueue.Close()
	singingSuccessQueue := mqManager.NewMessageQueue("mpc_sign_success")
	defer singingSuccessQueue.Close()

	logger.Info("Node is running", "peerID", nodeID, "name", *nodeName)

	peerNodeIDs := GetPeerIDs(peers)
	peerRegistry := mpc.NewRegistry(nodeID, peerNodeIDs, consulClient.KV())

	mpcNode := mpc.NewNode(
		nodeID,
		peerNodeIDs,
		pubsub,
		directMessaging,
		badgerKV,
		peerRegistry,
	)
	defer mpcNode.Close()

	eventConsumer := eventconsumer.NewEventConsumer(
		mpcNode,
		pubsub,
		genKeySuccessQueue,
		singingSuccessQueue,
	)
	eventConsumer.Run()
	defer eventConsumer.Close()
	// Create a channel to receive signals

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// Block the execution until a signal is received
	<-signals

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
		[]byte("1JwFmsc9lxlLfkPl"),
	)
	if err != nil {
		logger.Fatal("Failed to create badger kv store", err)
	}
	logger.Info("Connected to badger kv store", "path", dbPath)
	return badgerKv
}

func NewNATsConnection(natsURL string) *nats.Conn {
	natsConn, err := nats.Connect(natsURL, nats.Name("MPC NATs client"), nats.NoEcho())
	if err != nil {
		logger.Fatal("Failed to connect to nats", err)
	}
	return natsConn
}
