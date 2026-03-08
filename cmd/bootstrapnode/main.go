package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/evm-pmpc/evm-pmpc-node/pkg/keygen"
	"github.com/evm-pmpc/evm-pmpc-node/pkg/logger"
	"github.com/libp2p/go-libp2p"
	kadDHT "github.com/libp2p/go-libp2p-kad-dht"
	"go.uber.org/zap"
)

const (
	ProtocolPrefix = "/evm-pmpc-node/0.1.0"
	KeyFile        = "bootstrap.key"
	Port           = "4001"
)

func main() {
	logger.Init()
	defer zap.L().Sync()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	priv, err := keygen.LoadOrGenerateKey(KeyFile)
	if err != nil {
		zap.L().Fatal("failed to handle identity key", zap.Error(err))
	}

	host, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", Port),
			fmt.Sprintf("/ip4/0.0.0.0/udp/%s/quic-v1", Port),
		),
		libp2p.EnableNATService(),
		libp2p.EnableRelayService(),
	)
	if err != nil {
		zap.L().Fatal("failed to create libp2p host", zap.Error(err))
	}

	kadDHT, err := kadDHT.New(ctx, host,
		kadDHT.Mode(kadDHT.ModeServer),
		kadDHT.ProtocolPrefix(ProtocolPrefix),
	)
	if err != nil {
		zap.L().Fatal("failed to create DHT", zap.Error(err))
	}

	if err = kadDHT.Bootstrap(ctx); err != nil {
		zap.L().Fatal("failed to bootstrap DHT", zap.Error(err))
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

	zap.L().Info("shutting down")
	host.Close()
}
