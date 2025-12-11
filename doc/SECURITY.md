# Security Policy

## Supported Versions

We release patches for security vulnerabilities for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 0.x.x   | :white_check_mark: |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please report them via email to: **security@witnz.tech** (to be set up)

<!-- Temporary: Until email is set up, use GitHub Security Advisories -->
Alternatively, use [GitHub Security Advisories](https://github.com/witnz/witnz/security/advisories/new)

### What to Include

Please include the following information:

- Type of vulnerability
- Full paths of source file(s) related to the vulnerability
- Location of the affected source code (tag/branch/commit)
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the vulnerability

### Response Timeline

- We will acknowledge your email within **48 hours**
- We will provide a detailed response within **7 days**
- We will work on a fix and keep you updated on the progress
- We will notify you when the vulnerability is fixed
- We will publicly disclose the vulnerability after a fix is released

## Security Best Practices

When deploying Witnz:

1. **Network Isolation**: Deploy nodes in a private network or VPN
2. **PostgreSQL Security**: Use strong passwords, restrict network access
3. **Configuration Files**: Protect `witnz.yaml` with appropriate file permissions (600)
4. **Replication User**: Use a dedicated PostgreSQL user with minimal privileges
5. **Regular Updates**: Keep Witnz and PostgreSQL up to date
6. **Monitor Logs**: Regularly review Witnz logs for suspicious activity

## Known Security Considerations

- **Logical Replication Slot**: Creates a dedicated replication slot - ensure PostgreSQL has sufficient storage for WAL retention
- **Hash Storage**: BoltDB files contain hash chains - protect with file system permissions
- **Inter-node Communication**: Currently uses TCP without TLS (Phase 3 will add TLS/mTLS)

## Disclosure Policy

- Security vulnerabilities will be disclosed publicly only after a fix is available
- Credit will be given to the reporter (unless they wish to remain anonymous)
- CVE IDs will be assigned for confirmed vulnerabilities
