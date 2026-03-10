package main

import (
	"context"
	"flag"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/evm-pmpc/evm-pmpc-node/api"
	"github.com/evm-pmpc/evm-pmpc-node/internal/dht"
	"github.com/evm-pmpc/evm-pmpc-node/internal/discovery"
	"github.com/evm-pmpc/evm-pmpc-node/internal/network"
	"github.com/evm-pmpc/evm-pmpc-node/internal/pubsub"
	"github.com/evm-pmpc/evm-pmpc-node/pkg/config"
	"github.com/evm-pmpc/evm-pmpc-node/pkg/keygen"
	"github.com/evm-pmpc/evm-pmpc-node/pkg/logger"

	"github.com/libp2p/go-libp2p/core/peer"
	multiaddr "github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to the config file")
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

	if err := run(cfg); err != nil {
		zap.L().Fatal("application failed", zap.Error(err))
	}
}

func run(cfg *config.Config) error {
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

	if cfg.Network.BootstrapAPI != "" {
		zap.L().Info("fetching dynamic bootstrap addresses from API", zap.String("url", cfg.Network.BootstrapAPI))
		apiAddrs, err := api.FetchBootstrapAddresses(cfg.Network.BootstrapAPI)
		if err != nil {
			zap.L().Warn("failed to fetch from bootstrap API, continuing with static addrs", zap.Error(err))
		} else {
			for _, m := range apiAddrs {
				ma, err := multiaddr.NewMultiaddr(m)
				if err != nil {
					continue
				}
				info, err := peer.AddrInfoFromP2pAddr(ma)
				if err != nil {
					continue
				}
				bootstrapAddrs = append(bootstrapAddrs, *info)
			}
			zap.L().Info("merged dynamic bootstrap addresses", zap.Int("total_addrs", len(bootstrapAddrs)))
		}
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

	zap.L().Info("node started", zap.String("peerID", node.ID().String()))
	for _, addr := range addrs {
		zap.L().Info("listening", zap.String("addr", addr.String()))
	}

	pubsubService, err := pubsub.NewPubSubService(ctx, node)
	if err != nil {
		return fmt.Errorf("failed to create pubsub service: %w", err)
	}
	defer pubsubService.Close()

	topicName := cfg.PubSub.Topic
	if topicName == "" {
		topicName = "evm-pmpc-general"
	}

	if _, err := pubsubService.JoinTopic(topicName); err != nil {
		return fmt.Errorf("failed to join pubsub topic: %w", err)
	}

	pubsubService.Subscribe(topicName, func(msg *pubsub.Message) {
		zap.L().Info("pubsub message received",
			zap.String("topic", topicName),
			zap.String("type", msg.Type),
			zap.String("from", msg.SenderID),
		)
	})

	<-ctx.Done()

	zap.L().Info("received graceful shutdown signal")
	return nil
}
