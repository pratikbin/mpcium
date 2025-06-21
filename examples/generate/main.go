package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	// Parse the -n flag
	numWallets := flag.Int("n", 1, "Number of wallets to generate")
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

	for i := 0; i < *numWallets; i++ {
		walletID := uuid.New().String()
		if err := mpcClient.CreateWallet(context.Background(), walletID); err != nil {
			logger.Error("CreateWallet failed", err)
			continue
		}
		time.Sleep(100 * time.Millisecond)
		logger.Info("CreateWallet sent", "walletID", walletID)
	}

	logger.Info("All CreateWallet requests sent, awaiting results...")

	// Wait for shutdown signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	fmt.Println("Shutting down.")
}
