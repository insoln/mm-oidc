# Keycloak Setup Guide

This document walks you through preparing a Keycloak 25.x realm and client for the Mattermost OIDC plugin. Follow the steps below before configuring Mattermost so the plugin has a trusted issuer, redirect URI, and all required claims.

## Prerequisites

- Keycloak 25.x+ (modern admin console)
- Administrator account with permission to create realms, clients, roles, and users.
- Target Mattermost site URL (for example `https://chat.example.com`).
- Redirect endpoint determined during plugin installation: `https://<mattermost-host>/plugins/com.mm.oidc/callback`.

## 1. Create or choose a realm

1. Sign in to the Keycloak Admin Console: `https://<keycloak-host>/admin`.
2. Open the realm selector (top left) and click **Create realm**.
3. Provide a name (e.g., `mattermost`) and click **Create**. You can use `master`, but a dedicated realm isolates users and roles more cleanly.

## 2. Create the confidential client

1. In the left navigation, select **Clients → Create client**.
2. Fill in:
   - **Client type**: *OpenID Connect*.
   - **Client ID**: choose `mm-oidc` or another memorable identifier.
   - Click **Next**.
3. On **Capability config** enable:
   - ✅ **Client authentication**
   - ✅ **Authorization** (optional; leave disabled unless you need fine-grained permissions)
   - ✅ **Standard flow**
   - ❌ **Implicit flow** (disable)
   - ❌ **Direct access grants** (disable)
   - ❌ **Service accounts** (disable)
   - Click **Save**.
4. On **Login settings** set:
   - **Root URL**: `https://<mattermost-host>`
   - **Valid redirect URIs**: `https://<mattermost-host>/plugins/com.mm.oidc/callback`
   - **Web origins**: `https://<mattermost-host>` (add staging URLs if needed)
   - **Post logout redirect URIs**: optional; typically the Mattermost home page
   - Click **Save**.
5. Open the **Credentials** tab and copy the generated **Client secret**. You will paste it into Mattermost later.

## 3. Add protocol mappers

Still under the client, open **Client scopes → Add mapper → By configuration**. Create the following:

| Mapper name        | Type            | Source property | Token claim             | Include in |
|--------------------|-----------------|-----------------|-------------------------|------------|
| `preferred_username` | User Property   | `username`      | `preferred_username`    | ID, Access, UserInfo |
| `given_name`       | User Property   | `firstName`     | `given_name`            | ID, Access, UserInfo |
| `family_name`      | User Property   | `lastName`      | `family_name`           | ID, Access, UserInfo |
| `full_name`        | Full name       | —               | `name`                  | ID, Access, UserInfo |
| `email`            | User Property   | `email`         | `email`                 | ID, Access, UserInfo |
| `mm-oidc-client-roles` (optional) | Client roles | Roles of this client | `resource_access.<client_id>.roles` | Access |

> The optional roles mapper lets the plugin promote users to Mattermost System Admins when a specific client role is present.

## 4. Create a System Admin client role (optional)

1. Inside the client, open **Roles → Create role**.
2. Set **Role name** to `system_admin` (or another name you will map inside Mattermost).
3. Save the role.
4. Assign it to privileged users: **Users → Select user → Role mapping → Assign role → Client Roles → <client> → system_admin**.

## 5. Verify account attributes

- For each user that will log in, ensure **Email** is set and **Email verified** is toggled on; the plugin refuses to provision users without verified email addresses.
- Optionally set **First name** and **Last name** so Mattermost receives profile data on first login.

## 6. Record values for Mattermost

When you configure the plugin, you will need:

- **Issuer URL**: `https://<keycloak-host>/realms/<realm>`
- **Client ID**: the identifier you chose (e.g., `mm-oidc`)
- **Client secret**: from the Credentials tab
- **Scopes**: `openid profile email` (add `roles` if you created the mapper)

Store these in a secure secret manager until you enter them into the Mattermost System Console.

## 7. Automating via scripts/dev-bootstrap.sh

The repository’s dev tooling can provision the entire realm+client automatically:

```bash
./scripts/dev-up.sh   # runs scripts/dev-bootstrap.sh under the hood
```

`scripts/dev-bootstrap.sh` uses `kcadm` to:

- Wait for Keycloak to become ready (per `deploy/env/dev.env`).
- Create the realm, client, protocol mappers, and client role if they do not exist.
- Grant the role to the admin account and sync the client secret into Mattermost.

Review the script for command-by-command details if you need to replicate the automation in your own infrastructure-as-code.

## 8. Validate with Playwright

Once Keycloak is configured (manually or via automation), run the Playwright flow to confirm the login succeeds end-to-end:

```bash
./scripts/e2e-test.sh
```

The test suite launches the OIDC flow, signs in through Keycloak, and verifies that Mattermost receives the correct cookies. Keep the documentation and the automated tests in sync so they describe the same sequence of actions.

## 9. Troubleshooting tips

- **Invalid redirect URI** – double-check the exact scheme/host/port in both the Keycloak client and Mattermost’s plugin settings.
- **Missing claims** – ensure every mapper is set to include values in the ID token; the plugin logs which claims are missing at the `DEBUG` level.
- **Role not honored** – confirm the brokered claim name (`resource_access.<client_id>.roles`) matches what you configured in Mattermost’s admin role mapping.
- **HTTP issuer during development** – set `OIDC_ALLOW_INSECURE=true` in Mattermost only for local testing; production deployments must use HTTPS everywhere.

Follow these steps each time you spin up a new Keycloak environment (dev, staging, prod) to keep your IdP configuration consistent across instances.
