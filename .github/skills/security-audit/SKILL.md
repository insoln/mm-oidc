---
name: security-audit
description: Perform security audits and enforce best practices for the Mattermost OIDC plugin including HTTPS validation, secret management, PKCE implementation, and vulnerability scanning. Use this skill when conducting security reviews or preparing for production deployment.
license: Apache-2.0
metadata:
  author: insoln
  version: "1.0"
  category: security
  tags: [security, audit, compliance, best-practices, vulnerabilities]
---

# Security Audit Skill

## Summary
This skill provides comprehensive security checks and best practices for the Mattermost OIDC plugin to ensure secure authentication and protect against common vulnerabilities.

## When to Use This Skill
- Pre-production security review
- Compliance audits
- Security incident investigation
- Regular security assessments
- Preparing for penetration testing

## Security Checklist

### HTTPS Enforcement

**Requirements:**
- [ ] Issuer URL uses HTTPS in production
- [ ] Redirect URL uses HTTPS in production
- [ ] Mattermost Site URL uses HTTPS
- [ ] "Allow insecure issuer" is disabled in production
- [ ] Valid TLS certificates (not self-signed in production)

**Check:**
```bash
# Verify issuer URL
mmctl config get PluginSettings.Plugins.com.mm.oidc.issuer_url
# Must start with https://

# Verify redirect URL
mmctl config get PluginSettings.Plugins.com.mm.oidc.redirect_url
# Must start with https://

# Check allow insecure flag
mmctl config get PluginSettings.Plugins.com.mm.oidc.allow_insecure_issuer
# Must be false
```

### Secret Management

**Requirements:**
- [ ] Client secret stored securely (not in source code)
- [ ] Client secret not exposed in logs
- [ ] Client secret rotated regularly (every 90 days)
- [ ] Access to System Console restricted to authorized admins
- [ ] Configuration backups encrypted

**Check:**
```bash
# Verify secret not logged
grep -i "client_secret" mattermost.log
# Should show [REDACTED] or similar

# Check config file permissions
ls -l /opt/mattermost/config/config.json
# Should be 600 or 640
```

**Rotate Secret:**
1. Generate new secret in Keycloak
2. Update plugin configuration
3. Test authentication
4. Invalidate old secret

### PKCE Implementation

**Requirements:**
- [ ] PKCE enabled for all flows
- [ ] Code challenge method is S256 (SHA-256)
- [ ] Code verifier securely generated (random, sufficient entropy)
- [ ] Code verifier not logged or exposed

**Verify:**
```bash
# Capture authorization request
# Check for code_challenge parameter
curl -v "https://chat.example.com/plugins/com.mm.oidc/login" 2>&1 | grep code_challenge

# Should show:
# code_challenge=<base64-encoded-hash>
# code_challenge_method=S256
```

### State and Nonce Validation

**Requirements:**
- [ ] State parameter generated for each auth request
- [ ] State validated on callback
- [ ] State stored with short TTL (5-10 minutes)
- [ ] Nonce validated in ID token
- [ ] Protection against replay attacks

**Check:**
```bash
# Review plugin code for state validation
grep -r "validateState" server/

# Check session storage configuration
# Verify TTL settings
```

### Token Handling

**Requirements:**
- [ ] ID tokens validated (signature, issuer, audience, expiration)
- [ ] Access tokens kept in-memory only
- [ ] Refresh tokens encrypted before storage
- [ ] Tokens not logged
- [ ] Token expiration enforced

**Verify:**
```bash
# Check no tokens in logs
grep -E "(access_token|id_token|refresh_token)" mattermost.log
# Should show [REDACTED] or no matches

# Verify token validation in code
grep -r "verifyIDToken" server/
```

### Input Validation

**Requirements:**
- [ ] All user inputs validated and sanitized
- [ ] URL parameters validated
- [ ] Redirect URLs validated against allowlist
- [ ] No SQL injection vulnerabilities
- [ ] No XSS vulnerabilities

**Check:**
```bash
# Review code for input validation
grep -r "validateInput\|sanitize" server/

# Test with malicious inputs
curl "https://chat.example.com/plugins/com.mm.oidc/login?redirect_to=javascript:alert(1)"
# Should be rejected or sanitized
```

### Error Handling

**Requirements:**
- [ ] Generic error messages to clients
- [ ] Detailed errors only in server logs
- [ ] No stack traces exposed to users
- [ ] No sensitive data in error messages

**Test:**
```bash
# Trigger error with invalid state
curl "https://chat.example.com/plugins/com.mm.oidc/callback?code=test&state=invalid"
# Should return generic error, not internal details
```

### Dependency Security

**Requirements:**
- [ ] All dependencies up to date
- [ ] No known vulnerabilities in dependencies
- [ ] Dependabot enabled for automated updates
- [ ] Regular vulnerability scans

**Check:**
```bash
# Go dependencies
cd server
go list -m all
govulncheck ./...

# Node dependencies
cd webapp
yarn audit

# Check for updates
yarn upgrade-interactive
```

### Network Security

**Requirements:**
- [ ] TLS 1.2+ enforced
- [ ] Strong cipher suites configured
- [ ] HTTP Strict Transport Security (HSTS) enabled
- [ ] Security headers configured (X-Frame-Options, CSP, etc.)

**Verify:**
```bash
# Test TLS configuration
nmap --script ssl-enum-ciphers -p 443 chat.example.com

# Check security headers
curl -I https://chat.example.com | grep -E "(Strict-Transport|X-Frame|Content-Security)"
```

## Vulnerability Assessment

### Common OIDC Vulnerabilities

**1. Authorization Code Interception**
- Mitigation: PKCE enforced
- Check: Verify code_challenge in auth request

**2. Redirect URI Manipulation**
- Mitigation: Strict redirect URI validation
- Check: Test with unauthorized redirect URIs

**3. State Parameter Fixation**
- Mitigation: Random state generation and validation
- Check: Attempt to reuse state parameter

**4. Token Leakage**
- Mitigation: Tokens not logged or exposed
- Check: Search logs and responses for tokens

**5. Insufficient Client Authentication**
- Mitigation: Client secret required (confidential client)
- Check: Verify Keycloak client type is confidential

### Penetration Testing

**Recommended Tests:**
1. Try bypassing authentication
2. Test for CSRF vulnerabilities
3. Attempt session fixation
4. Test token replay attacks
5. Try redirect URI bypass
6. Test for timing attacks
7. Check for information disclosure

**Tools:**
- OWASP ZAP for automated scanning
- Burp Suite for manual testing
- jwt_tool for JWT analysis

## Compliance

### GDPR Considerations

- [ ] User consent for data processing
- [ ] Data minimization (only collect necessary claims)
- [ ] Right to erasure implemented
- [ ] Data processing agreement with IdP
- [ ] Audit logging for access to personal data

### SOC 2 / ISO 27001

- [ ] Access controls documented
- [ ] Incident response procedures defined
- [ ] Regular security assessments scheduled
- [ ] Vendor risk assessment for Keycloak
- [ ] Data classification and handling procedures

## Monitoring and Alerting

### Security Metrics

- Failed authentication attempts
- Unusual login patterns
- Token validation failures
- Configuration changes
- Plugin enable/disable events

**Setup:**
```bash
# Export Prometheus metrics
curl http://localhost:8067/metrics | grep oidc

# Alert rules
# Rate of failed logins > threshold
# Token validation errors spike
# Plugin configuration changes
```

### Audit Logging

```bash
# Enable audit logging in Mattermost
mmctl config set ExperimentalAuditSettings.FileEnabled true

# Review audit logs
tail -f /opt/mattermost/logs/audit.log | grep oidc
```

## Incident Response

### Security Incident Checklist

1. **Contain:** Disable plugin if needed
2. **Investigate:** Review logs and metrics
3. **Remediate:** Fix vulnerability
4. **Verify:** Test fix thoroughly
5. **Document:** Record incident details
6. **Improve:** Update procedures

### Emergency Actions

```bash
# Disable plugin immediately
mmctl plugin disable com.mm.oidc

# Rotate client secret
# 1. Generate new in Keycloak
# 2. Update plugin config
# 3. Enable plugin

# Invalidate all sessions
mmctl user deleteallsessions
```

## Related Skills
- plugin-install
- troubleshooting
- observability
- keycloak-setup

## References
- [docs/ARCHITECTURE.md](../../../docs/ARCHITECTURE.md)
- [OWASP ASVS](https://owasp.org/www-project-application-security-verification-standard/)
- [OIDC Security Best Practices](https://openid.net/specs/openid-connect-core-1_0.html#Security)
