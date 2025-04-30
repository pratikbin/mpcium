<div class="title-block" style="text-align: center;" align="center">

# Mpcium - Threshold Signature Scheme nodes to generate secure crypto wallets

<p><img title="fystack logo" src="https://avatars.githubusercontent.com/u/149689344?s=400&u=13bed818667eefccd78ca4b4207d088eeb4f6110&v=4" width="320" height="320"></p>

[![Go Version](https://img.shields.io/badge/Go-v1.21+-00ADD8?logo=go&style=for-the-badge)](https://go.dev/)
[![License](https://img.shields.io/github/license/fystack/mpcium?style=for-the-badge)](./LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/fystack/mpcium?style=for-the-badge)](https://goreportcard.com/report/github.com/fystack/mpcium)
[![Version](https://img.shields.io/github/v/release/fystack/mpcium?label=version&logo=semantic-release&style=for-the-badge)](https://github.com/fystack/mpcium/releases)
[![Telegram](https://img.shields.io/badge/Telegram-Community%20-26A5E4?logo=telegram&style=for-the-badge)](https://t.me/+IsRhPyWuOFxmNmM9)
[![Made by Fystack](https://img.shields.io/badge/Made%20by-Fystack-7D3DF4?style=for-the-badge)](https://fystack.io)

This project employs multiple MPC nodes (Multi-Party Computation) to implement a secure and distributed threshold signature scheme. It aims to generate cryptographic signatures by distributing the private key among several nodes, requiring collaboration from a threshold number of nodes to produce a signature.

## Motivation

By distributing the private key among multiple nodes and requiring collaboration for signature generation, the project aims to enhance security, resilience, and trust in crypto currency wallet operations.

- **Enhanced Security**: With no single point of compromise.
- **Reduced Trust Requirements**: Trust is distributed among multiple nodes, reducing the reliance on a single trusted party.
- **Resilience to Node Failures**: Even if some nodes in the MPC network fail or become compromised, the threshold mechanism ensures that the system can still function as long as the specified threshold of nodes remains operational.
- **Privacy Preservation**: The collaborative nature of threshold signature generation helps preserve the privacy of the individual nodes, as no single node possesses the complete key

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                                    MPC Node                                         │
└─────────────────────────────────────────────────────────────────────────────────────┘
                   │                  │                  │                  │
                   ▼                  ▼                  ▼                  ▼
┌─────────────┐ ┌────────────┐ ┌─────────────┐ ┌────────────────┐ ┌─────────────────┐
│Configuration│ │ Peer       │ │ Key/Signing │ │ Event          │ │ Messaging       │
│             │ │ Management │ │ Session     │ │ Consumers      │ │ Infrastructure  │
└─────────────┘ └────────────┘ └─────────────┘ └────────────────┘ └─────────────────┘
      │               │              │                │                   │
      ▼               ▼              ▼                ▼                   ▼
┌─────────────┐ ┌────────────┐ ┌─────────────┐ ┌────────────────┐ ┌─────────────────┐
│Config Files │ │PeerRegistry│ │KeyGen       │ │Signing Consumer│ │NATS/JetStream   │
│& Env Vars   │ │            │ │Signing      │ │Event Consumer  │ │Direct Messaging │
└─────────────┘ └────────────┘ └─────────────┘ │Timeout Consumer│ │PubSub           │
                       │              │        └────────────────┘ └─────────────────┘
                       │              │                │                   │
                       ▼              ▼                ▼                   ▼
                 ┌────────────┐ ┌───────────────────────────────────────────────────┐
                 │ Consul KV  │ │                Storage Layer                      │
                 │            │ │                                                   │
                 └────────────┘ │   ┌───────────┐             ┌────────────────┐    │
                                │   │ BadgerDB  │             │ Consul KV      │    │
                                │   │(Local KV) │             │(Distributed KV)│    │
                                │   └───────────┘             └────────────────┘    │
                                └───────────────────────────────────────────────────┘
```

## Features

- Threshold Signature Generation: The project allows multiple MPC nodes to collectively generate a threshold signature.
- Threshold Signing: Each transaction require a threshold number of shares to be combined to generate a final signature

## Preview usage

### Start nodes

```shell
$ mpcium start -n node0
$ mpcium start -n node1
$ mpcium start -n node2

```

### Client

```go

import (
    "github.com/fystack/mpcium/client"
    "github.com/nats-io/nats.go"
)


func main () {
	natsConn, err := nats.Connect(natsURL)
	if err != nil {
		logger.Fatal("Failed to connect to NATS", err)
	}
	defer natsConn.Drain() // drain inflight msgs
	defer natsConn.Close()
	mpcClient := client.NewMPCClient(client.Options{
		NatsConn: natsConn,
		KeyPath:  "./event_initiator.key",
	})
	err = mpcClient.OnWalletCreationResult(func(event mpc.KeygenSuccessEvent) {
		logger.Info("Received wallet creation result", "event", event)
	})
	if err != nil {
		logger.Fatal("Failed to subscribe to wallet-creation results", err)
	}

	walletID := uuid.New().String()
	if err := mpcClient.CreateWallet(walletID); err != nil {
		logger.Fatal("CreateWallet failed", err)
	}
	logger.Info("CreateWallet sent, awaiting result...", "walletID", walletID)
}
```

### Diagaram

![Diagram](images/diagram.png)

## Encrypt

## Decrypt

#age --decrypt -o event_initiator.key event_initiator.key.age

## Generate strong password

< /dev/urandom tr -dc 'A-Za-z0-9!@#$^&\*()-\_=+[]{}|;:,.<>?/~' | head -c 16; echo
