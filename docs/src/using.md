# Everyday tasks

## Names and services

Add a service address and optional aliases, then verify them from a client using this resolver. A DNS name only resolves an address: HTTPS routing and a certificate for that name still belong to the reverse proxy/application setup.

## Filtering and upstreams

Review built-in and operator-managed blocklists and opt-outs. Configure the documented upstream encryption mode and test resolution after changing it. Keep query access scoped to the intended LAN; widening `allow_query` requires deliberate network review.

## DHCP and replication

The built-in DHCP service and replication have their own setup and primary-failure behavior in the product reference. Verify those paths before moving clients. A second node is not a substitute for tested backups.

## Backups

Settings → Backups → Disaster recovery pins/pairs a suite public key and shows destinations, schedule and newest local copy. Preserve the key pin. Restore needs the sealed capsule and enough custodian cards, entered through stdin according to the runbook.
