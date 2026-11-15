package dnssec

import (
	"crypto"
	"encoding/base64"
	"io"
	"strings"

	"github.com/cloudflare/circl/sign/ed448"
	"github.com/miekg/dns"
)

// verifySig checks one signature. miekg/dns covers every supported algorithm
// but Ed448, which it does not implement.
func verifySig(sig *dns.RRSIG, key *dns.DNSKEY, set []dns.RR) error {
	if sig.Algorithm != dns.ED448 {
		return sig.Verify(key, set)
	}
	return verifyEd448(sig, key, set)
}

// verifyEd448 runs miekg/dns's own checks and canonical form, then circl's
// Ed448. miekg's Verify fails only at the algorithm switch for Ed448, so its
// ErrAlg means every check before it passed.
func verifyEd448(sig *dns.RRSIG, key *dns.DNSKEY, set []dns.RR) error {
	if err := sig.Verify(key, set); err != dns.ErrAlg {
		if err == nil {
			return dns.ErrAlg // cannot happen: miekg has no Ed448
		}
		return err
	}
	pub, err := base64.StdEncoding.DecodeString(key.PublicKey)
	if err != nil || len(pub) != ed448.PublicKeySize {
		return dns.ErrKey
	}
	raw, err := base64.StdEncoding.DecodeString(sig.Signature)
	if err != nil || len(raw) != ed448.SignatureSize {
		return dns.ErrSig
	}
	data, err := signedData(sig, set)
	if err != nil {
		return err
	}
	if !ed448.Verify(pub, data, raw, "") {
		return dns.ErrSig
	}
	return nil
}

// signedData is the RFC 4034 §3.1.8.1 input to the signature, produced by
// miekg/dns's signer: it is signed as Ed25519 (unhashed, like Ed448) and the
// one algorithm byte is put back. TestSignedDataMatchesMiekg proves the bytes.
func signedData(sig *dns.RRSIG, set []dns.RR) ([]byte, error) {
	s := *sig
	s.Algorithm = dns.ED25519
	rrs := make([]dns.RR, len(set))
	for i, rr := range set {
		rrs[i] = dns.Copy(rr)
		h := rrs[i].Header()
		h.Ttl = sig.OrigTtl // Sign takes OrigTtl from here when it is 0
		// Sign derives Labels from the owner; give it the wildcard owner the
		// signature was made over.
		if labels := dns.SplitDomainName(h.Name); len(labels) > int(sig.Labels) {
			h.Name = "*." + strings.Join(labels[len(labels)-int(sig.Labels):], ".") + "."
		}
	}
	c := &capture{}
	if err := s.Sign(c, rrs); err != nil {
		return nil, err
	}
	c.data[2] = dns.ED448 // TypeCovered (2 bytes), then Algorithm
	return c.data, nil
}

// capture is a crypto.Signer that keeps what it is asked to sign.
type capture struct{ data []byte }

func (c *capture) Public() crypto.PublicKey { return nil }

func (c *capture) Sign(_ io.Reader, data []byte, _ crypto.SignerOpts) ([]byte, error) {
	c.data = append([]byte(nil), data...)
	return make([]byte, 64), nil
}
