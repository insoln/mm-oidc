---
name: oidc-flow-test
description: Test the complete OpenID Connect authentication flow including Authorization Code + PKCE, token exchange, user provisioning, and session management. Use this skill when validating OIDC integration, debugging authentication issues, or running regression tests.
license: Apache-2.0
metadata:
  author: insoln
  version: "1.0"
  category: testing
  tags: [oidc, authentication, testing, e2e, integration]
---

# OIDC Flow Test Skill

## Summary
This skill provides comprehensive guidance for testing the OpenID Connect authentication flow in the Mattermost OIDC plugin, including manual testing procedures, automated test execution, and debugging techniques.

## When to Use This Skill
- Validating new OIDC integration
- Testing after configuration changes
- Debugging authentication failures
- Running regression tests
- Verifying token exchange and user provisioning
- CI/CD pipeline integration

## Prerequisites
- Running Mattermost instance with OIDC plugin installed
- Configured OIDC provider (Keycloak)
- Test user accounts in OIDC provider
- For automated tests: Playwright installed

## OIDC Flow Overview

The complete flow consists of:
1. **Initiation**: User clicks login, plugin generates state/nonce/PKCE challenge
2. **Authorization**: Redirect to IdP with OIDC parameters
3. **Authentication**: User authenticates with IdP
4. **Callback**: IdP redirects back with authorization code
5. **Token Exchange**: Plugin exchanges code for tokens using PKCE verifier
6. **Validation**: Plugin validates ID token and claims
7. **Provisioning**: Plugin creates or updates Mattermost user
8. **Session**: Plugin establishes Mattermost session with cookie

## Manual Testing

### Basic Login Flow

#### Step 1: Initiate Login
```bash
# Navigate to plugin landing page
open https://chat.example.com/plugins/com.mm.oidc/

# Or direct to login endpoint
open https://chat.example.com/plugins/com.mm.oidc/login
```

Expected behavior:
- Plugin landing page displays with "Start Login" button
- Shows current configuration (issuer, redirect URL)

#### Step 2: Start Authorization
Click **Start Login** button

Expected behavior:
- Redirect to Keycloak authorization endpoint
- URL contains OIDC parameters:
  - `client_id`: Plugin client ID
  - `redirect_uri`: Plugin callback URL
  - `response_type=code`: Authorization Code flow
  - `scope`: Requested scopes (openid profile email)
  - `state`: Random state parameter
  - `nonce`: Random nonce for replay protection
  - `code_challenge`: PKCE challenge
  - `code_challenge_method=S256`: SHA-256 PKCE

#### Step 3: Authenticate
Enter Keycloak credentials and submit

Expected behavior:
- Keycloak validates credentials
- Displays consent screen if required
- Redirects back to Mattermost callback

#### Step 4: Callback Processing
Automatic redirect to plugin callback URL

Expected behavior:
- Plugin receives authorization code and state
- Plugin validates state matches session
- Plugin exchanges code for tokens (with PKCE verifier)
- Plugin validates ID token signature and claims
- Plugin provisions or updates Mattermost user
- Plugin creates Mattermost session
- Sets `MMAUTHTOKEN` cookie
- Redirects to original URL or home page

#### Step 5: Verify Session
Check Mattermost UI

Expected behavior:
- User is logged in
- Profile displays correct information (name, email)
- User can access Mattermost features

### Testing Different Scenarios

#### Test New User Provisioning
```bash
# Create new user in Keycloak
# Ensure email is verified
# Login with new credentials
# Verify new Mattermost account created
```

#### Test Existing User Login
```bash
# Use existing Mattermost user
# Login via OIDC
# Verify profile updated from IdP
```

#### Test Admin Role Promotion
```bash
# Assign system_admin role in Keycloak
# Login via OIDC
# Verify user promoted to Mattermost system admin
# Check System Console access
```

#### Test Invalid Credentials
```bash
# Enter wrong password in Keycloak
# Verify error displayed
# Verify no Mattermost session created
```

#### Test State Validation
```bash
# Attempt to replay callback request
# Verify plugin rejects with invalid state error
```

## Automated Testing

### Running E2E Tests

```bash
# Run all OIDC flow tests
./scripts/e2e-test.sh

# Run specific test file
cd e2e
yarn test tests/oidc-flow.spec.ts

# Run in headed mode (visible browser)
yarn test:headed tests/oidc-flow.spec.ts

# Debug mode with Playwright Inspector
yarn test:debug tests/oidc-flow.spec.ts
```

### Test Suites

#### Complete Login Flow Test
```typescript
// tests/oidc-flow.spec.ts
test('should complete full OIDC login flow', async ({ page }) => {
  // 1. Navigate to plugin landing page
  await page.goto('/plugins/com.mm.oidc/');
  
  // 2. Click "Start Login"
  await page.click('text=Start Login');
  
  // 3. Wait for Keycloak redirect
  await page.waitForURL(/keycloak/);
  
  // 4. Fill in credentials
  await page.fill('input[name="username"]', 'admin');
  await page.fill('input[name="password"]', 'Keycloak123!');
  await page.click('input[type="submit"]');
  
  // 5. Wait for callback and redirect
  await page.waitForURL(/mattermost/);
  
  // 6. Verify session cookie
  const cookies = await page.context().cookies();
  const authCookie = cookies.find(c => c.name === 'MMAUTHTOKEN');
  expect(authCookie).toBeDefined();
  
  // 7. Verify user logged in
  await expect(page.locator('.user-popover')).toBeVisible();
});
```

#### Invalid Credentials Test
```typescript
test('should handle invalid credentials', async ({ page }) => {
  await page.goto('/plugins/com.mm.oidc/');
  await page.click('text=Start Login');
  await page.waitForURL(/keycloak/);
  
  await page.fill('input[name="username"]', 'invalid');
  await page.fill('input[name="password"]', 'wrong');
  await page.click('input[type="submit"]');
  
  // Verify error message
  await expect(page.locator('.alert-error')).toBeVisible();
});
```

#### State Preservation Test
```typescript
test('should preserve state across redirect', async ({ page }) => {
  await page.goto('/plugins/com.mm.oidc/login?redirect_to=/channels/town-square');
  
  // Complete login...
  
  // Verify redirected to original URL
  await expect(page).toHaveURL(/channels\/town-square/);
});
```

### CI Integration

```yaml
# .github/workflows/e2e-tests.yml
name: E2E Tests
on: [push, pull_request]
jobs:
  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Start dev stack
        run: ./scripts/dev-up.sh
      - name: Run E2E tests
        run: ./scripts/e2e-test.sh
      - name: Upload test results
        if: always()
        uses: actions/upload-artifact@v3
        with:
          name: playwright-report
          path: e2e/playwright-report/
```

## Debugging Authentication Issues

### Enable Debug Logging

#### Mattermost Logs
```bash
# Follow logs in real-time
./scripts/dev-logs.sh mattermost

# Or via mmctl
mmctl logs --logrus | grep -i oidc
```

#### Keycloak Logs
```bash
# Follow Keycloak logs
./scripts/dev-logs.sh keycloak

# Or via kubectl/docker
docker compose -f deploy/docker-compose.dev.yml logs -f keycloak
```

### Inspect OIDC Parameters

#### Browser Developer Tools
1. Open browser DevTools (F12)
2. Go to Network tab
3. Initiate login
4. Inspect authorization request:
   - Check `client_id` matches configuration
   - Verify `redirect_uri` exactly matches Keycloak client
   - Confirm `code_challenge` is present (PKCE)
   - Verify `state` and `nonce` are random

#### Capture Authorization Request
```bash
# Using curl with manual flow
curl -v 'https://keycloak.example.com/realms/mattermost/protocol/openid-connect/auth?client_id=mm-oidc&redirect_uri=https%3A%2F%2Fchat.example.com%2Fplugins%2Fcom.mm.oidc%2Fcallback&response_type=code&scope=openid+profile+email&state=random123&nonce=random456&code_challenge=challenge&code_challenge_method=S256'
```

### Validate ID Token

#### Decode JWT
```bash
# Install jwt-cli
cargo install jwt-cli

# Decode ID token
jwt decode <id_token>

# Verify signature
jwt verify <id_token> --jwks-url https://keycloak.example.com/realms/mattermost/protocol/openid-connect/certs
```

#### Check Claims
Required claims:
- `sub`: Subject identifier (user ID)
- `iss`: Issuer (must match configured issuer)
- `aud`: Audience (must match client ID)
- `exp`: Expiration time
- `iat`: Issued at time
- `nonce`: Must match request nonce
- `email`: User email
- `email_verified`: true
- `preferred_username`: Username

### Common Issues and Solutions

#### Redirect URI Mismatch
**Symptom:** "invalid redirect_uri" error from Keycloak

**Debug:**
```bash
# Check configured redirect URI in Keycloak
# Check plugin configuration
mmctl config get PluginSettings.Plugins.com.mm.oidc.redirect_url

# Verify exact match (case-sensitive, including protocol, port, path)
```

**Solution:** Update redirect URI in Keycloak or plugin to match exactly

#### Invalid State Parameter
**Symptom:** "invalid state" error in plugin logs

**Debug:**
```bash
# Check plugin logs for state mismatch
grep "state" mattermost.log

# Verify state is stored and retrieved correctly
```

**Solution:** 
- Check plugin KV store is working
- Verify no proxy caching state parameter
- Check session cookies are preserved

#### Missing Claims
**Symptom:** "missing required claim" error

**Debug:**
```bash
# Decode ID token and check claims
jwt decode <id_token>

# Check Keycloak protocol mappers
# Verify user attributes are populated
```

**Solution:**
- Add missing protocol mappers in Keycloak
- Ensure user has required attributes (email, username)
- Verify email_verified is true

#### PKCE Validation Failure
**Symptom:** "invalid code_verifier" error

**Debug:**
```bash
# Check PKCE challenge and verifier match
# Verify code_challenge_method is S256
```

**Solution:**
- Ensure plugin generates and stores verifier correctly
- Check no proxy or cache interferes with PKCE flow

## Performance Testing

### Measure Flow Latency
```bash
# Using curl and timing
time curl -L -c cookies.txt -b cookies.txt \
  -X GET https://chat.example.com/plugins/com.mm.oidc/login

# Using Playwright with performance metrics
const startTime = Date.now();
await page.goto('/plugins/com.mm.oidc/login');
// ... complete flow ...
const endTime = Date.now();
console.log(`Flow completed in ${endTime - startTime}ms`);
```

### Load Testing
```bash
# Using k6 or similar tool
import http from 'k6/http';
import { check } from 'k6';

export default function () {
  const res = http.get('https://chat.example.com/plugins/com.mm.oidc/health');
  check(res, { 'status is 200': (r) => r.status === 200 });
}
```

## Best Practices

1. **Test all user scenarios**: new user, existing user, admin promotion
2. **Verify error handling**: invalid credentials, network failures
3. **Check security**: PKCE, state validation, nonce verification
4. **Test across browsers**: Chrome, Firefox, Safari
5. **Validate mobile compatibility**: iOS, Android
6. **Monitor performance**: track auth flow latency
7. **Run tests in CI**: automate regression testing
8. **Keep tests up to date**: sync with flow changes

## Related Skills
- dev-environment
- keycloak-setup
- plugin-install
- troubleshooting
- e2e-testing

## References
- [docs/E2E_TESTING.md](../../../docs/E2E_TESTING.md)
- [docs/ARCHITECTURE.md](../../../docs/ARCHITECTURE.md)
- [e2e/tests/oidc-flow.spec.ts](../../../e2e/tests/oidc-flow.spec.ts)
- [OIDC Specification](https://openid.net/specs/openid-connect-core-1_0.html)
