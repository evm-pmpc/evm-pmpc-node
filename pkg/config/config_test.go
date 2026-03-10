package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	content := `
identity:
  key_file: "test.key"
api:
  enabled: true
  port: 9090
  auth_token: "secret-token"
network:
  listen_port: 4002
  min_peers: 10
  max_peers: 50
  bootstrap_addrs:
    - "/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWTest"
  bootstrap_api: "http://localhost:8080"
discovery:
  rendezvous: "test-room"
  protocol_prefix: "/test/0.1.0"
pubsub:
  topic: "test-topic"
logging:
  level: "debug"
  format: "json"
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.Identity.KeyFile != "test.key" {
		t.Errorf("expected key_file 'test.key', got '%s'", cfg.Identity.KeyFile)
	}

	if !cfg.API.Enabled {
		t.Error("expected API to be enabled")
	}

	if cfg.API.Port != 9090 {
		t.Errorf("expected API port 9090, got %d", cfg.API.Port)
	}

	if cfg.API.AuthToken != "secret-token" {
		t.Errorf("expected auth_token 'secret-token', got '%s'", cfg.API.AuthToken)
	}

	if cfg.Network.ListenPort != 4002 {
		t.Errorf("expected listen_port 4002, got %d", cfg.Network.ListenPort)
	}

	if cfg.Network.MinPeers != 10 {
		t.Errorf("expected min_peers 10, got %d", cfg.Network.MinPeers)
	}

	if cfg.Network.MaxPeers != 50 {
		t.Errorf("expected max_peers 50, got %d", cfg.Network.MaxPeers)
	}

	if len(cfg.Network.BootstrapAddrs) != 1 {
		t.Fatalf("expected 1 bootstrap addr, got %d", len(cfg.Network.BootstrapAddrs))
	}

	if cfg.Network.BootstrapAPI != "http://localhost:8080" {
		t.Errorf("expected bootstrap_api 'http://localhost:8080', got '%s'", cfg.Network.BootstrapAPI)
	}

	if cfg.Discovery.Rendezvous != "test-room" {
		t.Errorf("expected rendezvous 'test-room', got '%s'", cfg.Discovery.Rendezvous)
	}

	if cfg.PubSub.Topic != "test-topic" {
		t.Errorf("expected pubsub topic 'test-topic', got '%s'", cfg.PubSub.Topic)
	}

	if cfg.Logging.Format != "json" {
		t.Errorf("expected logging format 'json', got '%s'", cfg.Logging.Format)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path.yaml")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoadEnvOverride(t *testing.T) {
	content := `
identity:
  key_file: "original.key"
network:
  listen_port: 4002
`
	tmpFile, err := os.CreateTemp("", "config-env-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	os.Setenv("PMPC_LOGGING_LEVEL", "debug")
	defer os.Unsetenv("PMPC_LOGGING_LEVEL")

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.Logging.Level != "debug" {
		t.Errorf("expected env override to 'debug', got '%s'", cfg.Logging.Level)
	}
}
