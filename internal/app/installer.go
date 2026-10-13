package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/Busnes-app/ky-primitives/applysetup"
	"github.com/Busnes-app/kydns-server/internal/config"
)

func InstallerConfigPath() string {
	if path := os.Getenv("KYDNS_CONFIG"); path != "" {
		return path
	}
	return "/etc/kydns/kydns.yaml"
}

func InstallerSocket(dataDir string) (string, error) {
	dir, err := filepath.Abs(dataDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "installer", "setup.sock"), nil
}

func RunInstaller(args []string, out io.Writer) error {
	opts, err := applysetup.Parse(args)
	if err != nil {
		return err
	}
	if opts.Capabilities {
		return applysetup.WriteCapabilities(out, "kydns", "kyidentity-oidc", "exact-admin-subjects", "admin-only", "directory-webhook", "check")
	}
	if opts.Restart {
		return errors.New("KyDNS setup is live; installer restart is not supported")
	}
	cfg, err := config.Load(InstallerConfigPath())
	if err != nil {
		return errors.New("existing KyDNS configuration unavailable")
	}
	socket, err := InstallerSocket(cfg.DataDir)
	if err != nil {
		return err
	}
	var b applysetup.Common
	if err = applysetup.Decode(opts.File, &b); err != nil {
		return err
	}
	if err = b.Validate("kydns", "/auth/sso/callback"); err != nil {
		return err
	}
	raw, _ := json.Marshal(b)
	report, err := applysetup.CallLocal(context.Background(), socket, raw, opts.Check)
	if err != nil {
		return err
	}
	if report.Service != b.Service || report.SetupID != b.SetupID {
		return errors.New("setup report identity mismatch")
	}
	return report.Write(out)
}
