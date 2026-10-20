# Packages

[Product overview](../README.md) · [Standalone installation](../INSTALL.md)

Commands and unlinked file paths below are relative to the repository root.

## Install from a package

Debian and Ubuntu, including Raspberry Pi OS, on `amd64` and `arm64`:

```sh
# pick the .deb matching your architecture from the latest release
curl -LO https://github.com/Busnes-app/kydns-server/releases/latest/download/kydns_<version>_arm64.deb
```

Verify it came from this repository's release workflow at that version's tag before installing:

```sh
gh attestation verify kydns_<version>_arm64.deb --repo Busnes-app/KyDNS-server \
  --cert-identity https://github.com/Busnes-app/KyDNS-server/.github/workflows/release.yml@refs/tags/v<version> \
  && sudo apt install ./kydns_<version>_arm64.deb
```

Fedora, and the RHEL 9 and 10 family including Rocky and Alma, on `x86_64`
and `aarch64`:

```sh
# pick the .rpm matching your architecture from the latest release
curl -LO https://github.com/Busnes-app/kydns-server/releases/latest/download/kydns-<version>-1.aarch64.rpm
gh attestation verify kydns-<version>-1.aarch64.rpm --repo Busnes-app/KyDNS-server \
  --cert-identity https://github.com/Busnes-app/KyDNS-server/.github/workflows/release.yml@refs/tags/v<version> \
  && sudo dnf install ./kydns-<version>-1.aarch64.rpm
```

Neither package starts KyDNS, because it wants port 53 and your host may
already run a resolver. Check first:

```sh
sudo ss -lnup 'sport = :53'
```

The `.deb` is enabled at boot already, so it only needs starting:

```sh
sudo systemctl start kydns
```

The `.rpm` follows your host's systemd presets, which leave a new service
disabled, so it needs both:

```sh
sudo systemctl enable --now kydns
```

Then read the one-time setup token:

```sh
sudo cat /var/lib/kydns/setup-token
```

The web UI listens on `127.0.0.1:8053` only. KyDNS speaks plain HTTP, so reach
it over an SSH tunnel:

```sh
ssh -L 8053:127.0.0.1:8053 <this-host>
```

then open `http://127.0.0.1:8053/setup` and enter the token to create your
admin account. To publish the UI on your LAN, use a TLS-terminating reverse
proxy and firewall the admin port so only that proxy can reach it.
`KYDNS_TRUSTED_PROXIES` controls forwarded headers, not network access.
For a proxy on another host, change `admin.listen` in
`/etc/kydns/kydns.yaml` to `0.0.0.0:8053` and set `KYDNS_TRUSTED_PROXIES` to the
proxy's address (comma-separated IPs or CIDRs) with `systemctl edit kydns`.

| Path | What it is |
|---|---|
| `/etc/kydns/kydns.yaml` | Configuration. Your edits survive upgrades. |
| `/var/lib/kydns` | Database and tokens. **Back this up.** |
| `/usr/share/doc/kydns/kydns.example.yaml` | Every setting, documented. |
| `/usr/share/doc/kydns/copyright`, `/usr/share/licenses/kydns/LICENSE.txt` | The MIT licence text, deb and rpm respectively. |

Removing the package leaves `/var/lib/kydns` in place — with `apt purge` and
with `dnf remove` alike. It holds your whole registry and every credential.
Delete it yourself when you are sure.
