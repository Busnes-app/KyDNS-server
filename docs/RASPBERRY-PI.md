# Raspberry pi

[Product overview](../README.md) · [Standalone installation](../INSTALL.md)

Commands and unlinked file paths below are relative to the repository root.

## Raspberry Pi SD image

For a Raspberry Pi 3, 4, 5, or Zero 2 W running 64-bit Raspberry Pi OS Lite,
build the image from the arm64 package:

```sh
make rpi-image VERSION=0.0.0-dev
sudo dd if=dist/kydns_0.0.0-dev_rpi64.img of=/dev/sdX bs=4M status=progress conv=fsync
```

The tagged release asset is compressed to `*.img.xz`; unpack it with
`xz -d` before flashing.

Replace `/dev/sdX` with the whole SD card device. The image is a Raspberry Pi
OS Lite base, not a new distribution. On its first boot it installs the
staged `.deb`, enables KyDNS, and starts it. Raspberry Pi OS first-boot setup
still supplies the normal user, password, hostname, and network choices; the
image needs network access on that boot. Read the setup token with:

```sh
sudo cat /var/lib/kydns/setup-token
```

The base is pinned. `packaging/rpi-base.pin` names one official arm64 Lite
release and the SHA-256 Raspberry Pi published for it, and the builder refuses
a download that does not match — a release is signed and attested, so what it
was built on top of is a reviewed decision rather than whatever the mirror
served that morning. A weekly job proposes the next release once it has been
out for three days; `RPI_BASE_URL` and `RPI_BASE_SHA256` override the pin for
a local build. The output is not safe to flash while compressed and is
intentionally not a raw device path.
