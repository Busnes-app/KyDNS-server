package replica

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadOrCreateIdentityIsStable(t *testing.T) {
	dir := t.TempDir()
	first, err := LoadOrCreateIdentity(dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreateIdentity(dir)
	if err != nil {
		t.Fatal(err)
	}
	if first.NodeID != second.NodeID {
		t.Fatalf("NodeID changed across loads: %q then %q", first.NodeID, second.NodeID)
	}
	if first.NodeID == "" {
		t.Fatal("NodeID is empty")
	}
}

func TestDistinctDirsGetDistinctIdentities(t *testing.T) {
	a, err := LoadOrCreateIdentity(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b, err := LoadOrCreateIdentity(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if a.NodeID == b.NodeID {
		t.Fatal("two nodes share a NodeID")
	}
}

// The private key is the node's whole identity. A world-readable key means any
// local account can impersonate this node to its peers.
func TestPrivateKeyIsNotReadableByOthers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX modes only")
	}
	dir := t.TempDir()
	if _, err := LoadOrCreateIdentity(dir); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(dir, "node_key"))
	if err != nil {
		t.Fatal(err)
	}
	if mode := fi.Mode().Perm(); mode != 0o600 {
		t.Fatalf("node_key mode = %#o, want 0600", mode)
	}
}

// A key relaxed to 0644 by a chmod or a careless restore is no longer secret,
// so it is refused rather than used.
func TestPermissiveKeyIsRefused(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX modes only")
	}
	dir := t.TempDir()
	if _, err := LoadOrCreateIdentity(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "node_key"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateIdentity(dir); err == nil {
		t.Fatal("a world-readable node_key was loaded")
	}
}

// node_key files already on disk are 32 raw seed bytes and must keep loading
// as the same identity, or every paired peer would refuse this node.
func TestExistingRawKeyKeepsItsIdentity(t *testing.T) {
	dir := t.TempDir()
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i)
	}
	if err := os.WriteFile(filepath.Join(dir, "node_key"), seed, 0o600); err != nil {
		t.Fatal(err)
	}
	id, err := LoadOrCreateIdentity(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := Fingerprint(ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)); id.NodeID != want {
		t.Fatalf("NodeID = %s, want %s", id.NodeID, want)
	}
}

func TestFingerprintIsDeterministic(t *testing.T) {
	id, err := LoadOrCreateIdentity(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got := Fingerprint(id.PublicKey); got != id.NodeID {
		t.Fatalf("Fingerprint() = %q, NodeID = %q; they must agree", got, id.NodeID)
	}
}

func TestCorruptKeyIsAnErrorNotANewIdentity(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadOrCreateIdentity(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "node_key"), []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateIdentity(dir); err == nil {
		t.Fatal("a corrupt key silently minted a new identity; every peer would " +
			"refuse this node and the operator would have no idea why")
	}
}
