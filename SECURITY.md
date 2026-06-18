# Security Policy

## Project status

VaultForge is an educational and portfolio project.

It is not an audited password manager.

## Supported data

Only synthetic test data may be used until client-side encryption is complete
and independently reviewed.

Do not store real:

- Passwords
- API keys
- Private keys
- Database credentials
- Access tokens
- Refresh tokens
- Personal secrets

## Reporting a vulnerability

Do not include passwords, tokens, private keys, real credentials, or decrypted
vault contents in a public issue.

Report suspected security problems privately to the repository owner.

TODO: Add a private security-reporting contact method before publishing the
repository broadly.

## Known limitations

- VaultForge has not received an independent security audit.
- Client-side encryption may not exist during early development.
- Local development may use HTTP between local services.
- Account recovery is not implemented.
- A compromised browser can access decrypted data while the vault is unlocked.
- Some non-secret metadata may remain visible to backend services.

See docs/threat-model.md for the full threat model, accepted risks, and trust
boundaries.
