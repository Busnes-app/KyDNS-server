package replica

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"

	"github.com/Busnes-app/ky-primitives/keyfile"
)

const keyFile = "node_key"

// Identity is this node's long-lived keypair. The node ID is the public key's
// fingerprint, so identity is the key itself rather than a name two operators
// could pick the same way.
type Identity struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
	NodeID     string
}

// Fingerprint is the peer identifier shown to operators and pinned at pairing.
func Fingerprint(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return hex.EncodeToString(sum[:])
}

// LoadOrCreateIdentity reads data_dir/node_key, generating it on first use. A
// key that exists but cannot be parsed, or that others can read, is an error:
// silently replacing it would change this node's identity and make every
// paired peer refuse it. Raw is the format node_key has always been written in.
func LoadOrCreateIdentity(dataDir string) (*Identity, error) {
	seed, err := keyfile.LoadOrCreateEncoded(filepath.Join(dataDir, keyFile), ed25519.SeedSize, keyfile.Raw)
	if err != nil {
		return nil, err
	}
	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)
	return &Identity{PrivateKey: priv, PublicKey: pub, NodeID: Fingerprint(pub)}, nil
}
