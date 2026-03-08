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
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	multiaddr "github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"
)

func SetupPing(h host.Host) *ping.PingService {
	ps := &ping.PingService{Host: h}
	h.SetStreamHandler(ping.ID, ps.PingHandler)
	return ps
}

func PingPeer(ctx context.Context, h host.Host, ps *ping.PingService, addr string, count int) error {
	ma, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return fmt.Errorf("invalid multiaddr: %w", err)
	}

	peerInfo, err := peer.AddrInfoFromP2pAddr(ma)
	if err != nil {
		return fmt.Errorf("invalid peer address: %w", err)
	}

	if err := h.Connect(ctx, *peerInfo); err != nil {
		return fmt.Errorf("failed to connect to peer: %w", err)
	}

	zap.L().Info("sending ping messages", zap.Int("count", count), zap.String("target", ma.String()))
	ch := ps.Ping(ctx, peerInfo.ID)
	for i := 0; i < count; i++ {
		res := <-ch
		if res.Error != nil {
			return fmt.Errorf("ping failed: %w", res.Error)
		}
		zap.L().Info("ping success", zap.String("target", ma.String()), zap.Duration("rtt", res.RTT))
	}

	return nil
}

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
