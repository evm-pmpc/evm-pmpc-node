package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/evm-pmpc/evm-pmpc-node/internal/dht"
	"github.com/evm-pmpc/evm-pmpc-node/internal/discovery"
	"github.com/evm-pmpc/evm-pmpc-node/internal/network"
	"github.com/evm-pmpc/evm-pmpc-node/pkg/keygen"
	"github.com/evm-pmpc/evm-pmpc-node/pkg/logger"

	"github.com/libp2p/go-libp2p/core/peer"
	multiaddr "github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"
)

const KeyFile = "worker.key"

func main() {
	logger.Init()
	defer zap.L().Sync()

	if len(os.Args) < 2 {
		zap.L().Fatal("usage: evm-pmpc-node <bootstrap-multiaddr> [ping-address]")
	}

	bootstrapMA, err := multiaddr.NewMultiaddr(os.Args[1])
	if err != nil {
		zap.L().Fatal("invalid bootstrap address", zap.Error(err))
	}

	bootstrapInfo, err := peer.AddrInfoFromP2pAddr(bootstrapMA)
	if err != nil {
		zap.L().Fatal("invalid bootstrap peer address", zap.Error(err))
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	priv, err := keygen.LoadOrGenerateKey(KeyFile)
	if err != nil {
		zap.L().Fatal("failed to handle identity key", zap.Error(err))
	}

	node, err := network.NewWorkerHost(ctx, priv, []peer.AddrInfo{*bootstrapInfo})
	if err != nil {
		zap.L().Fatal("failed to create host", zap.Error(err))
	}

	ps := network.SetupPing(node)

	if len(os.Args) == 3 {
		pingAddr := os.Args[2]
		go func() {
			if err := network.PingPeer(ctx, node, ps, pingAddr, 5); err != nil {
				zap.L().Error("failed to ping peer", zap.Error(err))
			}
		}()
	}

	if err := discovery.InitMDNS(node); err != nil {
		zap.L().Fatal("failed to start mDNS", zap.Error(err))
	}

	if err := dht.SetupDiscovery(ctx, node, *bootstrapInfo); err != nil {
		zap.L().Fatal("failed to setup DHT discovery", zap.Error(err))
	}

	peerInfo := peer.AddrInfo{
		ID:    node.ID(),
		Addrs: node.Addrs(),
	}
	addrs, err := peer.AddrInfoToP2pAddrs(&peerInfo)
	if err != nil {
		zap.L().Fatal("failed to get node addresses", zap.Error(err))
	}

	for _, addr := range addrs {
		zap.L().Info("libp2p node address", zap.String("addr", addr.String()))
	}

	<-ctx.Done()

	zap.L().Info("received signal, shutting down")
	if err := node.Close(); err != nil {
		zap.L().Error("failed to close node", zap.Error(err))
	}
}
