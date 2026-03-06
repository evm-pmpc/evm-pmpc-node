package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"

	"github.com/evm-pmpc/evm-pmpc-node/internal/keygen"
	"github.com/libp2p/go-libp2p"
	kadDHT "github.com/libp2p/go-libp2p-kad-dht"
)

const (
	ProtocolPrefix = "/evm-pmpc-node/0.1.0"
	KeyFile        = "bootstrap.key"
	Port           = "4001"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	priv, err := keygen.LoadOrGenerateKey(KeyFile)
	if err != nil {
		log.Fatalf("[bootstrap] - Failed to handle identity key: %v", err)
	}

	host, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", Port),
			fmt.Sprintf("/ip4/0.0.0.0/udp/%s/quic-v1", Port),
		),
		libp2p.EnableNATService(),
	)
	if err != nil {
		log.Fatalf("[bootstrap] - Failed to create libp2p host: %v", err)
	}

	kadDHT, err := kadDHT.New(ctx, host,
		kadDHT.Mode(kadDHT.ModeServer),
		kadDHT.ProtocolPrefix(ProtocolPrefix),
	)
	if err != nil {
		log.Fatalf("[bootstrap] - Failed to create DHT: %v", err)
	}

	if err = kadDHT.Bootstrap(ctx); err != nil {
		log.Fatalf("[bootstrap] - Failed to bootstrap DHT: %v", err)
	}

	fmt.Println("[bootstrap] - Bootstrap Node is Active")
	fmt.Printf("[bootstrap] - PeerID: %s\n", host.ID())
	fmt.Println("[bootstrap] - Add these to your worker nodes:")
	for _, addr := range host.Addrs() {
		fmt.Printf("  %s/p2p/%s\n", addr, host.ID())
	}

	<-ctx.Done()

	fmt.Println("[bootstrap] - Shutting down")
	host.Close()
}
