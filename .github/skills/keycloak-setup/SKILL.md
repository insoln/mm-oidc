---
name: keycloak-setup
description: Configures Keycloak realm, OIDC client, protocol mappers, and roles for integration with the Mattermost OIDC plugin. Use this skill when setting up a new Keycloak instance, configuring an existing realm, or troubleshooting IdP configuration issues.
license: Apache-2.0
metadata:
  author: insoln
  version: "1.0"
  category: configuration
  tags: [keycloak, oidc, idp, authentication, setup]
---

# Keycloak Setup Skill

## Summary
This skill provides guidance for configuring Keycloak 25.x+ as an OpenID Connect identity provider for the Mattermost OIDC plugin.

## When to Use This Skill
- Setting up new Keycloak instance for Mattermost
- Configuring existing realm for OIDC
- Adding protocol mappers
- Setting up role mapping
- Troubleshooting IdP configuration
- Automating setup via scripts

## Prerequisites
- Keycloak 25.x+ with admin access
- Target Mattermost site URL
- Plugin callback URL: `https://<mattermost>/plugins/com.mm.oidc/callback`

## Manual Configuration

### 1. Create Realm
1. Sign in to Keycloak Admin Console
2. Click realm selector → **Create realm**
3. Name it (e.g., `mattermost`)
4. Click **Create**

### 2. Create OIDC Client
1. Navigate to **Clients** → **Create client**
2. Set **Client ID** (e.g., `mm-oidc`)
3. Enable **Client authentication** and **Standard flow**
4. Disable **Implicit flow** and **Direct access grants**
5. Configure redirect URIs: `https://<mattermost>/plugins/com.mm.oidc/callback`
6. Save and copy **Client secret** from Credentials tab

### 3. Add Protocol Mappers
Required mappers for user provisioning:

| Mapper | Type | User Property | Claim Name |
|--------|------|---------------|------------|
| preferred_username | User Property | username | preferred_username |
| email | User Property | email | email |
| given_name | User Property | firstName | given_name |
| family_name | User Property | lastName | family_name |
| full_name | Full name | — | name |

### 4. Create System Admin Role (Optional)
1. **Clients** → **mm-oidc** → **Roles** → **Create role**
2. Name: `system_admin`
3. Assign to users for Mattermost admin promotion

### 5. Verify User Attributes
- Ensure **Email** is set and **Email verified** is enabled
- Set **First name** and **Last name** for profile data

### 6. Record Values
Collect for Mattermost configuration:
- Issuer URL: `https://<keycloak>/realms/<realm>`
- Client ID and Secret
- Scopes: `openid profile email` (add `roles` if needed)

## Automated Setup

```bash
# Using dev-bootstrap script
./scripts/dev-up.sh

# Or manually
./scripts/dev-bootstrap.sh
```

The script automatically:
- Creates realm and client
- Adds protocol mappers
- Creates system_admin role
- Configures Mattermost plugin

## Validation

```bash
# Test discovery endpoint
curl https://<keycloak>/realms/<realm>/.well-known/openid-configuration

# Run E2E tests
./scripts/e2e-test.sh
```

## Troubleshooting

### Invalid Redirect URI
- Verify exact match between Keycloak and Mattermost
- Check http/https, trailing slashes, ports

### Missing Claims
- Check protocol mappers are added
- Verify mappers include claims in ID token
- Ensure user attributes are populated

### Role Mapping Not Working
- Verify system_admin role exists
- Check user has role assigned
- Include `roles` scope in plugin config

## Related Skills
- dev-environment
- plugin-install
- oidc-flow-test
- troubleshooting

## References
- [docs/KEYCLOAK_SETUP.md](../../../docs/KEYCLOAK_SETUP.md)
- [scripts/dev-bootstrap.sh](../../../scripts/dev-bootstrap.sh)
- [Keycloak Documentation](https://www.keycloak.org/documentation)
