package config

import (
	"fmt"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	Identity  IdentityConfig  `koanf:"identity"`
	API       APIConfig       `koanf:"api"`
	Network   NetworkConfig   `koanf:"network"`
	Discovery DiscoveryConfig `koanf:"discovery"`
	PubSub    PubSubConfig    `koanf:"pubsub"`
	Logging   LoggingConfig   `koanf:"logging"`
}

type APIConfig struct {
	Enabled   bool   `koanf:"enabled"`
	Port      int    `koanf:"port"`
	AuthToken string `koanf:"auth_token"`
}

type IdentityConfig struct {
	KeyFile string `koanf:"key_file"`
}

type NetworkConfig struct {
	ListenPort     int      `koanf:"listen_port"`
	MinPeers       int      `koanf:"min_peers"`
	MaxPeers       int      `koanf:"max_peers"`
	BootstrapAddrs []string `koanf:"bootstrap_addrs"`
	BootstrapAPI   string   `koanf:"bootstrap_api"`
}

type DiscoveryConfig struct {
	Rendezvous     string `koanf:"rendezvous"`
	ProtocolPrefix string `koanf:"protocol_prefix"`
}

type PubSubConfig struct {
	Topic string `koanf:"topic"`
}

type LoggingConfig struct {
	Level  string `koanf:"level"`
	Format string `koanf:"format"`
}

func Load(path string) (*Config, error) {
	k := koanf.New(".")

	if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
		return nil, fmt.Errorf("failed to load config file: %w", err)
	}

	k.Load(env.Provider("PMPC_", ".", func(s string) string {
		return strings.ReplaceAll(strings.ToLower(
			strings.TrimPrefix(s, "PMPC_")), "_", ".")
	}), nil)

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}
