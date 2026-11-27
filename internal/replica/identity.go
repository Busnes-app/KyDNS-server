package replica

import (
	"crypto"
	"crypto/ed25519"
	"crypto/mldsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Busnes-app/ky-primitives/keyfile"
)

const keyFile = "node_key"

// Identity is this node's long-lived keypair. The node ID is the public key's
// fingerprint, so identity is the key itself rather than a name two operators
// could pick the same way.
type Identity struct {
	PrivateKey crypto.Signer
	PublicKey  crypto.PublicKey
	NodeID     string
}

// Fingerprint is the peer identifier shown to operators and pinned at pairing.
// It supports ML-DSA-65 (default for new nodes) and legacy Ed25519 keys.
func Fingerprint(pub crypto.PublicKey) string {
	fp, err := publicKeyFingerprint(pub)
	if err != nil {
		return ""
	}
	return fp
}

func publicKeyFingerprint(pub crypto.PublicKey) (string, error) {
	switch k := pub.(type) {
	case *mldsa.PublicKey:
		if k.Parameters() != mldsa.MLDSA65() {
			return "", fmt.Errorf("unsupported ML-DSA parameter set %v, want ML-DSA-65", k.Parameters())
		}
		sum := sha256.Sum256(k.Bytes())
		return hex.EncodeToString(sum[:]), nil
	case ed25519.PublicKey:
		if len(k) != ed25519.PublicKeySize {
			return "", errors.New("invalid Ed25519 public key length")
		}
		sum := sha256.Sum256(k)
		return hex.EncodeToString(sum[:]), nil
	default:
		return "", fmt.Errorf("peer key is %T, want ML-DSA-65 or legacy ed25519", pub)
	}
}

// ValidateNodeKeyBytes checks that raw is either a valid PKCS #8 PEM ML-DSA-65
// private key or a legacy 32-byte raw Ed25519 seed.
func ValidateNodeKeyBytes(raw []byte) error {
	_, err := parseNodeKey(raw)
	return err
}

func parseNodeKey(raw []byte) (crypto.Signer, error) {
	// Legacy node_key files are 32 raw Ed25519 seed bytes and keep loading as
	// the exact same identity so already-paired clusters upgrade cleanly.
	if len(raw) == ed25519.SeedSize {
		return ed25519.NewKeyFromSeed(raw), nil
	}
	block, rest := pem.Decode(raw)
	if block == nil || block.Type != "PRIVATE KEY" || strings.TrimSpace(string(rest)) != "" {
		return nil, errors.New("node_key must contain an ML-DSA-65 PKCS #8 PEM key or a legacy 32-byte Ed25519 seed")
	}
	defer clear(block.Bytes)
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse node_key PKCS #8: %w", err)
	}
	key, ok := parsed.(*mldsa.PrivateKey)
	if !ok || key.PublicKey().Parameters() != mldsa.MLDSA65() {
		return nil, fmt.Errorf("node_key PKCS #8 key is %T, want ML-DSA-65", parsed)
	}
	return key, nil
}

// LoadOrCreateIdentity reads data_dir/node_key, generating an ML-DSA-65 key on
// first use. A key that exists but cannot be parsed, or that others can read,
// is an error: silently replacing it would change this node's identity and make
// every paired peer refuse it. Existing 32-byte raw Ed25519 node_key files keep
// loading with their original fingerprint.
func LoadOrCreateIdentity(dataDir string) (*Identity, error) {
	path := filepath.Join(dataDir, keyFile)
	if _, err := os.Stat(path); err == nil {
		return loadIdentityFile(path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	priv, err := mldsa.GenerateKey(mldsa.MLDSA65())
	if err != nil {
		return nil, fmt.Errorf("generate ML-DSA-65 node identity: %w", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("marshal ML-DSA-65 node identity: %w", err)
	}
	defer clear(der)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	defer clear(pemBytes)

	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return loadIdentityFile(path)
	}
	if err != nil {
		return nil, err
	}
	if _, err := f.Write(pemBytes); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return buildIdentity(priv)
}

func loadIdentityFile(path string) (*Identity, error) {
	if err := keyfile.RequireOwnerOnly(path); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	defer clear(raw)
	priv, err := parseNodeKey(raw)
	if err != nil {
		return nil, err
	}
	return buildIdentity(priv)
}

func buildIdentity(priv crypto.Signer) (*Identity, error) {
	pub := priv.Public()
	fp, err := publicKeyFingerprint(pub)
	if err != nil {
		return nil, err
	}
	return &Identity{PrivateKey: priv, PublicKey: pub, NodeID: fp}, nil
}
