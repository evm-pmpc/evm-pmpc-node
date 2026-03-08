package main

import (
	"context"
	"flag"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/evm-pmpc/evm-pmpc-node/internal/dht"
	"github.com/evm-pmpc/evm-pmpc-node/internal/discovery"
	"github.com/evm-pmpc/evm-pmpc-node/internal/network"
	"github.com/evm-pmpc/evm-pmpc-node/pkg/config"
	"github.com/evm-pmpc/evm-pmpc-node/pkg/keygen"
	"github.com/evm-pmpc/evm-pmpc-node/pkg/logger"

	"github.com/libp2p/go-libp2p/core/peer"
	multiaddr "github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to the config file")
	pingAddr := flag.String("ping", "", "optional peer address to ping after connecting")
	flag.Parse()

	logger.Init()
	defer zap.L().Sync()

	cfg, err := config.Load(*configPath)
	if err != nil {
		zap.L().Fatal("failed to load configuration", zap.Error(err), zap.String("path", *configPath))
	}

	if cfg.Logging.Format == "json" {
		logger.InitJSON()
	}

	if err := run(cfg, *pingAddr); err != nil {
		zap.L().Fatal("application failed", zap.Error(err))
	}
}

func run(cfg *config.Config, pingAddr string) error {
	var bootstrapAddrs []peer.AddrInfo
	for _, m := range cfg.Network.BootstrapAddrs {
		ma, err := multiaddr.NewMultiaddr(m)
		if err != nil {
			return fmt.Errorf("invalid bootstrap address in config (%s): %w", m, err)
		}
		info, err := peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			return fmt.Errorf("invalid bootstrap peer info (%s): %w", m, err)
		}
		bootstrapAddrs = append(bootstrapAddrs, *info)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	priv, err := keygen.LoadOrGenerateKey(cfg.Identity.KeyFile)
	if err != nil {
		return fmt.Errorf("failed to handle identity key: %w", err)
	}

	node, err := network.NewWorkerHost(ctx, priv, cfg.Network.ListenPort, cfg.Network.MinPeers, cfg.Network.MaxPeers, bootstrapAddrs)
	if err != nil {
		return fmt.Errorf("failed to create host: %w", err)
	}
	defer func() {
		if err := node.Close(); err != nil {
			zap.L().Error("failed to cleanly close host", zap.Error(err))
		}
	}()

	ps := network.SetupPing(node)

	if pingAddr != "" {
		go func() {
			if err := network.PingPeer(ctx, node, ps, pingAddr, 5); err != nil {
				zap.L().Error("failed to ping peer", zap.Error(err))
			}
		}()
	}

	if err := discovery.InitMDNS(node, cfg.Discovery.Rendezvous); err != nil {
		return fmt.Errorf("failed to start mDNS: %w", err)
	}

	dhtInstance, err := dht.SetupDiscovery(ctx, node, cfg.Discovery, bootstrapAddrs)
	if err != nil {
		return fmt.Errorf("failed to setup DHT discovery: %w", err)
	}
	defer func() {
		if err := dhtInstance.Close(); err != nil {
			zap.L().Error("failed to cleanly close DHT", zap.Error(err))
		}
	}()

	peerInfo := peer.AddrInfo{
		ID:    node.ID(),
		Addrs: node.Addrs(),
	}
	addrs, err := peer.AddrInfoToP2pAddrs(&peerInfo)
	if err != nil {
		return fmt.Errorf("failed to get node addresses: %w", err)
	}

	for _, addr := range addrs {
		zap.L().Info("libp2p node address", zap.String("addr", addr.String()))
	}

	<-ctx.Done()

	zap.L().Info("received graceful shutdown signal")
	return nil
}
