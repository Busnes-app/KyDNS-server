// Package auth handles operator authentication: password hashing, login
// backoff, and sessions.
package auth

import (
	"errors"
	"net/netip"
	"sync"
	"time"

	"github.com/Busnes-app/ky-primitives/password"
)

// MinPasswordLen is the shortest admin password accepted, wherever one is
// set: the web setup form and the reset-password command both use it.
const MinPasswordLen = 12

// HashPassword returns a PHC-format argon2id string that embeds its own
// parameters, so raising the cost later does not invalidate old hashes.
func HashPassword(plaintext string) (string, error) {
	return password.Hash(plaintext)
}

// ErrBusy means too many password checks are running. It is "try again", not a
// wrong password, so it must never count as a failed login.
var ErrBusy = password.ErrBusy

// VerifyPassword compares in constant time. A malformed hash is a false, never
// a panic: a corrupted row must not crash the login page. The only error is
// ErrBusy.
func VerifyPassword(encoded, plaintext string) (bool, error) {
	ok, err := password.Verify(plaintext, encoded)
	if errors.Is(err, password.ErrBusy) {
		return false, ErrBusy
	}
	return err == nil && ok, nil
}

const (
	backoffBase = 100 * time.Millisecond
	backoffCap  = 30 * time.Second
	// backoffMaxShift keeps the doubling from overflowing before the cap
	// clamps it. 2^18 * 100ms already far exceeds backoffCap.
	backoffMaxShift = 18
	// backoffMaxKeys bounds memory against a caller rotating addresses.
	backoffMaxKeys = 4096
)

// Backoff slows repeated failed logins per source. It deliberately never
// locks an account: a permanent lockout hands an attacker a denial of service
// against the operator.
type Backoff struct {
	mu       sync.Mutex
	failures map[string]int
}

func NewBackoff() *Backoff { return &Backoff{failures: map[string]int{}} }

// Delay is how long to wait before answering the next attempt from key.
func (b *Backoff) Delay(key string) time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := b.failures[bucket(key)]
	if n == 0 {
		return 0
	}
	if n > backoffMaxShift {
		return backoffCap
	}
	d := backoffBase << (n - 1)
	if d > backoffCap {
		return backoffCap
	}
	return d
}

func (b *Backoff) Fail(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	key = bucket(key)
	if _, ok := b.failures[key]; !ok && len(b.failures) >= backoffMaxKeys {
		for k := range b.failures { // evict one arbitrary entry
			delete(b.failures, k)
			break
		}
	}
	b.failures[key]++
}

func (b *Backoff) Reset(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.failures, bucket(key))
}

// bucket groups an IPv6 source by its /64, the usual allocation to one host
// or LAN; anything else is its own bucket.
func bucket(key string) string {
	ip, err := netip.ParseAddr(key)
	if err != nil || !ip.Is6() || ip.Is4In6() {
		return key
	}
	p, _ := ip.Prefix(64)
	return p.String()
}
