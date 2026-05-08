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

const (
	bootstrapRetryInterval = 5 * time.Second
	findPeersInterval      = 1 * time.Minute
	findPeersErrorBackoff  = 10 * time.Second
)

func SetupDiscovery(ctx context.Context, h host.Host, cfg config.DiscoveryConfig, bootstrapAddrs []peer.AddrInfo) (*kadDht.IpfsDHT, error) {
	if err := connectToBootstraps(ctx, h, bootstrapAddrs); err != nil {
		return nil, err
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

	go runFindPeersLoop(ctx, h, routingDiscovery, cfg.Rendezvous)

	return dht, nil
}

// connectToBootstraps blocks until at least one bootstrap peer is reachable
// or ctx is cancelled. It is a no-op when bootstrapAddrs is empty.
func connectToBootstraps(ctx context.Context, h host.Host, bootstrapAddrs []peer.AddrInfo) error {
	if len(bootstrapAddrs) == 0 {
		return nil
	}

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
					return
				}
				zap.L().Info("connected to bootstrap node", zap.String("peerID", p.ID.String()))
				atomic.AddInt32(&connected, 1)
			}(peerinfo)
		}
		wg.Wait()

		if connected > 0 {
			zap.L().Info("successfully connected to bootstrap network", zap.Int32("connected_nodes", connected))
			return nil
		}

		zap.L().Warn("could not connect to any bootstrap nodes, retrying", zap.Duration("after", bootstrapRetryInterval))
		if !sleepCtx(ctx, bootstrapRetryInterval) {
			return fmt.Errorf("context cancelled while waiting for bootstrap node")
		}
	}
}

// runFindPeersLoop periodically queries the rendezvous string in the DHT and
// dials any newly-discovered peer. Both the success interval and the
// post-error backoff are interruptible by ctx so shutdown is prompt.
func runFindPeersLoop(ctx context.Context, h host.Host, rd *routing.RoutingDiscovery, rendezvous string) {
	for {
		if ctx.Err() != nil {
			return
		}

		peerChan, err := rd.FindPeers(ctx, rendezvous)
		if err != nil {
			zap.L().Warn("failed to find peers", zap.Error(err))
			if !sleepCtx(ctx, findPeersErrorBackoff) {
				return
			}
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

		if !sleepCtx(ctx, findPeersInterval) {
			return
		}
	}
}

// sleepCtx waits for d or until ctx is cancelled, whichever comes first. It
// returns true if the full duration elapsed and false if ctx was cancelled.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}
