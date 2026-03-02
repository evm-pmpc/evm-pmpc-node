package main

import (
	"context"
	"fmt"
	"sync"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
)

func InitDHT(ctx context.Context, h host.Host) (*dht.IpfsDHT, error) {
	kademliaDHT, err := dht.New(ctx, h, dht.Mode(dht.ModeAutoServer))
	if err != nil {
		return nil, fmt.Errorf("failed to create DHT: %w", err)
	}

	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		return nil, fmt.Errorf("failed to bootstrap DHT: %w", err)
	}

	var wg sync.WaitGroup
	for _, peerAddr := range dht.DefaultBootstrapPeers {
		pi, err := peer.AddrInfoFromP2pAddr(peerAddr)
		if err != nil {
			fmt.Println("Error parsing bootstrap peer:", err)
			continue
		}

		wg.Add(1)
		go func(pi peer.AddrInfo) {
			defer wg.Done()
			if err := h.Connect(ctx, pi); err != nil {
				fmt.Println("Warning: could not connect to bootstrap peer", pi.ID, ":", err)
			} else {
				fmt.Println("Connected to bootstrap peer:", pi.ID)
			}
		}(*pi)
	}
	wg.Wait()

	return kademliaDHT, nil
}

func DiscoverPeers(ctx context.Context, h host.Host, kademliaDHT *dht.IpfsDHT, rendezvous string) {
	routingDiscovery := drouting.NewRoutingDiscovery(kademliaDHT)

	dutil.Advertise(ctx, routingDiscovery, rendezvous)
	fmt.Println("Advertising under rendezvous:", rendezvous)

	fmt.Println("Searching for peers...")
	peerChan, err := routingDiscovery.FindPeers(ctx, rendezvous)
	if err != nil {
		fmt.Println("Error finding peers:", err)
		return
	}

	for p := range peerChan {
		if p.ID == h.ID() {
			continue
		}
		if h.Network().Connectedness(p.ID) == 1 {
			continue 
		}

		fmt.Println("Found peer:", p.ID, "-> connecting...")
		if err := h.Connect(ctx, p); err != nil {
			fmt.Println("Failed to connect to peer:", p.ID, err)
		} else {
			fmt.Println("Connected to peer:", p.ID)
		}
	}
}
