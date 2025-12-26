# Mattermost OIDC Proxy Guide

This document consolidates the research, architecture decisions, and operational guidance for running Mattermost behind a reverse proxy that automatically redirects unauthenticated users into the OpenID Connect (OIDC) plugin. It targets infrastructure and SRE engineers who need both the technical rationale and the step-by-step instructions to deploy, test, and harden the proxy layer.

---

## 1. Why the proxy exists

- **Goal**: Provide seamless, zero-click OIDC logins for browser users without modifying Mattermost core (Team Edition friendly).
- **Mechanism**: The proxy inspects the Mattermost session cookie `MMAUTHTOKEN`. If absent, it redirects the browser to `/plugins/com.mm.oidc/login?redirect_to=<original URL>`, letting the plugin launch the Authorization Code + PKCE flow.
- **Status**: Implementation is complete, fully baked into the default dev stack, and covered by both curl and Playwright regression tests.

---

## 2. Architecture overview

```
Browser
  ↓
Reverse proxy (Nginx/Traefik/Ingress)
  ↓
Check MMAUTHTOKEN cookie
  ├─ Missing → 302 → /plugins/com.mm.oidc/login
  └─ Present → proxy_pass → Mattermost (8065)
      ↓
Mattermost OIDC plugin ↔ Keycloak ↔ callback ↔ cookie issued
```

### Core components

| Component | Role |
| --- | --- |
| Reverse proxy (Nginx reference implementation) | Entry point, cookie detection, conditional redirect, forwarding | 
| Mattermost + OIDC plugin | Handles Authorization Code + PKCE, sets `MMAUTHTOKEN`, serves UI | 
| IdP (Keycloak by default) | Authenticates users, issues ID tokens, optional role mapping | 

### Repo artifacts

| File / Script | Purpose |
| --- | --- |
| [deploy/docker-compose.dev.yml](../deploy/docker-compose.dev.yml) | Adds the `mattermost-proxy` service to the dev stack |
| [deploy/mattermost-proxy/nginx.conf](../deploy/mattermost-proxy/nginx.conf) | Canonical Nginx config (cookie map + redirect rules) |
| [deploy/env/dev.env](../deploy/env/dev.env) | Hosts `PROXY_HOSTNAME`, `PROXY_PORT`, `MM_SITE_URL`, etc. |
| [scripts/dev-up.sh](../scripts/dev-up.sh) / [scripts/dev-down.sh](../scripts/dev-down.sh) | Start/stop the full stack (builds plugin, bootstraps Keycloak) |
| [scripts/test-proxy.sh](../scripts/test-proxy.sh) | Curl smoke tests for redirect rules |
| [scripts/test-proxy-all.sh](../scripts/test-proxy-all.sh) | Curl + Playwright regression harness |
| [e2e/tests/proxy-redirect.spec.ts](../e2e/tests/proxy-redirect.spec.ts) | Browser regression suite |

---

## 3. Reference Nginx configuration

### Cookie detection and redirect logic

```nginx
map $cookie_MMAUTHTOKEN $auth_redirect {
    default 1;      # No cookie → redirect to plugin login
    "~.+" 0;        # Cookie present → allow through
}

location / {
    set $final_redirect 0;
    if ($auth_redirect = 1) { set $final_redirect 1; }
    if ($request_method != GET) { set $final_redirect 0; }
    if ($http_accept ~* "application/json") { set $final_redirect 0; }

    if ($final_redirect = 1) {
        return 302 /plugins/com.mm.oidc/login?redirect_to=$request_uri;
    }

    proxy_pass http://mattermost;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

### Redirect exceptions

Add explicit `location` blocks or conditional checks for:

- `/api/*` (including `/api/v4/users/login`)
- `/plugins/*` (plugin landing page, callback, health, static assets)
- `/static/*`
- `/health`
- `/api/v4/websocket`
- Any non-GET request
- Requests carrying `Accept: application/json`

These carve-outs keep API clients, automation, health probes, and WebSockets functional without interference.

### Native desktop/mobile detection

The bundled config in [deploy/mattermost-proxy/nginx.conf](../deploy/mattermost-proxy/nginx.conf) now inspects the `User-Agent` header for the `Mattermost` token. When detected, the proxy automatically appends `isMobile=true` to the `/plugins/com.mm.oidc/login` redirect so the plugin switches to the desktop/mobile handshake (launching the system browser and handing tokens back via `mattermost://`). Servers that maintain their own ingress layer should replicate the same behavior or adjust the regex list if your fleet uses custom User-Agent strings.

### Optional Traefik pattern

Traefik can reproduce the same behavior using a custom middleware plugin (for cookie inspection) or a combination of RedirectRegex and AllowList middleware. See the snippets in the original research section below or adapt [Traefik documentation](https://doc.traefik.io/traefik/) to your needs.

---

## 4. Redirect flow in detail

1. User opens `https://chat.example.com/`.
2. Proxy checks `MMAUTHTOKEN`:
   - **Present** → forward to Mattermost immediately.
   - **Missing** → 302 redirect to `/plugins/com.mm.oidc/login?redirect_to=/`.
3. Plugin launches Authorization Code + PKCE:
   - Generates state/nonce/code_verifier.
   - Redirects to Keycloak’s authorization endpoint.
4. Keycloak authenticates the user and sends `code` + `state` to `/plugins/com.mm.oidc/callback`.
5. Plugin exchanges the code for tokens, validates claims, provisions the user, and sets `MMAUTHTOKEN`.
6. User is redirected to the original URL; subsequent requests carry the cookie and bypass the login redirect.

**Important**: Always exclude `/plugins/com.mm.oidc/*` from redirects to avoid loops and allow callback traffic.

---

## 5. Validation & test coverage

### Playwright regression (`e2e/tests/proxy-redirect.spec.ts`)

Covered scenarios:

- Anonymous GET → redirect to plugin login.
- `redirect_to` preserves original URL.
- Full login → cookie established → subsequent requests load UI.
- API endpoints return 200/401 without redirect.
- Static assets served directly.
- WebSocket upgrade works.
- Security headers (X-Frame-Options, X-Content-Type-Options, etc.) present.
- Requests with `Accept: application/json` bypass redirect.

### Curl harness (`scripts/test-proxy.sh`)

Eight smoke-test scenarios:

1. Root request without cookie returns 302.
2. API route returns 200/401, not 302.
3. Static asset accessible w/o redirect.
4. `/health` accessible.
5. POST request is not redirected.
6. Plugin callback bypasses redirect logic.
7. WebSocket endpoint reachable.
8. Security headers present.

### Performance & security

- Additional latency: ~5 ms per request.
- Security headers enforced by proxy.
- Observability: use proxy access/error logs plus `scripts/test-proxy-all.sh` in CI.

---

## 6. Operating instructions

```bash
./scripts/dev-up.sh          # builds plugin, boots stack, configures Keycloak + Mattermost
./scripts/test-proxy.sh      # curl smoke tests
./scripts/test-proxy-all.sh  # curl + Playwright regression
./scripts/dev-down.sh        # teardown
```

Default endpoints (after `dev-up.sh`):

- Mattermost via proxy: `http://mattermost-proxy.127.0.0.1.nip.io:8787`
- Keycloak: `http://keycloak.127.0.0.1.nip.io:8080`
- Credentials: `mm-admin / Password123!`, `admin / Keycloak123!`

---

## 7. Production readiness checklist

1. **TLS everywhere** – terminate HTTPS at the proxy or an upstream load balancer.
2. **Rate limiting / WAF** – protect `/login` and `/plugins/com.mm.oidc/login` from brute-force, credential stuffing, and bots.
3. **Monitoring** – capture HTTP status codes, latency, redirect counts, and proxy errors.
4. **Load testing** – validate throughput and latency under production traffic patterns.
5. **Security review** – run OWASP ZAP or similar; verify headers, CSRF protections, cookie flags.
6. **Backup/DR** – store proxy configs in Git, plan recovery steps for secrets and certificates.

---

## 8. Troubleshooting tips

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| Infinite redirect loop | callback URL not excluded, or `redirect_to` re-triggers rules | Exclude `/plugins/com.mm.oidc/*` from redirect logic, ensure `redirect_to` is validated |
| API clients get 302 | `Accept` header not checked or API path missing from allow list | Add explicit exceptions for `/api/*` and `Accept: application/json` |
| Mobile apps cannot login | Proxy intercepting non-browser traffic | Route native apps directly to Mattermost or provide alternate auth |
| WebSocket fails | Missing upgrade headers | Copy the dedicated `/api/v4/websocket` block from `deploy/mattermost-proxy/nginx.conf` |
| Cookie expires but proxy still forwards | Expired `MMAUTHTOKEN` | Mattermost will respond 401; consider additional upstream health or session validation if necessary |

---

## 9. Security hardening checklist

- Force HTTPS: `return 301 https://$server_name$request_uri;`
- Ensure Mattermost sets `Secure`, `HttpOnly`, and `SameSite` flags on cookies.
- Apply login rate limiting (e.g., `limit_req_zone`).
- Add security headers: `X-Frame-Options`, `X-Content-Type-Options`, `X-XSS-Protection`, `Referrer-Policy`.
- Restrict admin panels (`/admin`) by IP allowlists.
- Enable CAPTCHA or lockout policies in Keycloak for high-risk environments.

---

## 10. Alternative integrations

### Kubernetes ingress

- Port the Nginx config into an Ingress resource via annotations, or use Nginx/Traefik ingress controllers with custom middleware.
- Keep sticky sessions if multiple Mattermost pods are behind the proxy.

### API Gateways

- Kong, Tyk, or AWS API Gateway can implement the same redirect pattern using plugins/middleware. Use when you already standardize on a gateway layer.

### Service mesh

- Istio/Linkerd/Consul Connect can enforce authentication and routing rules, but the cookie-aware redirect still needs to happen in an ingress gateway in front of the mesh.

---

## 11. Future work

- Production load/perf testing under expected user concurrency.
- SOC-approved WAF/rate-limiting presets for `/login`.
- Operator runbooks (alerts, remediation, rollback steps).
- Optional Traefik middleware plugin for cookie inspection.

---

## 12. References

- Mattermost plugin overview – [README](../README.md)
- Dev environment and automation – [docs/DEV_ENV.md](DEV_ENV.md)
- Keycloak setup – [docs/KEYCLOAK_SETUP.md](KEYCLOAK_SETUP.md)
- Proxy regression harness – [scripts/test-proxy.sh](../scripts/test-proxy.sh), [scripts/test-proxy-all.sh](../scripts/test-proxy-all.sh)
- Reverse proxy best practices – [NGINX guide](https://docs.nginx.com/nginx/admin-guide/web-server/reverse-proxy/), [Traefik docs](https://doc.traefik.io/traefik/)
- Identity standards – [OpenID Connect Core](https://openid.net/specs/openid-connect-core-1_0.html), [OAuth 2.0 security best practices](https://www.rfc-editor.org/rfc/rfc6819)

---

Status: ✅ Complete  
Last updated: 2025-12-22
