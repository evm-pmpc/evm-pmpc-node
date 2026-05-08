package keygen

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadOrGenerateKey_Generate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "id.key")

	priv, err := LoadOrGenerateKey(path)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if priv == nil {
		t.Fatal("nil priv key")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("written key file is empty")
	}
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != 0600 {
			t.Errorf("expected mode 0600, got %o", perm)
		}
	}

	// No leftover temp files in the same directory.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "id.key" {
			t.Errorf("unexpected leftover file in dir: %s", e.Name())
		}
	}
}

func TestLoadOrGenerateKey_Reload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "id.key")

	first, err := LoadOrGenerateKey(path)
	if err != nil {
		t.Fatalf("first: %v", err)
	}

	second, err := LoadOrGenerateKey(path)
	if err != nil {
		t.Fatalf("second: %v", err)
	}

	if !first.Equals(second) {
		t.Fatal("reloaded private key differs from generated one")
	}
}

func TestLoadOrGenerateKey_EmptyFileRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "id.key")

	if err := os.WriteFile(path, []byte{}, 0600); err != nil {
		t.Fatalf("seed empty file: %v", err)
	}

	if _, err := LoadOrGenerateKey(path); err == nil {
		t.Fatal("expected error on empty key file, got nil")
	}
}
