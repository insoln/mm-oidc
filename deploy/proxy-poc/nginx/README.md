# NGINX Proxy POC for OIDC Redirect

This is a proof-of-concept implementation of automatic OIDC redirect using NGINX as a reverse proxy for Mattermost.

## Overview

This POC demonstrates how to automatically redirect unauthenticated users to the OIDC login page using NGINX as a reverse proxy, without modifying Mattermost core or the OIDC plugin.

## Architecture

```
Browser → NGINX (port 8787) → Mattermost (internal:8065)
                ↓
          Cookie Check
                ↓
    No MMAUTHTOKEN? → Redirect to /plugins/com.mm.oidc/login
    Has MMAUTHTOKEN? → Proxy to Mattermost
```

## Features

- ✅ Automatic redirect to OIDC for unauthenticated users
- ✅ Cookie-based authentication check (`MMAUTHTOKEN`)
- ✅ API endpoints bypass redirect (return 401 instead)
- ✅ WebSocket support without redirect
- ✅ Static files served without redirect
- ✅ Plugin endpoints (including callback) bypass redirect
- ✅ Security headers configured
- ✅ Comprehensive logging

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Make (for building the plugin)
- Port 8787 (proxy) and 8080 (Keycloak) available

### Start the POC

```bash
./start.sh
```

This will:
1. Build the OIDC plugin
2. Start all services (NGINX, Mattermost, Keycloak, databases)
3. Bootstrap Mattermost and Keycloak
4. Configure the OIDC plugin

### Test the Redirect

```bash
# Run automated curl tests
./test-curl.sh

# Or test manually
curl -v http://proxy.127.0.0.1.nip.io:8787/ 2>&1 | grep Location
# Should redirect to /plugins/com.mm.oidc/login
```

### Access Points

- **Mattermost (via NGINX)**: http://proxy.127.0.0.1.nip.io:8787
- **Keycloak (direct)**: http://keycloak.127.0.0.1.nip.io:8080
- **NGINX logs**: `docker compose logs -f nginx`

### Stop the POC

```bash
./stop.sh
```

## How It Works

### Cookie Detection

NGINX checks for the `MMAUTHTOKEN` cookie using the `map` directive:

```nginx
map $cookie_MMAUTHTOKEN $auth_redirect {
    default 1;      # No cookie = redirect
    "~.+" 0;        # Has cookie = don't redirect
}
```

### Redirect Logic

For the root location (`/`), NGINX:

1. Checks if `MMAUTHTOKEN` cookie exists
2. Checks if it's a GET request (don't redirect POST/PUT/DELETE)
3. Checks if it's not an API client (`Accept: application/json`)
4. If all conditions met → redirect to `/plugins/com.mm.oidc/login?redirect_to=$request_uri`
5. Otherwise → proxy to Mattermost

### Exclusions

These paths bypass the redirect logic:

- `/api/*` - API endpoints (return 401 instead)
- `/plugins/*` - Plugin endpoints (including callback)
- `/static/*` - Static files
- `/health` - Health checks
- `/api/v4/websocket` - WebSocket connections

## Testing

### Automated Tests (curl)

```bash
./test-curl.sh
```

This runs 7 test scenarios:
1. ✅ Root URL redirects unauthenticated users
2. ✅ API endpoints don't redirect
3. ✅ Plugin endpoints accessible
4. ✅ Static files accessible
5. ✅ Health checks work
6. ✅ POST requests don't redirect
7. ⚠️ Authenticated requests (manual test)

### Manual Testing

1. **Test redirect without cookie**:
   ```bash
   curl -v http://proxy.127.0.0.1.nip.io:8787/
   # Should see: Location: /plugins/com.mm.oidc/login?redirect_to=/
   ```

2. **Test API doesn't redirect**:
   ```bash
   curl -v http://proxy.127.0.0.1.nip.io:8787/api/v4/system/ping
   # Should see: 200 OK (not redirect)
   ```

3. **Test with browser**:
   - Open http://proxy.127.0.0.1.nip.io:8787 in browser
   - Should automatically redirect to OIDC login
   - After login, should have normal access

### E2E Tests (Playwright)

E2E tests for the proxy redirect are located in `e2e/tests/proxy-redirect.spec.ts`.

Run them with:
```bash
cd ../../../e2e
yarn test proxy-redirect
```

## Configuration

### Environment Variables

Edit `.env` to customize:

```bash
# Proxy port (exposed via nip.io hostname)
PROXY_PORT=8787
PROXY_HOSTNAME=proxy.127.0.0.1.nip.io

# Mattermost configuration
MM_SITE_URL=http://proxy.127.0.0.1.nip.io:8787
MM_ADMIN_USERNAME=mm-admin
MM_ADMIN_PASSWORD=Password123!

# Keycloak configuration  
KC_ADMIN=admin
KC_ADMIN_PASSWORD=Keycloak123!
# KC_HOSTNAME=keycloak.127.0.0.1.nip.io
```

### NGINX Configuration

The main configuration is in `nginx.conf`. Key sections:

- **Cookie mapping**: `map $cookie_MMAUTHTOKEN $auth_redirect`
- **Redirect logic**: `location /` with conditional redirect
- **WebSocket support**: `location /api/v4/websocket`
- **API bypass**: `location /api/`
- **Security headers**: `add_header` directives

## Security Considerations

### Implemented

- ✅ Security headers (X-Frame-Options, X-Content-Type-Options, etc.)
- ✅ API endpoints don't redirect (prevent token leakage)
- ✅ WebSocket connections preserved
- ✅ POST/PUT/DELETE requests not redirected
- ✅ Detailed logging for audit

### Recommendations for Production

- [ ] Use HTTPS with valid certificates
- [ ] Configure rate limiting
- [ ] Add IP whitelisting for admin paths
- [ ] Set up monitoring and alerts
- [ ] Enable access logging to SIEM
- [ ] Configure session timeout
- [ ] Add WAF rules

## Troubleshooting

### Redirect loop

**Problem**: Browser keeps redirecting.

**Solution**: Check that `/plugins/com.mm.oidc/*` is excluded from redirect logic.

### API calls fail

**Problem**: Mobile app or API clients can't authenticate.

**Solution**: Ensure API endpoints bypass redirect and return 401.

### WebSocket disconnects

**Problem**: Real-time features don't work.

**Solution**: Verify WebSocket upgrade is not redirected.

### Logs

View NGINX logs:
```bash
docker compose logs -f nginx
docker compose exec nginx cat /var/log/nginx/access.log
docker compose exec nginx cat /var/log/nginx/error.log
```

## Limitations

- Requires running NGINX in front of Mattermost
- Adds latency (minimal, but measurable)
- More complex troubleshooting
- Mobile apps need special handling
- API-only clients must bypass proxy or use tokens

## Next Steps

1. Test with real load
2. Benchmark performance impact
3. Add Prometheus metrics export
4. Create Kubernetes Ingress version
5. Document production deployment

## References

- [Main documentation](../../../docs/PROXY_OIDC_REDIRECT.md)
- [NGINX reverse proxy guide](https://docs.nginx.com/nginx/admin-guide/web-server/reverse-proxy/)
- [Mattermost OIDC plugin](../../../README.md)
