package dht

import (
	"context"
	"fmt"
	"time"

	kadDht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/util"
	"go.uber.org/zap"
)

const (
	ProtocolPrefix = "/evm-pmpc-node/0.1.0"
	Rendezvous     = "evm-pmpc-rendezvous-room"
)

func SetupDiscovery(ctx context.Context, h host.Host, bootstrapAddr peer.AddrInfo) error {
	if err := h.Connect(ctx, bootstrapAddr); err != nil {
		return fmt.Errorf("failed to connect to bootstrap node: %w", err)
	}
	zap.L().Info("connected to bootstrap node", zap.String("peerID", bootstrapAddr.ID.String()))

	dht, err := kadDht.New(ctx, h, kadDht.ProtocolPrefix(ProtocolPrefix))
	if err != nil {
		return fmt.Errorf("failed to create DHT: %w", err)
	}

	if err := dht.Bootstrap(ctx); err != nil {
		return fmt.Errorf("failed to bootstrap DHT: %w", err)
	}

	routingDiscovery := routing.NewRoutingDiscovery(dht)
	util.Advertise(ctx, routingDiscovery, Rendezvous)
	zap.L().Info("advertised on rendezvous", zap.String("rendezvous", Rendezvous))

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			peerChan, err := routingDiscovery.FindPeers(ctx, Rendezvous)
			if err != nil {
				zap.L().Warn("failed to find peers", zap.Error(err))
				time.Sleep(10 * time.Second)
				continue
			}

			for p := range peerChan {
				if p.ID == h.ID() || len(p.Addrs) == 0 {
					continue
				}

				if err := h.Connect(ctx, p); err == nil {
					zap.L().Info("connected to peer", zap.String("peerID", p.ID.String()))
				}
			}
			time.Sleep(1 * time.Minute)
		}
	}()

	return nil
}
