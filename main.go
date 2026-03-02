package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/libp2p/go-libp2p"
	peerstore "github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	multiaddr "github.com/multiformats/go-multiaddr"
)

const Rendezvous = "evm-pmpc/evmp-pmpc-node/0.1.0"

type discoveryNotifee struct {
	h interface {
		Connect(context.Context, peerstore.AddrInfo) error
	}
	ctx context.Context
}

func (n *discoveryNotifee) HandlePeerFound(pi peerstore.AddrInfo) {
	fmt.Println("mDNS: discovered local peer", pi.ID)
	if err := n.h.Connect(n.ctx, pi); err != nil {
		fmt.Println("mDNS: failed to connect to", pi.ID, err)
	}
}

func GetLocalIPv4() string {
	localIP := ""

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		fmt.Println("Error: ", err)
		return ""
	}

	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)

		if ok && !ipNet.IP.IsLoopback() {
			if ipv4 := ipNet.IP.To4(); ipv4 != nil {
				localIP = ipv4.String()
			}
		}
	}

	return localIP
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	address := "/ip4/" + GetLocalIPv4() + "/tcp/0"

	node, err := libp2p.New(
		libp2p.ListenAddrStrings(
			address,
			"/ip4/0.0.0.0/tcp/0",
			"/ip4/0.0.0.0/udp/0/quic-v1",
		),
		libp2p.Ping(false),
		libp2p.NATPortMap(),
		libp2p.EnableRelay(),
	)
	if err != nil {
		panic(err)
	}

	pingService := &ping.PingService{Host: node}
	node.SetStreamHandler(ping.ID, pingService.PingHandler)

	peerInfo := peerstore.AddrInfo{
		ID:    node.ID(),
		Addrs: node.Addrs(),
	}
	addrs, err := peerstore.AddrInfoToP2pAddrs(&peerInfo)
	if err != nil {
		panic(err)
	}
	fmt.Println("libp2p node address:", addrs[0])

	kademliaDHT, err := InitDHT(ctx, node)
	if err != nil {
		panic(err)
	}

	go DiscoverPeers(ctx, node, kademliaDHT, Rendezvous)

	mdnsService := mdns.NewMdnsService(node, Rendezvous, &discoveryNotifee{h: node, ctx: ctx})
	if err := mdnsService.Start(); err != nil {
		fmt.Println("mDNS start error:", err)
	}

	if len(os.Args) > 1 {
		addr, err := multiaddr.NewMultiaddr(os.Args[1])
		if err != nil {
			panic(err)
		}
		peer, err := peerstore.AddrInfoFromP2pAddr(addr)
		if err != nil {
			panic(err)
		}
		if err := node.Connect(ctx, *peer); err != nil {
			panic(err)
		}
		fmt.Println("sending 5 ping messages to", addr)
		ch := pingService.Ping(ctx, peer.ID)
		for range 5 {
			res := <-ch
			fmt.Println("pinged", addr, "in", res.RTT)
		}
	} else {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		fmt.Println("Received signal, shutting down...")
	}

	if err := node.Close(); err != nil {
		panic(err)
	}
}
