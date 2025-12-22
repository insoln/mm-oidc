# Mattermost OIDC Plugin

Securely connect Mattermost Server v9.x+ to modern OpenID Connect providers (Keycloak v25.x by default) using the Authorization Code + PKCE flow. The plugin ships hardened state/nonce handling, encrypted refresh-token storage, and observability hooks so you can run SSO without upgrading to the Enterprise edition.

## What you get

- ✅ Full OIDC login flow with automatic user provisioning and optional admin promotion via client roles.
- ✅ Ready-made Docker stack (Mattermost + Keycloak + Postgres + proxy) for demos, QA, and CI.
- ✅ Proxy recipe that rewrites `/login` to the plugin route without breaking API clients.
- ✅ Playwright regression suite that mirrors the documented installation steps.

## Choose your deployment path

Use the [User Guide](docs/USER_GUIDE.md) for step-by-step instructions covering three supported scenarios:

1. **Existing Mattermost instances** – upload `mm-oidc.tar.gz`, configure the IdP client, and validate the flow from the System Console.
2. **Standalone stack** – run `./scripts/dev-up.sh` to launch the bundled compose environment, then explore the plugin landing page at `/plugins/com.mm.oidc/`.
3. **Proxy-assisted login** – drop the maintained Nginx container (or Helm/K8s manifests) in front of Mattermost so `/login` automatically redirects into the plugin without touching server code.

## Quick install checklist (prod environments)

1. Download the latest release artifact from [GitHub Releases](https://github.com/insoln/mm-oidc/releases).
2. Create a confidential OIDC client in your IdP with redirect `https://<mattermost>/plugins/com.mm.oidc/callback` and the standard profile/email mappers (see [docs/KEYCLOAK_SETUP.md](docs/KEYCLOAK_SETUP.md)).
3. Upload `mm-oidc.tar.gz` via **System Console → Plugin Management → Plugin Upload**.
4. Fill out the plugin settings (Issuer URL, Client ID/Secret, Scopes) in **System Console → Plugins → Mattermost OIDC**.
5. Point users to `/plugins/com.mm.oidc/login` or enable the proxy recipe so `/login` flows through the plugin automatically.
6. Run `./scripts/e2e-test.sh` to execute the Playwright suite and confirm the documented steps succeed end-to-end.

Details, screenshots, and troubleshooting tips for each step live in [docs/USER_GUIDE.md](docs/USER_GUIDE.md).

## Validation via Playwright

The repository includes browser tests that reproduce every flow described in the guide:

```bash
# Run against the bundled dev stack
./scripts/e2e-test.sh

# Include proxy-specific assertions (curl + Playwright)
./scripts/test-proxy-all.sh
```

These tests must remain green before promoting documentation updates to production.

## For developers & contributors

Looking for build, testing, or architecture details? Jump into the developer docs:

- Architecture & security notes – [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- Local dev stack + automation – [docs/DEV_ENV.md](docs/DEV_ENV.md)
- Playwright tips & CI integration – [docs/E2E_TESTING.md](docs/E2E_TESTING.md)
- Proxy internals and redirect research – [docs/PROXY_IMPLEMENTATION_SUMMARY.md](docs/PROXY_IMPLEMENTATION_SUMMARY.md)

Please keep user-facing instructions inside [docs/USER_GUIDE.md](docs/USER_GUIDE.md) up to date whenever you change plugin behavior or deployment requirements.
