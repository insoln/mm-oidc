---
name: plugin-install
description: Install and configure the Mattermost OIDC plugin in a Mattermost server instance. Use this skill when deploying the plugin to production, staging, or test environments, or when updating plugin configuration.
license: Apache-2.0
metadata:
  author: insoln
  version: "1.0"
  category: deployment
  tags: [installation, configuration, deployment, mattermost]
---

# Plugin Install Skill

## Summary
This skill guides the installation and configuration of the Mattermost OIDC plugin, including uploading the plugin archive, configuring OIDC settings, enabling the plugin, and validating the installation.

## When to Use This Skill
- Installing plugin on production Mattermost server
- Deploying plugin to staging or test environments
- Updating plugin configuration
- Migrating from existing authentication methods
- Troubleshooting installation issues

## Prerequisites
- Mattermost Server v9.0+
- System administrator access to Mattermost
- Plugin archive file (`mm-oidc.tar.gz`)
- OIDC provider (Keycloak) configured with client credentials
- HTTPS enabled (or `Allow insecure issuer` for dev/test)

## Installation Methods

### Method 1: System Console (Web UI)

#### Step 1: Enable Plugin Uploads
1. Sign in as system admin
2. Navigate to **System Console → Plugin Management → Management**
3. Enable **Plugin Uploads** if disabled
4. Click **Save**

#### Step 2: Upload Plugin
1. In same page, scroll to **Upload Plugin**
2. Click **Choose File** and select `mm-oidc.tar.gz`
3. Click **Upload**
4. Wait for upload to complete

#### Step 3: Enable Plugin
1. Find `Mattermost OIDC (com.mm.oidc)` in plugin list
2. Click **Enable** button
3. Plugin status should change to "Running"

### Method 2: Using mmctl CLI

```bash
# Login to Mattermost
mmctl auth login https://chat.example.com

# Upload and install plugin
mmctl plugin add mm-oidc.tar.gz

# Enable plugin
mmctl plugin enable com.mm.oidc

# Verify installation
mmctl plugin list
```

### Method 3: Automated Deployment

```bash
# Using dev-bootstrap script (local dev)
./scripts/dev-up.sh

# Or manual upload in container
docker compose -f deploy/docker-compose.dev.yml exec mattermost \
  mmctl plugin add --force /plugins/mm-oidc.tar.gz
```

## Configuration

### Step 1: Navigate to Plugin Settings
1. **System Console → Plugins → Mattermost OIDC**

### Step 2: Configure OIDC Provider

#### Required Settings:

**Issuer URL**
- Format: `https://<keycloak-host>/realms/<realm>`
- Example: `https://keycloak.example.com/realms/mattermost`
- Must be HTTPS in production
- Obtain from Keycloak realm settings or discovery endpoint

**Client ID**
- The client identifier configured in Keycloak
- Example: `mm-oidc`
- Case-sensitive, must match exactly

**Client Secret**
- Confidential secret from Keycloak client credentials tab
- Click "Regenerate" in Keycloak if needed
- Never commit to source control

**Redirect URL**
- Format: `https://<mattermost-host>/plugins/com.mm.oidc/callback`
- Example: `https://chat.example.com/plugins/com.mm.oidc/callback`
- Must match redirect URI configured in Keycloak
- Use proxy hostname if proxy is deployed

**Scopes**
- Default: `openid profile email`
- Add `roles` if using role mapping
- Space-separated list

#### Optional Settings:

**Allow Insecure Issuer**
- Enable only for dev/test with HTTP issuer
- Always disable in production
- Does not bypass certificate validation

### Step 3: Save Configuration
1. Click **Save** at bottom of settings page
2. Plugin automatically restarts with new config

## Validation

### Check Plugin Health
```bash
# Via curl
curl https://chat.example.com/plugins/com.mm.oidc/health

# Expected response:
# {"status":"ok","issuer":"https://keycloak.example.com/realms/mattermost"}
```

### Test Login Flow
1. Log out of Mattermost
2. Navigate to plugin landing page:
   `https://chat.example.com/plugins/com.mm.oidc/`
3. Click **Start Login** button
4. Authenticate with Keycloak
5. Verify redirect back to Mattermost with valid session
6. Check user profile is populated correctly

### Verify Configuration
```bash
# Using mmctl
mmctl config get PluginSettings.Plugins.com.mm.oidc

# Check logs for errors
mmctl logs --logrus
```

## Configuration Examples

### Production Setup (Keycloak)
```
Issuer URL: https://keycloak.company.com/realms/production
Client ID: mattermost-prod
Client Secret: [generate in Keycloak]
Redirect URL: https://chat.company.com/plugins/com.mm.oidc/callback
Scopes: openid profile email roles
Allow Insecure Issuer: false
```

### Development Setup
```
Issuer URL: http://keycloak.127.0.0.1.nip.io:8080/realms/master
Client ID: mm-oidc
Client Secret: [from dev.env]
Redirect URL: http://mattermost-proxy.127.0.0.1.nip.io:8787/plugins/com.mm.oidc/callback
Scopes: openid profile email
Allow Insecure Issuer: true
```

### Multi-Tenant Setup
```
# Tenant A
Issuer URL: https://keycloak.company.com/realms/tenant-a
Redirect URL: https://tenant-a.company.com/plugins/com.mm.oidc/callback

# Tenant B
Issuer URL: https://keycloak.company.com/realms/tenant-b
Redirect URL: https://tenant-b.company.com/plugins/com.mm.oidc/callback
```

## Common Installation Issues

### Plugin Upload Fails
**Problem:** "Error uploading plugin" or size limit exceeded

**Solution:**
```bash
# Check max file size setting
mmctl config get FileSettings.MaxFileSize

# Increase if needed (size in bytes)
mmctl config set FileSettings.MaxFileSize 100000000

# Or upload via mmctl
mmctl plugin add mm-oidc.tar.gz
```

### Plugin Won't Enable
**Problem:** Plugin shows "Error" status after enabling

**Solution:**
1. Check Mattermost logs:
   ```bash
   mmctl logs --logrus | grep -i plugin
   ```
2. Common causes:
   - Binary architecture mismatch (ensure Linux AMD64)
   - Missing dependencies
   - Invalid plugin.json
3. Verify plugin archive:
   ```bash
   tar -tzf mm-oidc.tar.gz
   ```

### Configuration Not Saved
**Problem:** Settings revert after saving

**Solution:**
- Check file system permissions on Mattermost config
- Ensure database connection is working
- Verify no environment variable overrides
- Check Mattermost logs for permission errors

### Health Endpoint Returns 404
**Problem:** `/plugins/com.mm.oidc/health` not accessible

**Solution:**
- Verify plugin is enabled: `mmctl plugin list`
- Check plugin status is "Running"
- Restart plugin: `mmctl plugin disable/enable com.mm.oidc`
- Check Mattermost routing configuration

## Security Considerations

### Production Deployment
1. **Always use HTTPS** for issuer and redirect URLs
2. **Rotate client secrets** regularly
3. **Restrict System Console access** to authorized admins
4. **Audit plugin configuration** changes
5. **Monitor failed login attempts**
6. **Enable rate limiting** on login endpoints
7. **Use strong, random client secrets** (32+ characters)

### Configuration Best Practices
```bash
# Store secrets securely
# Use environment variables or secret manager
export OIDC_CLIENT_SECRET="$(vault read -field=secret secret/mm-oidc)"

# Set via mmctl without exposing in shell history
mmctl config set PluginSettings.Plugins.com.mm.oidc.client_secret --secret

# Or use config file with restricted permissions
chmod 600 /opt/mattermost/config/config.json
```

## Updating the Plugin

### Upgrade to New Version
```bash
# 1. Download new version
curl -LO https://github.com/insoln/mm-oidc/releases/download/v0.0.3/mm-oidc.tar.gz

# 2. Upload new version
mmctl plugin add --force mm-oidc.tar.gz

# 3. Enable (if disabled)
mmctl plugin enable com.mm.oidc

# 4. Verify new version
mmctl plugin list | grep com.mm.oidc
```

### Rollback to Previous Version
```bash
# 1. Disable current plugin
mmctl plugin disable com.mm.oidc

# 2. Delete current plugin
mmctl plugin delete com.mm.oidc

# 3. Upload previous version
mmctl plugin add mm-oidc-v0.0.2.tar.gz

# 4. Enable plugin
mmctl plugin enable com.mm.oidc
```

### Configuration Migration
- Plugin preserves configuration across updates
- Check release notes for breaking changes
- Test in staging before production update
- Backup configuration before major updates

## Integration with Proxy

If deploying with reverse proxy for automatic redirects:

### Update Mattermost Site URL
```bash
# Must match proxy hostname
mmctl config set ServiceSettings.SiteURL https://chat.example.com
```

### Update Plugin Redirect URL
```
# Use proxy hostname, not Mattermost internal address
Redirect URL: https://chat.example.com/plugins/com.mm.oidc/callback
```

### Verify Proxy Configuration
```bash
# Test redirect behavior
curl -i https://chat.example.com/

# Should redirect to plugin login
# Location: /plugins/com.mm.oidc/login?redirect_to=/
```

## Monitoring and Observability

### Health Checks
```bash
# Add to monitoring system
curl -f https://chat.example.com/plugins/com.mm.oidc/health || alert
```

### Log Analysis
```bash
# Watch for errors
mmctl logs --logrus | grep -i "oidc\|plugin"

# Check authentication attempts
mmctl logs | grep "oidc_login_attempt"
```

### Metrics Collection
- Plugin exports Prometheus metrics if configured
- Monitor: `oidc_login_attempt_total`, `oidc_login_duration_seconds`
- Set up alerts for failed login spikes

## Multi-Instance Deployment

### Kubernetes/HA Setup
```yaml
# Ensure consistent plugin installation across pods
# Use init container to download plugin
initContainers:
  - name: plugin-installer
    image: curlimages/curl
    command:
      - sh
      - -c
      - |
        curl -LO https://github.com/insoln/mm-oidc/releases/download/v0.0.2/mm-oidc.tar.gz
        mv mm-oidc.tar.gz /plugins/
    volumeMounts:
      - name: plugins
        mountPath: /plugins
```

### Configuration Synchronization
- Use ConfigMap or Secret for plugin settings
- Ensure all instances use same configuration
- Coordinate plugin upgrades across instances

## Related Skills
- keycloak-setup
- proxy-setup
- troubleshooting
- security-audit
- observability

## References
- [docs/USER_GUIDE.md](../../../docs/USER_GUIDE.md)
- [docs/DEVELOPER_GUIDE.md](../../../docs/DEVELOPER_GUIDE.md)
- [plugin.json](../../../plugin.json)
