package main

import (
	"context"
	"flag"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/evm-pmpc/evm-pmpc-node/api"
	"github.com/evm-pmpc/evm-pmpc-node/pkg/config"
	"github.com/evm-pmpc/evm-pmpc-node/pkg/keygen"
	"github.com/evm-pmpc/evm-pmpc-node/pkg/logger"
	"github.com/libp2p/go-libp2p"
	kadDHT "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/protocol"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"go.uber.org/zap"
)

func main() {
	configPath := flag.String("config", "config-bootstrap.yaml", "path to the config file")
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
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	priv, err := keygen.LoadOrGenerateKey(cfg.Identity.KeyFile)
	if err != nil {
		return fmt.Errorf("failed to handle identity key: %w", err)
	}

	cm, err := connmgr.NewConnManager(
		cfg.Network.MinPeers,
		cfg.Network.MaxPeers,
		connmgr.WithGracePeriod(time.Minute),
	)
	if err != nil {
		return fmt.Errorf("failed to create connection manager: %w", err)
	}

	rm, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(rcmgr.DefaultLimits.AutoScale()))
	if err != nil {
		return fmt.Errorf("failed to create resource manager: %w", err)
	}

	host, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", cfg.Network.ListenPort),
			fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1", cfg.Network.ListenPort),
		),
		libp2p.NATPortMap(),
		libp2p.ForceReachabilityPublic(),
		libp2p.EnableNATService(),
		libp2p.EnableRelayService(),
		libp2p.ConnectionManager(cm),
		libp2p.ResourceManager(rm),
	)
	if err != nil {
		return fmt.Errorf("failed to create libp2p host: %w", err)
	}
	defer func() {
		if err := host.Close(); err != nil {
			zap.L().Error("failed to cleanly close host", zap.Error(err))
		}
	}()

	if cfg.API.Enabled {
		apiServer := api.NewServer(host, cfg.API.Port, cfg.API.AuthToken)
		if err := apiServer.Start(); err != nil {
			return fmt.Errorf("failed to start api server: %w", err)
		}
	}

	dhtInstance, err := kadDHT.New(ctx, host,
		kadDHT.Mode(kadDHT.ModeServer),
		kadDHT.ProtocolPrefix(protocol.ID(cfg.Discovery.ProtocolPrefix)),
	)
	if err != nil {
		return fmt.Errorf("failed to create DHT: %w", err)
	}
	defer func() {
		if err := dhtInstance.Close(); err != nil {
			zap.L().Error("failed to cleanly close DHT", zap.Error(err))
		}
	}()

	if err = dhtInstance.Bootstrap(ctx); err != nil {
		return fmt.Errorf("failed to bootstrap DHT: %w", err)
	}

	zap.L().Info("bootstrap node is active",
		zap.String("peerID", host.ID().String()),
	)
	for _, addr := range host.Addrs() {
		zap.L().Info("listening address",
			zap.String("multiaddr", fmt.Sprintf("%s/p2p/%s", addr, host.ID())),
		)
	}

	<-ctx.Done()

	zap.L().Info("received graceful shutdown signal")
	return nil
}
