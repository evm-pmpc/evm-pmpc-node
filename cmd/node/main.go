package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/evm-pmpc/evm-pmpc-node/internal/discovery"
	"github.com/evm-pmpc/evm-pmpc-node/internal/network"

	"github.com/libp2p/go-libp2p"
	peerstore "github.com/libp2p/go-libp2p/core/peer"
)

func main() {
	address := "/ip4/0.0.0.0/tcp/0"

	node, err := libp2p.New(
		libp2p.ListenAddrStrings(address),
		libp2p.Ping(false),
		libp2p.NATPortMap(),
		libp2p.EnableHolePunching(),
	)
	if err != nil {
		panic(err)
	}

	pingService := network.SetupPing(node)

	if err := discovery.InitMDNS(node); err != nil {
		panic(err)
	}

	peerInfo := peerstore.AddrInfo{
		ID:    node.ID(),
		Addrs: node.Addrs(),
	}
	addrs, err := peerstore.AddrInfoToP2pAddrs(&peerInfo)
	if err != nil {
		panic(err)
	}
	fmt.Println("[main] - libp2p node address:", addrs[0])

	if len(os.Args) > 1 {
		if err := network.PingPeer(context.Background(), node, pingService, os.Args[1], 5); err != nil {
			panic(err)
		}
	} else {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		fmt.Println("[main] - Received signal, shutting down")
	}

	if err := node.Close(); err != nil {
		panic(err)
	}
}
