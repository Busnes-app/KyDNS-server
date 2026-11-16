# Getting started

## Install safely

Follow the package, Docker or Raspberry Pi installation path in the product reference. Preserve configuration, database and keys across updates. Set the private domain and the LAN query allow-list before directing clients at the resolver.

## Register a service

For example, `kydns service add kypost --address 192.168.1.20 --alias webmail --check https://kypost.home.arpa/health` creates names in the configured private domain. Substitute your own address and health URL. Point one test client at KyDNS and confirm both service and ordinary upstream resolution.

Do not enable built-in DHCP alongside an existing DHCP service without planning the change. Configure the recovery key and a destination before relying on scheduled backups.
