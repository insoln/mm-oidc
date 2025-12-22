# Testing Guide for NGINX Proxy OIDC Redirect

This document describes how to test the NGINX proxy OIDC redirect functionality.

## Test Levels

### 1. Configuration Validation (No services needed)

Validates NGINX configuration syntax:

```bash
./validate-config.sh
```

**What it tests:**
- NGINX configuration syntax
- Proper directive usage
- Valid upstream configuration

**Expected result:**
```
nginx: configuration file /etc/nginx/nginx.conf test is successful
```

### 2. Static Analysis (No services needed)

Manual inspection of the configuration:

```bash
cat nginx.conf | grep -A 5 "map \$cookie_MMAUTHTOKEN"
```

**What to verify:**
- Cookie detection logic is correct
- Redirect URLs are properly formatted
- Exclusion paths are comprehensive
- Security headers are present

### 3. Curl Tests (Requires running services)

Automated HTTP tests using curl:

```bash
# Start the stack first
./start.sh

# Run curl tests
./test-curl.sh
```

**Test scenarios:**
1. ✅ Root URL redirects unauthenticated users
2. ✅ Redirect includes original URL in query param
3. ✅ API endpoints return 401, not redirect
4. ✅ Plugin endpoints accessible
5. ✅ Static files accessible
6. ✅ Health check works
7. ✅ POST requests don't redirect

### 4. Playwright E2E Tests (Requires running services)

Browser-based end-to-end tests:

```bash
# Start the stack first
./start.sh

# Run all tests (curl + Playwright)
./test-all.sh

# Or run Playwright tests directly
cd ../../../e2e
yarn playwright test proxy-redirect.spec.ts
```

**Test scenarios:**
1. ✅ Unauthenticated user redirect
2. ✅ Redirect with query parameters preserved
3. ✅ Full OIDC login flow
4. ✅ Authenticated access after login
5. ✅ API endpoint behavior
6. ✅ Security headers present
7. ✅ JSON Accept header bypass

## Manual Testing Checklist

### Prerequisites
- [ ] Docker and Docker Compose installed
- [ ] Ports 80 and 8080 available
- [ ] Plugin built (`make package`)

### Setup
1. [ ] Navigate to `deploy/proxy-poc/nginx/`
2. [ ] Copy `.env.example` to `.env` (if needed)
3. [ ] Run `./start.sh`
4. [ ] Wait for services to be ready (~1-2 minutes)

### Test Cases

#### TC1: Redirect without authentication
```bash
# Open browser in private/incognito mode
# Navigate to: http://localhost/
# Expected: Redirect to /plugins/com.mm.oidc/login
```

#### TC2: Direct URL access
```bash
# Navigate to: http://localhost/channels/town-square
# Expected: Redirect to /plugins/com.mm.oidc/login?redirect_to=/channels/town-square
```

#### TC3: Full login flow
```bash
# Navigate to: http://localhost/
# Click "Start Login"
# Enter Keycloak credentials (admin / Keycloak123!)
# Expected: Redirect back to Mattermost with MMAUTHTOKEN cookie
# Expected: Normal Mattermost UI visible
```

#### TC4: API endpoint behavior
```bash
curl -v http://localhost/api/v4/system/ping
# Expected: HTTP 200 OK (not 302 redirect)
```

#### TC5: API auth failure
```bash
curl -v http://localhost/api/v4/users/me
# Expected: HTTP 401 Unauthorized (not 302 redirect)
```

#### TC6: Plugin endpoints accessible
```bash
curl -v http://localhost/plugins/com.mm.oidc/health
# Expected: HTTP 200 with JSON response (not redirect)
```

#### TC7: Static files accessible
```bash
curl -v http://localhost/static/
# Expected: HTTP response (not 302 to OIDC)
```

#### TC8: Health check
```bash
curl -v http://localhost/health
# Expected: HTTP 200 OK
```

#### TC9: POST request not redirected
```bash
curl -v -X POST http://localhost/api/v4/users/login \
  -H "Content-Type: application/json" \
  -d '{"login_id":"test","password":"test"}'
# Expected: HTTP 401 (not 302 redirect)
```

#### TC10: Authenticated requests
```bash
# After logging in via browser, get MMAUTHTOKEN from cookies
# Test with curl:
curl -v -b "MMAUTHTOKEN=<your_token>" http://localhost/
# Expected: HTTP 200 with Mattermost HTML (not redirect)
```

#### TC11: WebSocket connection
```bash
# Open browser DevTools → Network tab → Filter WS
# Navigate to Mattermost after login
# Expected: WebSocket connection to /api/v4/websocket established
# Expected: No redirect errors
```

#### TC12: Security headers
```bash
curl -v http://localhost/plugins/com.mm.oidc/health 2>&1 | grep -i "x-frame"
# Expected: X-Frame-Options header present
```

### Cleanup
- [ ] Run `./stop.sh`
- [ ] Verify containers stopped: `docker compose ps`
- [ ] (Optional) Remove volumes: `docker compose down -v`

## Troubleshooting Test Failures

### Redirect loop
**Symptom:** Browser shows "Too many redirects"

**Possible causes:**
1. Plugin endpoints included in redirect logic
2. Callback URL redirecting
3. Cookie not being set properly

**Debug:**
```bash
# Check NGINX logs
docker compose logs nginx | tail -50

# Check if callback is excluded
grep -A 5 "location /plugins/" nginx.conf
```

### API tests fail with 302
**Symptom:** API endpoints redirect instead of returning 401

**Possible causes:**
1. `/api/` path not excluded properly
2. Redirect logic too broad

**Debug:**
```bash
# Test API directly
curl -v http://localhost/api/v4/system/ping 2>&1 | grep -E "HTTP|Location"

# Check NGINX config
grep -A 10 "location /api/" nginx.conf
```

### Services won't start
**Symptom:** `start.sh` fails or times out

**Possible causes:**
1. Ports already in use
2. Docker resources insufficient
3. Plugin build failed

**Debug:**
```bash
# Check ports
lsof -i :80 -i :8080

# Check Docker
docker compose ps
docker compose logs

# Rebuild plugin
cd ../../../
make package
```

### E2E tests fail
**Symptom:** Playwright tests timeout or fail

**Possible causes:**
1. Services not fully started
2. Network issues
3. Timing issues

**Debug:**
```bash
# Verify services are healthy
docker compose ps
curl http://localhost/health
curl http://localhost:8080/health/ready

# Run with headed browser to see what's happening
cd ../../../e2e
yarn playwright test proxy-redirect.spec.ts --headed

# Check Playwright trace
yarn playwright show-report
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Test Proxy POC

on: [push, pull_request]

jobs:
  test-proxy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Build plugin
        run: make package
      
      - name: Validate NGINX config
        run: |
          cd deploy/proxy-poc/nginx
          ./validate-config.sh
      
      - name: Start services
        run: |
          cd deploy/proxy-poc/nginx
          ./start.sh
      
      - name: Run curl tests
        run: |
          cd deploy/proxy-poc/nginx
          ./test-curl.sh
      
      - name: Install Playwright
        run: |
          cd e2e
          yarn install
          yarn playwright install --with-deps
      
      - name: Run E2E tests
        run: |
          cd e2e
          yarn playwright test proxy-redirect.spec.ts
      
      - name: Upload test results
        if: always()
        uses: actions/upload-artifact@v3
        with:
          name: playwright-report
          path: e2e/playwright-report/
      
      - name: Stop services
        if: always()
        run: |
          cd deploy/proxy-poc/nginx
          ./stop.sh
```

## Performance Testing

### Load Testing with Apache Bench

```bash
# Test redirect performance (no auth cookie)
ab -n 1000 -c 10 http://localhost/

# Test authenticated requests (with cookie)
ab -n 1000 -c 10 -C "MMAUTHTOKEN=your_token" http://localhost/

# Test API endpoints
ab -n 1000 -c 10 http://localhost/api/v4/system/ping
```

### Expected Results
- Redirects: < 10ms latency
- Proxied requests: < 50ms latency
- API endpoints: < 100ms latency

## Test Coverage Summary

| Test Category | Tests | Coverage |
|--------------|-------|----------|
| Config Validation | 1 | Syntax |
| Curl Tests | 7 | HTTP behavior |
| Playwright E2E | 12 | Browser flows |
| Manual Tests | 12 | User scenarios |
| **Total** | **32** | **Full stack** |

## Next Steps

1. **Automated Testing**: Integrate all tests into CI/CD
2. **Load Testing**: Benchmark under realistic traffic
3. **Security Testing**: Run OWASP ZAP or similar
4. **Chaos Testing**: Test failure scenarios
5. **Multi-browser**: Test on Safari, Firefox, Edge
