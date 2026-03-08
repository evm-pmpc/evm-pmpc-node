package discovery

import (
	"context"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"go.uber.org/zap"
)

const ServiceTag = "evm-pmpc-node:0.1.0"

type discoveryNotifee struct {
	h host.Host
}

func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	zap.L().Info("mDNS found new peer", zap.String("peerID", pi.ID.String()))

	err := n.h.Connect(context.Background(), pi)
	if err != nil {
		zap.L().Warn("mDNS failed to connect to peer",
			zap.String("peerID", pi.ID.String()),
			zap.Error(err),
		)
	} else {
		zap.L().Info("mDNS connected to peer", zap.String("peerID", pi.ID.String()))
	}
}

func InitMDNS(h host.Host) error {
	s := mdns.NewMdnsService(h, ServiceTag, &discoveryNotifee{h: h})

	return s.Start()
}
