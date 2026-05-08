package keygen

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p/core/crypto"
	"go.uber.org/zap"
)

const keyFileMode os.FileMode = 0600

func LoadOrGenerateKey(fileName string) (crypto.PrivKey, error) {
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
		if err != nil {
			return nil, fmt.Errorf("failed to generate key pair: %w", err)
		}

		raw, err := crypto.MarshalPrivateKey(priv)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal private key: %w", err)
		}

		if err := atomicWriteFile(fileName, raw, keyFileMode); err != nil {
			return nil, fmt.Errorf("failed to write private key: %w", err)
		}

		zap.L().Info("identity generated and saved", zap.String("file", fileName))
		return priv, nil
	}

	raw, err := os.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed reading the file %s: %w", fileName, err)
	}

	if len(raw) == 0 {
		return nil, fmt.Errorf("identity key file %s is empty (likely a crashed write); delete it to regenerate", fileName)
	}

	priv, err := crypto.UnmarshalPrivateKey(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal the private key: %w", err)
	}

	zap.L().Info("loaded existing identity", zap.String("file", fileName))
	return priv, nil
}

// atomicWriteFile writes data to path via a temp file in the same directory,
// fsyncs it, and renames into place. This guarantees that path is either the
// previous content or the new content — never a partial write.
func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if dir == "" {
		dir = "."
	}

	tmp, err := os.CreateTemp(dir, ".keygen-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	cleanup := func() {
		_ = os.Remove(tmpName)
	}

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("fsync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}
