# Security Policy

## Supported Versions

Security fixes are provided for the latest tagged release.

| Version | Supported |
| ------- | --------- |
| 0.2.x   | Yes       |
| < 0.2   | No        |

## Reporting a Vulnerability

Do not open a public issue for a suspected vulnerability. Use GitHub's private
[security advisory form](https://github.com/caelyreth/untie/security/advisories/new)
and include:

- the affected Untie version or commit;
- the input, font, host environment, and configuration needed to reproduce it;
- the observed impact and any known mitigations.

Maintainers will validate the report, coordinate a fix, and publish an advisory
when a release is available. Please avoid disclosing the issue publicly until
the advisory or fix is released.

## Scope

Reports about parser panics, resource-limit bypasses, unsafe IR or display
invariants, font parsing, WebAssembly integration, and the documentation site
are in scope. Vulnerabilities in third-party dependencies should also be
reported to the relevant upstream project.
