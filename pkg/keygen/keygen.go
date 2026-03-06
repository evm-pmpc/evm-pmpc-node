package keygen

import (
	"fmt"
	"os"

	"github.com/libp2p/go-libp2p/core/crypto"
)

func LoadOrGenerateKey(fileName string) (crypto.PrivKey, error) {
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
		if err != nil {
			return nil, fmt.Errorf("[keygen] - failed to generate key pair: %w", err)
		}

		raw, err := crypto.MarshalPrivateKey(priv)
		if err != nil {
			return nil, fmt.Errorf("[keygen] - failed to marshal private key %w", err)
		}

		err = os.WriteFile(fileName, raw, 0600)
		if err != nil {
			return nil, fmt.Errorf("[keygen] - failed to write private key %w", err)
		}

		fmt.Printf("[keygen] - Identity generated and saved to %s\n", fileName)
		return priv, nil
	}

	raw, err := os.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("[keygen] - Failed reading the file %s: %w", fileName, err)
	}

	priv, err := crypto.UnmarshalPrivateKey(raw)
	if err != nil {
		return nil, fmt.Errorf("[keygen] - Failed to unmarshal the private key: %w", err)
	}

	fmt.Printf("[keygen] - Loaded existing identify from %s\n", fileName)
	return priv, nil
}
