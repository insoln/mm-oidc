---
name: troubleshooting
description: Diagnose and resolve common issues with the Mattermost OIDC plugin including authentication failures, configuration errors, redirect loops, and integration problems. Use this skill when debugging production issues or investigating user-reported problems.
license: Apache-2.0
metadata:
  author: insoln
  version: "1.0"
  category: operations
  tags: [debugging, troubleshooting, diagnostics, support]
---

# Troubleshooting Skill

## Summary
This skill provides systematic approaches to diagnosing and resolving issues with the Mattermost OIDC plugin, covering common problems, debugging techniques, and resolution strategies.

## When to Use This Skill
- Authentication failures
- Configuration errors
- Redirect loops
- User provisioning issues
- Integration problems
- Production incidents
- Performance degradation

## Diagnostic Approach

### 1. Gather Information
- What is the exact error message?
- When did the problem start?
- Which users are affected?
- What was changed recently?

### 2. Check Component Health
```bash
# Plugin health
curl https://chat.example.com/plugins/com.mm.oidc/health

# Keycloak health
curl https://keycloak.example.com/health

# Mattermost logs
mmctl logs --logrus | grep -i oidc

# Network connectivity
ping keycloak.example.com
```

### 3. Review Configuration
```bash
# Plugin settings
mmctl config get PluginSettings.Plugins.com.mm.oidc

# Keycloak client config
# Check via admin console
```

## Common Issues

### Authentication Failures

#### Symptom: "Invalid redirect_uri"
**Cause:** Mismatch between configured and requested redirect URI

**Debug:**
```bash
# Check plugin config
mmctl config get PluginSettings.Plugins.com.mm.oidc.redirect_url

# Check Keycloak client valid redirect URIs
# Must match exactly including protocol, host, port, path
```

**Solution:**
1. Update Keycloak client redirect URIs
2. Or update plugin redirect URL
3. Ensure no trailing slashes
4. Check http vs https

#### Symptom: "Invalid state parameter"
**Cause:** State mismatch, expired session, or replay attack

**Debug:**
```bash
# Check plugin logs for state validation
grep "state" mattermost.log

# Verify session storage working
# Check Redis/database connectivity
```

**Solution:**
1. Clear browser cookies and retry
2. Check plugin KV store is functional
3. Verify no caching proxy interfering
4. Ensure clock synchronization between servers

#### Symptom: "Missing required claim: email"
**Cause:** ID token missing required claims

**Debug:**
```bash
# Decode ID token
jwt decode <id_token>

# Check Keycloak protocol mappers
# Verify user has email attribute
```

**Solution:**
1. Add missing protocol mappers in Keycloak
2. Ensure user email is set and verified
3. Check mapper includes claim in ID token

### Configuration Issues

#### Symptom: Plugin won't enable
**Cause:** Configuration validation failure or binary incompatibility

**Debug:**
```bash
# Check plugin status
mmctl plugin list

# View detailed error
mmctl logs | grep -A 10 "plugin.*enable"

# Verify binary architecture
file server/dist/plugin-linux-amd64
```

**Solution:**
1. Check logs for validation errors
2. Verify all required settings provided
3. Ensure binary is Linux AMD64
4. Check Mattermost version compatibility

#### Symptom: "HTTPS required" error
**Cause:** HTTP issuer URL in production mode

**Debug:**
```bash
# Check issuer URL
mmctl config get PluginSettings.Plugins.com.mm.oidc.issuer_url
```

**Solution:**
- Use HTTPS issuer URL
- Or enable "Allow insecure issuer" for dev/test only

### Redirect Loop

#### Symptom: Browser keeps redirecting infinitely
**Cause:** Plugin routes being redirected by proxy

**Debug:**
```bash
# Test callback directly
curl -i https://chat.example.com/plugins/com.mm.oidc/callback

# Should return 200 or 400, not 302
```

**Solution:**
1. Exclude `/plugins/` from proxy redirect rules
2. Check proxy configuration
3. Verify no multiple proxies interfering

### User Provisioning Issues

#### Symptom: User not created in Mattermost
**Cause:** Missing claims or provisioning failure

**Debug:**
```bash
# Check provisioning logs
grep "provision" mattermost.log

# Verify required claims present
jwt decode <id_token>
```

**Solution:**
1. Ensure email and username claims present
2. Check email_verified is true
3. Verify no conflicts with existing users

#### Symptom: Admin role not assigned
**Cause:** Role mapping misconfigured

**Debug:**
```bash
# Check if roles claim present
jwt decode <id_token> | grep roles

# Verify system_admin in roles array
```

**Solution:**
1. Assign system_admin role in Keycloak
2. Add roles scope to plugin config
3. Verify role mapper configured correctly

## Debugging Techniques

### Enable Verbose Logging

#### Mattermost
```bash
# Set log level to DEBUG
mmctl config set LogSettings.ConsoleLevel DEBUG

# Or in config.json
"LogSettings": {
  "ConsoleLevel": "DEBUG"
}

# Tail logs
tail -f /opt/mattermost/logs/mattermost.log
```

#### Keycloak
```bash
# Enable event logging
# Keycloak Admin Console → Events → Config
# Enable "Login", "Login Error"

# View event log
# Events → Login events
```

### Capture Network Traffic

```bash
# Using browser DevTools
# 1. Open Network tab
# 2. Initiate login
# 3. Inspect requests/responses

# Using tcpdump
tcpdump -i any -s 0 -w capture.pcap 'host keycloak.example.com'

# Using Wireshark
# Open capture.pcap and filter:
# http.host == "keycloak.example.com"
```

### Test with curl

```bash
# Manual OIDC flow simulation
# 1. Get authorization URL
AUTH_URL="https://keycloak.example.com/realms/mattermost/protocol/openid-connect/auth"
PARAMS="client_id=mm-oidc&redirect_uri=https://chat.example.com/plugins/com.mm.oidc/callback&response_type=code&scope=openid+profile+email"
curl -i "${AUTH_URL}?${PARAMS}"

# 2. Exchange code for tokens
TOKEN_URL="https://keycloak.example.com/realms/mattermost/protocol/openid-connect/token"
curl -X POST ${TOKEN_URL} \
  -d grant_type=authorization_code \
  -d client_id=mm-oidc \
  -d client_secret=<secret> \
  -d code=<code> \
  -d redirect_uri=https://chat.example.com/plugins/com.mm.oidc/callback
```

## Performance Issues

### Symptom: Slow authentication
**Cause:** Network latency, database bottleneck, or inefficient queries

**Debug:**
```bash
# Measure latency
time curl -L https://chat.example.com/plugins/com.mm.oidc/login

# Check Mattermost metrics
curl http://localhost:8067/metrics | grep oidc

# Database query performance
# Check slow query log
```

**Solution:**
1. Optimize network path
2. Enable caching where appropriate
3. Scale database if needed
4. Review plugin queries

### Symptom: High memory usage
**Cause:** Memory leak or excessive caching

**Debug:**
```bash
# Monitor plugin memory
top -p $(pidof mattermost)

# Check for memory leaks
# Use pprof or similar profiler
```

**Solution:**
1. Restart plugin periodically
2. Reduce cache size
3. Report bug with memory profile

## Integration Issues

### Symptom: Mobile app can't login
**Cause:** Proxy redirecting mobile traffic

**Solution:**
1. Detect mobile user agent in proxy
2. Bypass redirect for mobile apps
3. Or route mobile to direct Mattermost endpoint

### Symptom: API clients failing
**Cause:** API requests being redirected

**Solution:**
1. Exclude `/api/*` from proxy redirect
2. Add `Accept: application/json` header bypass
3. Use direct Mattermost URL for API clients

## Recovery Procedures

### Emergency Disable

```bash
# Disable plugin immediately
mmctl plugin disable com.mm.oidc

# Users can still login with password
# Until issue is resolved
```

### Rollback

```bash
# Upload previous version
mmctl plugin add mm-oidc-v0.0.1.tar.gz --force

# Enable
mmctl plugin enable com.mm.oidc
```

### Configuration Reset

```bash
# Backup current config
mmctl config get PluginSettings.Plugins.com.mm.oidc > backup.json

# Reset to defaults
mmctl config set PluginSettings.Plugins.com.mm.oidc.issuer_url ""
mmctl config set PluginSettings.Plugins.com.mm.oidc.client_id ""
# ... reset other settings

# Reconfigure from scratch
```

## Related Skills
- dev-environment
- oidc-flow-test
- security-audit
- observability

## References
- [docs/USER_GUIDE.md](../../../docs/USER_GUIDE.md)
- [docs/KEYCLOAK_SETUP.md](../../../docs/KEYCLOAK_SETUP.md)
- [docs/PROXY_GUIDE.md](../../../docs/PROXY_GUIDE.md)
