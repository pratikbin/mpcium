package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cryptoniumX/mpcium/pkg/infra"
	"github.com/nats-io/nats.go"
)

func main() {
	natsConn, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatal(err)
	}

	natsPubSub := infra.NewNATSPubSub(natsConn)
	handler(natsPubSub)

	defer natsConn.Close()

	go func() {
		select {}

	}()

	// Create a channel to receive signals
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// Block the execution until a signal is received
	<-signals

}

func handler(pubsub infra.PubSub) {
	fmt.Println("Subscribing to topic 'mpc:generate'")
	// ...
	pubsub.Subscribe("mpc:generate", func(msg []byte) {
		fmt.Printf("msg = %+v\n", string(msg))
	})
}
