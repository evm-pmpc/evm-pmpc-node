package network

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
)

func NewWorkerHost(ctx context.Context, priv crypto.PrivKey, listenPort int, minPeers int, maxPeers int, bootstrapAddrs []peer.AddrInfo) (host.Host, error) {
	cm, err := connmgr.NewConnManager(
		minPeers,
		maxPeers,
		connmgr.WithGracePeriod(time.Minute),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection manager: %w", err)
	}

	rm, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(rcmgr.DefaultLimits.AutoScale()))
	if err != nil {
		return nil, fmt.Errorf("failed to create resource manager: %w", err)
	}

	return libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort),
			fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1", listenPort),
		),
		libp2p.NATPortMap(),
		libp2p.EnableHolePunching(),
		libp2p.EnableAutoRelayWithStaticRelays(bootstrapAddrs),
		libp2p.EnableAutoNATv2(),
		libp2p.EnableNATService(),
		libp2p.ConnectionManager(cm),
		libp2p.ResourceManager(rm),
	)
}
