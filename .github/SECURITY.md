# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| latest (main) | ✅ |
| older releases | ❌ |

## Reporting a Vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Report privately via [GitHub Security Advisories](../../security/advisories/new) or email the maintainers directly (see `FUNDING.yml` for contact information).

Include:
- Description of the vulnerability and potential impact
- Steps to reproduce
- Affected versions
- Suggested fix (optional)

You will receive a response within 72 hours. If the vulnerability is confirmed, a fix will be prioritized and a CVE requested if warranted.

## Security Considerations

- **API keys**: Never commit API keys. Use environment variables.
- **Bond TTL**: Set `LockTTL` appropriate to your `ReasoningTimeout` (see `docs/concepts.md`).
- **Input validation**: `InputGuard` is enabled by default — do not disable in production.
- **TLS**: Always use TLS for Redis connections in production (`rediss://`).

## Operator Trust Model

**Operator credentials are high-privilege credentials.** Treat them with the same care as database admin credentials.

Any user with `operator` or `admin` role on the management API can:

- **Edit any Spirit's `SystemPrompt`** — the system prompt is injected directly into every LLM call for every end-user conversation handled by that Spirit. A compromised or malicious operator can inject arbitrary instructions, data-exfiltration commands, or disinformation into the LLM's context.
- **Create and deactivate operator accounts** (admin role).
- **Read full conversation history** via the Chronicle endpoint.

### Current audit trail

Spirit edits are tracked by the `Spirit.Version` counter and the full document history is preserved in MongoDB. However, **there is no real-time alert or separate audit log for admin actions**. If an operator makes a change and reverts it, the only record is the version history in the Spirit document.

### Recommendations for production

1. **Rotate operator credentials regularly.** Use short-lived JWTs where possible.
2. **Restrict admin role.** Only the minimum number of accounts that need to create/deactivate operators should hold the `admin` role.
3. **Monitor Spirit version bumps** in your observability stack. Unexpected `version` increments on a Spirit document are a signal worth alerting on.
4. **Use OIDC/JWKS** (`NewJWKSValidator`) for human operator logins rather than static API keys where your IdP enforces MFA.

This is a known limitation of v1. A two-person review flow for Spirit changes in high-assurance environments is on the roadmap.
