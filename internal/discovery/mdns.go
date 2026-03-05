package discovery

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

const ServiceTag = "evm-pmpc-node:0.1.0"

type discoveryNotifee struct {
	h host.Host
}

func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	fmt.Printf("[mDNS] - Found new peer: %s\n", pi.ID.String())

	err := n.h.Connect(context.Background(), pi)
	if err != nil {
		fmt.Printf("[mDNS] - Error connecting to peer %s: %s\n", pi.ID.String(), err)
	} else {
		fmt.Printf("[mDNS] - Successfully connected to peer: %s\n", pi.ID.String())
	}
}

func InitMDNS(h host.Host) error {
	s := mdns.NewMdnsService(h, ServiceTag, &discoveryNotifee{h: h})

	return s.Start()
}
