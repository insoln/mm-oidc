---
name: user-provisioning
description: Configure and manage user provisioning, profile synchronization, and role mapping for the Mattermost OIDC plugin. Use this skill when setting up user management policies, configuring role assignments, or migrating existing users to OIDC.
license: Apache-2.0
metadata:
  author: insoln
  version: "1.0"
  category: configuration
  tags: [users, provisioning, roles, profile-sync, migration]
---

# User Provisioning Skill

## Summary
This skill covers user provisioning strategies, claim mapping, role assignments, and user migration workflows for the Mattermost OIDC plugin.

## When to Use This Skill
- Setting up automatic user provisioning
- Configuring role-based access control
- Mapping OIDC claims to Mattermost user fields
- Migrating existing users from password to OIDC
- Troubleshooting user creation or update issues

## Provisioning Modes

### Automatic Provisioning
Users are automatically created on first OIDC login.

**Configuration:**
- Plugin creates new Mattermost users automatically
- User data populated from ID token claims
- Requires: email, username, email_verified

### Manual Provisioning
Users must be invited or pre-created before OIDC login.

**Configuration:**
- Disable auto-provisioning in plugin settings (if supported)
- Link OIDC identity to existing users
- Useful for controlled environments

## Claim Mapping

### Required Claims

**email**
- Claim name: `email`
- Source: User email attribute in Keycloak
- Must be present and verified
- Used as Mattermost user email

**preferred_username**
- Claim name: `preferred_username`
- Source: User username in Keycloak
- Used as Mattermost username
- Must be unique across instance

**email_verified**
- Claim name: `email_verified`
- Must be `true` for provisioning
- Set in Keycloak user attributes

### Optional Claims

**given_name / family_name**
- Used for display name
- Combined into full name

**name**
- Full display name
- Alternative to given_name/family_name

**picture**
- Profile picture URL
- Fetched and stored by plugin (if implemented)

## Role Mapping

### System Admin Promotion

**Configuration in Keycloak:**
1. Create client role `system_admin`
2. Add protocol mapper for client roles
3. Assign role to specific users

**Protocol Mapper:**
- Type: User Client Role
- Client: mm-oidc
- Claim name: `resource_access.mm-oidc.roles`
- Include in ID token

**Plugin Behavior:**
- Reads roles from ID token
- Promotes user to Mattermost system_admin if role present
- Demotes if role removed (configurable)

### Team/Channel Assignment

**Not implemented in current version**
- Future enhancement
- Could map OIDC groups to Mattermost teams
- Requires additional claim mapping

## User Migration

### From Password to OIDC

**Automatic Linking:**
The plugin supports automatic user conversion:
1. User exists with email/password
2. User logs in via OIDC with matching email
3. Plugin links OIDC subject to existing user
4. User profile updated from OIDC claims
5. Future logins use OIDC automatically

**Manual Steps:**
1. Ensure email addresses match in both systems
2. Communicate migration to users
3. Users login via OIDC for first time
4. Existing accounts preserved

### Bulk Migration

```bash
# Export existing users
mmctl user list --all > users.txt

# For each user, ensure matching Keycloak account exists
# Users will be linked on first OIDC login

# Optional: Force password reset to encourage OIDC adoption
mmctl user reset-password <username>
```

## Provisioning Examples

### New User Creation

**OIDC Flow:**
1. User authenticates with Keycloak
2. Plugin receives ID token with claims
3. Plugin checks if user exists by OIDC subject
4. If not found, checks by email
5. If no match, creates new Mattermost user
6. Populates profile from claims
7. Assigns default team (if configured)
8. Stores OIDC subject mapping

**Required Claims in ID Token:**
```json
{
  "sub": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "email": "user@example.com",
  "email_verified": true,
  "preferred_username": "jsmith",
  "given_name": "John",
  "family_name": "Smith",
  "name": "John Smith"
}
```

### Admin Promotion

**ID Token with Role:**
```json
{
  "sub": "...",
  "email": "admin@example.com",
  "resource_access": {
    "mm-oidc": {
      "roles": ["system_admin"]
    }
  }
}
```

**Plugin Action:**
- Detects `system_admin` in roles
- Promotes user to Mattermost system_admin
- Grants access to System Console

## Troubleshooting

### User Not Created

**Problem:** User can't login, no account created

**Debug:**
```bash
# Check plugin logs
grep "provision" mattermost.log

# Verify required claims present
jwt decode <id_token>
```

**Solution:**
1. Ensure email and preferred_username present
2. Check email_verified is true
3. Verify no special characters in username
4. Check for duplicate email/username conflicts

### Profile Not Updated

**Problem:** User profile doesn't reflect OIDC claims

**Debug:**
```bash
# Check user profile
mmctl user show <username>

# Verify claims in ID token
jwt decode <id_token>
```

**Solution:**
- Plugin may not update on every login
- Check if profile update logic is enabled
- Manually update if needed: `mmctl user update`

### Admin Role Not Assigned

**Problem:** User should be admin but isn't

**Debug:**
```bash
# Check user roles
mmctl user show <username> | grep -i role

# Verify role claim in token
jwt decode <id_token> | grep roles
```

**Solution:**
1. Verify system_admin role assigned in Keycloak
2. Ensure roles scope requested by plugin
3. Check protocol mapper configured correctly
4. Restart Mattermost if needed

## Best Practices

1. **Email Verification Required** - Always enforce verified emails
2. **Unique Usernames** - Handle conflicts with existing users
3. **Profile Sync** - Keep OIDC claims as source of truth
4. **Role Mapping** - Use OIDC roles for Mattermost permissions
5. **Audit Logging** - Track user creation and updates
6. **Graceful Fallback** - Support password login during migration
7. **Communication** - Notify users about authentication changes

## Related Skills
- keycloak-setup
- plugin-install
- troubleshooting
- security-audit

## References
- [docs/ARCHITECTURE.md](../../../docs/ARCHITECTURE.md)
- [docs/KEYCLOAK_SETUP.md](../../../docs/KEYCLOAK_SETUP.md)
