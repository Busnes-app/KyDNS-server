# Security model

[Product overview](../README.md) · [Standalone installation](../INSTALL.md)

Commands and unlinked file paths below are relative to the repository root.

## Security model

- DNS data stays on the operator's network.
- The server does not send query history to a KySecurity service.
- Administrative changes require authentication and are recorded locally.
- Configuration exports must not contain upstream credentials or private keys.
- Public DNS names and private service names remain separate by default.

See [DESIGN.md](../DESIGN.md) for the architecture, [LOGGING.md](../LOGGING.md)
for logging requirements, and [SECURITY.md](../SECURITY.md) for security policy.
