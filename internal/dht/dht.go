package dht

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/evm-pmpc/evm-pmpc-node/pkg/config"

	kadDht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/util"
	"go.uber.org/zap"
)

func SetupDiscovery(ctx context.Context, h host.Host, cfg config.DiscoveryConfig, bootstrapAddrs []peer.AddrInfo) (*kadDht.IpfsDHT, error) {
	if len(bootstrapAddrs) > 0 {
		zap.L().Info("attempting to connect to bootstrap nodes...", zap.Int("count", len(bootstrapAddrs)))
		for {
			var wg sync.WaitGroup
			var connected int32

			for _, peerinfo := range bootstrapAddrs {
				wg.Add(1)
				go func(p peer.AddrInfo) {
					defer wg.Done()
					if err := h.Connect(ctx, p); err != nil {
						zap.L().Debug("failed to connect to bootstrap node", zap.String("peerID", p.ID.String()), zap.Error(err))
					} else {
						zap.L().Info("connected to bootstrap node", zap.String("peerID", p.ID.String()))
						atomic.AddInt32(&connected, 1)
					}
				}(peerinfo)
			}
			wg.Wait()

			if connected > 0 {
				zap.L().Info("successfully connected to bootstrap network", zap.Int32("connected_nodes", connected))
				break
			}

			zap.L().Warn("could not connect to any bootstrap nodes, retrying in 5 seconds...")

			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled while waiting for bootstrap node")
			}
		}
	}

	dht, err := kadDht.New(ctx, h, kadDht.ProtocolPrefix(protocol.ID(cfg.ProtocolPrefix)))
	if err != nil {
		return nil, fmt.Errorf("failed to create DHT: %w", err)
	}

	if err := dht.Bootstrap(ctx); err != nil {
		return nil, fmt.Errorf("failed to bootstrap DHT: %w", err)
	}

	routingDiscovery := routing.NewRoutingDiscovery(dht)
	util.Advertise(ctx, routingDiscovery, cfg.Rendezvous)
	zap.L().Info("advertised on rendezvous", zap.String("rendezvous", cfg.Rendezvous))

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			peerChan, err := routingDiscovery.FindPeers(ctx, cfg.Rendezvous)
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

	return dht, nil
}
