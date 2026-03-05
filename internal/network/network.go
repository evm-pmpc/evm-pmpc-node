package network

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	multiaddr "github.com/multiformats/go-multiaddr"
)

func SetupPing(h host.Host) *ping.PingService {
	ps := &ping.PingService{Host: h}
	h.SetStreamHandler(ping.ID, ps.PingHandler)
	return ps
}

func PingPeer(ctx context.Context, h host.Host, ps *ping.PingService, addr string, count int) error {
	ma, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return fmt.Errorf("[network] - invalid multiaddr: %w", err)
	}

	peerInfo, err := peer.AddrInfoFromP2pAddr(ma)
	if err != nil {
		return fmt.Errorf("[network] - invalid peer address: %w", err)
	}

	if err := h.Connect(ctx, *peerInfo); err != nil {
		return fmt.Errorf("[network] - failed to connect to peer: %w", err)
	}

	fmt.Println("[network] - sending", count, "ping messages to", ma)
	ch := ps.Ping(ctx, peerInfo.ID)
	for i := 0; i < count; i++ {
		res := <-ch
		if res.Error != nil {
			return fmt.Errorf("[network] - ping failed: %w", res.Error)
		}
		fmt.Println("[network] - pinged", ma, "in", res.RTT)
	}

	return nil
}
