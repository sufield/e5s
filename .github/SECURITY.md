# Security Policy

## Supported Versions

The following versions of e5s are currently supported with security updates:

| Version | Supported          | Notes                          |
| ------- | ------------------ | ------------------------------ |
| 0.x.x   | :white_check_mark: | Current development version    |

As this project is in active development (pre-1.0), all security fixes are applied to the main branch. Once version 1.0 is released, we will maintain security updates for the latest major version and the previous major version for 6 months after a new major release.

## Reporting a Vulnerability

**We take security vulnerabilities seriously.** If you discover a security issue in e5s, please report it responsibly through one of these private channels:

### Preferred Method: GitHub Security Advisories
Report vulnerabilities privately through GitHub's Security Advisory feature:
- Navigate to: https://github.com/sufield/e5s/security/advisories/new
- Provide a detailed description of the vulnerability
- Include steps to reproduce, if possible
- We will acknowledge your report within 48 hours

### Alternative: Email
If you prefer email, send vulnerability reports to:
- **Email**: security@sufield.dev
- **Subject**: [SECURITY] e5s vulnerability report
- **PGP Key**: Available at https://keybase.io/sufield (optional but recommended)

### What to Include in Your Report
Please include the following information to help us assess and address the vulnerability:
- Description of the vulnerability
- Steps to reproduce the issue
- Potential impact and severity assessment
- Any proof-of-concept code (if applicable)
- Suggested fix or mitigation (if you have one)

## Response Timeline

We are committed to addressing security vulnerabilities promptly:

1. **Acknowledgment**: Within 48 hours of receiving your report
2. **Initial Assessment**: Within 7 days, we will provide an assessment of severity and expected timeline for a fix
3. **Fix Development**: Critical vulnerabilities will be prioritized and addressed within 14 days when possible
4. **Disclosure**: We follow coordinated disclosure practices:
   - We will work with you to determine an appropriate disclosure timeline
   - Typically, we disclose after a fix is available and users have had time to update
   - We will credit you in the security advisory (unless you prefer to remain anonymous)

## Security Advisories

Security advisories for e5s are published through:
- [GitHub Security Advisories](https://github.com/sufield/e5s/security/advisories)
- Release notes for security-related releases
- The project's main README

## Scope

This security policy applies to:
- The e5s library code (all packages in this repository)
- Official examples and documentation
- CI/CD workflows and build scripts

Out of scope:
- Third-party dependencies (report to the respective maintainers)
- Applications built using e5s (unless the vulnerability is in e5s itself)

## Security Best Practices for Users

When using e5s in production:
- Always use the latest stable release
- Keep your SPIRE infrastructure updated
- Follow SPIFFE/SPIRE security best practices
- Use TLS 1.3 when possible
- Implement proper certificate rotation
- Monitor for security advisories

## Security Features

e5s includes the following security features:
- mTLS authentication using SPIFFE identities
- Automatic certificate rotation via SPIRE Workload API
- Protection against Slowloris attacks (ReadHeaderTimeout)
- Path traversal prevention in configuration loading
- Regular security scanning via:
  - CodeQL (semantic code analysis)
  - gosec (Go security checker)
  - govulncheck (Go vulnerability scanner)
  - gitleaks (secret scanning)

## Contact

For general security questions (not vulnerability reports), you can:
- Open a discussion on GitHub: https://github.com/sufield/e5s/discussions
- Contact the maintainers via email: maintainers@sufield.dev

For vulnerability reports, please use the private reporting channels listed above.

---

**Thank you for helping keep e5s and its users secure!**
