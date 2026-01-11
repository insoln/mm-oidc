---
name: config-migration
description: Migrate and update plugin configuration between versions, environments, or deployment scenarios. Use this skill when upgrading the plugin, migrating between environments, or handling configuration schema changes.
license: Apache-2.0
metadata:
  author: insoln
  version: "1.0"
  category: operations
  tags: [migration, configuration, upgrade, versioning]
---

# Config Migration Skill

## Summary
This skill provides guidance for migrating configuration settings between plugin versions, environments, and deployment scenarios.

## When to Use This Skill
- Upgrading plugin to new version
- Migrating from dev to production
- Moving between Mattermost instances
- Handling configuration schema changes
- Backing up and restoring configuration

## Configuration Backup

### Export Current Configuration

```bash
# Export plugin configuration to JSON
mmctl config get PluginSettings.Plugins.com.mm.oidc > oidc-config-backup.json

# Or export entire Mattermost config
mmctl config get > mattermost-config-backup.json

# Include timestamp in backup name
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
mmctl config get PluginSettings.Plugins.com.mm.oidc > "oidc-config-$TIMESTAMP.json"
```

### Store Securely

```bash
# Encrypt backup (contains secrets)
gpg --symmetric --cipher-algo AES256 oidc-config-backup.json

# Store in secure location
aws s3 cp oidc-config-backup.json.gpg s3://secure-backups/mattermost/

# Or use HashiCorp Vault
vault kv put secret/mattermost/oidc @oidc-config-backup.json
```

## Configuration Restore

### Import Configuration

```bash
# Restore from backup
mmctl config set PluginSettings.Plugins.com.mm.oidc --config oidc-config-backup.json

# Or restore individual settings
mmctl config set PluginSettings.Plugins.com.mm.oidc.issuer_url "https://keycloak.example.com/realms/mattermost"
mmctl config set PluginSettings.Plugins.com.mm.oidc.client_id "mm-oidc"

# Restore client secret securely without exposing it in shell history or process listings
vault kv get -field=secret secret/mm-oidc | \
  mmctl config set PluginSettings.Plugins.com.mm.oidc.client_secret --value-from-stdin
```

### Verify Configuration

```bash
# Check restored values
mmctl config get PluginSettings.Plugins.com.mm.oidc

# Test plugin functionality
curl https://chat.example.com/plugins/com.mm.oidc/health
```

## Version Upgrades

### Pre-Upgrade Checklist

- [ ] Backup current configuration
- [ ] Review release notes for breaking changes
- [ ] Test upgrade in staging environment
- [ ] Plan rollback procedure
- [ ] Schedule maintenance window

### Upgrade Process

```bash
# 1. Backup configuration
mmctl config get PluginSettings.Plugins.com.mm.oidc > backup-pre-upgrade.json

# 2. Download new version
curl -LO https://github.com/insoln/mm-oidc/releases/download/v0.0.3/mm-oidc.tar.gz

# 3. Upload new plugin
mmctl plugin add --force mm-oidc.tar.gz

# 4. Verify configuration preserved
mmctl config get PluginSettings.Plugins.com.mm.oidc

# 5. Test functionality
./scripts/e2e-test.sh
```

### Post-Upgrade Validation

```bash
# Check plugin version
mmctl plugin list | grep com.mm.oidc

# Verify health
curl https://chat.example.com/plugins/com.mm.oidc/health

# Test login flow
# Navigate to /plugins/com.mm.oidc/ and test authentication

# Monitor logs for errors
mmctl logs --logrus | grep -i oidc
```

## Environment Migration

### Dev to Staging

```bash
# Export from dev
mmctl --local config get PluginSettings.Plugins.com.mm.oidc > dev-config.json

# Modify for staging
cat dev-config.json | jq '.issuer_url = "https://keycloak-staging.example.com/realms/mattermost"' > staging-config.json
cat staging-config.json | jq '.redirect_url = "https://chat-staging.example.com/plugins/com.mm.oidc/callback"' > staging-config.json
cat staging-config.json | jq '.allow_insecure_issuer = false' > staging-config.json

# Import to staging
mmctl --server https://chat-staging.example.com config set PluginSettings.Plugins.com.mm.oidc --config staging-config.json
```

### Staging to Production

```bash
# Extract staging config
mmctl --server https://chat-staging.example.com config get PluginSettings.Plugins.com.mm.oidc > staging-config.json

# Update for production
jq '.issuer_url = "https://keycloak.example.com/realms/mattermost"' staging-config.json > prod-config.json
jq '.redirect_url = "https://chat.example.com/plugins/com.mm.oidc/callback"' prod-config.json > prod-config.json
jq '.client_id = "mm-oidc-prod"' prod-config.json > prod-config.json

# Import production client secret from vault securely
# Using temporary file with restricted permissions to avoid command line exposure
vault kv get -field=secret secret/mm-oidc-prod > /tmp/secret.tmp
chmod 600 /tmp/secret.tmp
jq --rawfile secret /tmp/secret.tmp '.client_secret = $secret' prod-config.json > prod-config-final.json
rm -f /tmp/secret.tmp
mv prod-config-final.json prod-config.json

# Import to production
mmctl --server https://chat.example.com config set PluginSettings.Plugins.com.mm.oidc --config prod-config.json
```

## Configuration Schema Changes

### Version 0.0.1 to 0.0.2 Example

**Added fields:**
- `token_endpoint` (optional, auto-discovered if not set)
- `userinfo_endpoint` (optional, auto-discovered if not set)

**Changed fields:**
- `scopes`: Now comma or space separated (was space-only)

**Removed fields:**
- None

**Migration:**
```bash
# No manual migration needed
# Plugin handles backward compatibility
# New fields populated automatically via discovery
```

### Handling Breaking Changes

If a future version has breaking changes:

```bash
# 1. Export current config
mmctl config get PluginSettings.Plugins.com.mm.oidc > old-config.json

# 2. Transform to new schema (example)
cat old-config.json | \
  jq '.new_field = .old_field | del(.old_field)' | \
  jq '.renamed_field = .previous_name | del(.previous_name)' \
  > new-config.json

# 3. Import new config
mmctl config set PluginSettings.Plugins.com.mm.oidc --config new-config.json
```

## Multi-Instance Configuration

### Shared Configuration Template

```json
{
  "issuer_url": "${OIDC_ISSUER_URL}",
  "client_id": "${OIDC_CLIENT_ID}",
  "client_secret": "${OIDC_CLIENT_SECRET}",
  "redirect_url": "${MM_SITE_URL}/plugins/com.mm.oidc/callback",
  "scopes": "openid profile email",
  "allow_insecure_issuer": false
}
```

### Apply to Multiple Instances

```bash
# Using environment-specific values
for ENV in dev staging prod; do
  source ./config/$ENV.env
  
  cat config-template.json | \
    envsubst > "$ENV-config.json"
  
  mmctl --server "$MM_SITE_URL" config set PluginSettings.Plugins.com.mm.oidc --config "$ENV-config.json"
done
```

## Configuration Validation

### Pre-Deployment Validation

```bash
#!/bin/bash
# validate-config.sh

CONFIG_FILE=$1

# Check required fields
for field in issuer_url client_id client_secret redirect_url; do
  if ! jq -e ".$field" "$CONFIG_FILE" > /dev/null; then
    echo "ERROR: Missing required field: $field"
    exit 1
  fi
done

# Validate HTTPS in production
if [ "$ENV" = "production" ]; then
  if ! jq -r '.issuer_url' "$CONFIG_FILE" | grep -q "^https://"; then
    echo "ERROR: Production must use HTTPS issuer"
    exit 1
  fi
  
  if jq -r '.allow_insecure_issuer' "$CONFIG_FILE" | grep -q "true"; then
    echo "ERROR: allow_insecure_issuer must be false in production"
    exit 1
  fi
fi

echo "Configuration validation passed"
```

## Rollback Procedure

### Rollback to Previous Version

```bash
# 1. Disable current plugin
mmctl plugin disable com.mm.oidc

# 2. Restore backup configuration
mmctl config set PluginSettings.Plugins.com.mm.oidc --config backup-pre-upgrade.json

# 3. Upload previous plugin version
mmctl plugin add --force mm-oidc-v0.0.1.tar.gz

# 4. Enable plugin
mmctl plugin enable com.mm.oidc

# 5. Verify
curl https://chat.example.com/plugins/com.mm.oidc/health
```

## Best Practices

1. **Always backup before changes**
2. **Test migrations in staging first**
3. **Encrypt backups containing secrets**
4. **Version control configuration templates**
5. **Document environment-specific values**
6. **Automate validation checks**
7. **Plan and test rollback procedures**
8. **Monitor after migrations**

## Related Skills
- plugin-install
- troubleshooting
- security-audit
- release-management

## References
- [docs/USER_GUIDE.md](../../../docs/USER_GUIDE.md)
- [docs/DEVELOPER_GUIDE.md](../../../docs/DEVELOPER_GUIDE.md)
- [plugin.json](../../../plugin.json)
