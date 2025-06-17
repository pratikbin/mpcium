package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/fystack/mpcium/pkg/client"
	"github.com/fystack/mpcium/pkg/config"
	"github.com/fystack/mpcium/pkg/event"
	"github.com/fystack/mpcium/pkg/logger"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
)

func main() {
	const environment = "development"

	// Parse CLI argument
	var n int
	flag.IntVar(&n, "n", 1, "Number of wallets to create")
	flag.Parse()

	config.InitViperConfig()
	logger.Init(environment, false)

	natsURL := viper.GetString("nats.url")
	natsConn, err := nats.Connect(natsURL)
	if err != nil {
		logger.Fatal("Failed to connect to NATS", err)
	}
	defer natsConn.Drain()
	defer natsConn.Close()

	mpcClient := client.NewMPCClient(client.Options{
		NatsConn: natsConn,
		KeyPath:  "./event_initiator.key",
	})

	err = mpcClient.OnWalletCreationResult(func(event event.KeygenSuccessEvent) {
		logger.Info("Received wallet creation result", "event", event)
	})
	if err != nil {
		logger.Fatal("Failed to subscribe to wallet-creation results", err)
	}

	for i := 0; i < n; i++ {
		walletID := uuid.New().String()
		if err := mpcClient.CreateWallet(walletID); err != nil {
			logger.Error("CreateWallet failed", err, "walletID", walletID)
		} else {
			logger.Info("CreateWallet sent", "walletID", walletID, "index", strconv.Itoa(i+1))
		}
	}

	logger.Info(fmt.Sprintf("Dispatched %d wallet creation requests", n))
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	fmt.Println("Shutting down.")
}
