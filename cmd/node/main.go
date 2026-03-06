package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/evm-pmpc/evm-pmpc-node/internal/dht"
	"github.com/evm-pmpc/evm-pmpc-node/internal/discovery"
	"github.com/evm-pmpc/evm-pmpc-node/internal/network"

	"github.com/libp2p/go-libp2p/core/peer"
	multiaddr "github.com/multiformats/go-multiaddr"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("[main] - Usage: evm-pmpc-node <bootstrap-multiaddr>")
	}

	bootstrapMA, err := multiaddr.NewMultiaddr(os.Args[1])
	if err != nil {
		log.Fatalf("[main] - Invalid bootstrap address: %v", err)
	}

	bootstrapInfo, err := peer.AddrInfoFromP2pAddr(bootstrapMA)
	if err != nil {
		log.Fatalf("[main] - Invalid bootstrap peer address: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	node, err := network.NewWorkerHost(ctx, []peer.AddrInfo{*bootstrapInfo})
	if err != nil {
		log.Fatalf("[main] - Failed to create host: %v", err)
	}

	network.SetupPing(node)

	if err := discovery.InitMDNS(node); err != nil {
		log.Fatalf("[main] - Failed to start mDNS: %v", err)
	}

	if err := dht.SetupDiscovery(ctx, node, *bootstrapInfo); err != nil {
		log.Fatalf("[main] - Failed to setup DHT discovery: %v", err)
	}

	peerInfo := peer.AddrInfo{
		ID:    node.ID(),
		Addrs: node.Addrs(),
	}
	addrs, err := peer.AddrInfoToP2pAddrs(&peerInfo)
	if err != nil {
		log.Fatalf("[main] - Failed to get node addresses: %v", err)
	}
	fmt.Println("[main] - libp2p node address:", addrs[0])

	<-ctx.Done()

	fmt.Println("[main] - Received signal, shutting down")
	if err := node.Close(); err != nil {
		log.Fatalf("[main] - Failed to close node: %v", err)
	}
}
