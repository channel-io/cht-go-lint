# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 0.1.x   | Yes       |

## Reporting a Vulnerability

If you discover a security vulnerability in cht-go-lint, please report it responsibly.

**Do not open a public GitHub issue for security vulnerabilities.**

Instead, please email your report to the maintainers via the contact information in the repository or open a private security advisory at:

https://github.com/channel-io/cht-go-lint/security/advisories/new

### What to include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### Response timeline

- We will acknowledge receipt within 3 business days
- We will provide an initial assessment within 7 business days
- We aim to release a fix within 30 days for confirmed vulnerabilities

## Scope

cht-go-lint is a static analysis tool that reads Go source code. It does not execute analyzed code, make network requests, or modify files (except for the `init` command which creates a config file).

Potential security concerns are limited to:
- Path traversal when resolving config or source files
- Denial of service via crafted Go source files
