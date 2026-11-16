# Troubleshooting

- **Name resolves on the server but not a client:** check which resolver the client actually uses, its cache and the query allow-list.
- **Name resolves but HTTPS fails:** inspect the proxy route and certificate for that hostname; DNS alone does not configure them.
- **Clients lose addresses:** check for competing DHCP servers and the configured scope before changing DNS data.
- **Unexpected block:** inspect the matched filtering list and the intended opt-out.
- **Container permission failure after upgrade:** follow the non-root ownership migration in the product reference.
- **Backup has no destination:** a pinned key alone is insufficient. Configure a local directory or pair KyRecovery.

When reporting a problem, include the product version, platform, failed task and a redacted error. Exclude pairing links, device secrets, recovery codes, custodian shares and private content.
