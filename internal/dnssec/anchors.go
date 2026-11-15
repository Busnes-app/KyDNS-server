// Package dnssec validates forwarded answers against the root trust anchors.
// Signatures are checked by miekg/dns (and circl for Ed448); this package only
// walks the chain and decides what each answer proves.
package dnssec

import "github.com/miekg/dns"

// rootAnchors are the DS records of the root KSKs in force, from IANA's
// https://data.iana.org/root-anchors/root-anchors.xml (copy in testdata/,
// checked by TestAnchorsMatchIANA). KSK-2010 expired in 2019 and is omitted.
// ponytail: no RFC 5011 rollover; a new root KSK ships in a release.
var rootAnchors = []*dns.DS{
	mustDS(". 0 IN DS 20326 8 2 E06D44B80B8F1D39A95C0B0D7C65D08458E880409BBC683457104237C7F8EC8D"), // KSK-2017
	mustDS(". 0 IN DS 38696 8 2 683D2D0ACB8C9B712A1948B27F741219298D0A450D612C483AF444A4C0FB2B16"), // KSK-2024
}

func mustDS(s string) *dns.DS {
	rr, err := dns.NewRR(s)
	if err != nil {
		panic(err)
	}
	return rr.(*dns.DS)
}
