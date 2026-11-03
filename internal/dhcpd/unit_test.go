package dhcpd

import (
	"os"
	"regexp"
	"testing"
)

// Go reads the interface table over netlink, so a packaged unit that sandboxes
// address families without AF_NETLINK makes Inspect and Qualifies fail on every
// .deb, .rpm and Raspberry Pi install.
func TestPackagedUnitAllowsNetlink(t *testing.T) {
	b, err := os.ReadFile("../../packaging/kydns.service")
	if err != nil {
		t.Fatal(err)
	}
	line := regexp.MustCompile(`(?m)^RestrictAddressFamilies=(.*)$`).FindSubmatch(b)
	if line == nil {
		return // no sandbox, nothing to refuse netlink
	}
	if !regexp.MustCompile(`\bAF_NETLINK\b`).Match(line[1]) {
		t.Errorf("RestrictAddressFamilies=%s lacks AF_NETLINK", line[1])
	}
}
