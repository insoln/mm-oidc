import { test, expect } from '@playwright/test';

/**
 * OIDC Authentication Flow E2E Tests
 * 
 * These tests validate the full Authorization Code + PKCE flow:
 * 1. User clicks login on plugin page
 * 2. Redirects to Keycloak
 * 3. User authenticates with Keycloak
 * 4. Redirects back to Mattermost with auth code
 * 5. Plugin exchanges code for tokens and provisions user
 */

// Dev-only defaults for the local docker/dev stack.
// NOTE:
// - Real environments MUST provide credentials and URLs via environment variables.
// - These fallback values and the associated dev.env file are for local development ONLY.
// - dev.env must never contain production credentials or be used in any production environment.
const KEYCLOAK_ADMIN_USER = process.env.KC_ADMIN || 'admin';
const KEYCLOAK_ADMIN_PASSWORD = process.env.KC_ADMIN_PASSWORD || 'Keycloak123!';
const MM_SITE_URL = process.env.MM_SITE_URL || 'http://localhost:8065';

test.describe('OIDC Authentication Flow', () => {
  test.beforeEach(async ({ page }) => {
    // Clear cookies to ensure fresh login
    await page.context().clearCookies();
  });

  test('should complete full OIDC login flow', async ({ page }) => {
    // Step 1: Navigate to plugin landing page
    await page.goto('/plugins/com.mm.oidc/');
    
    // Step 2: Click the "Start Login" button to initiate OIDC flow
    const loginButton = page.locator('a[href*="/login"], button:has-text("Start Login"), a:has-text("Start Login")').first();
    await expect(loginButton).toBeVisible({ timeout: 10000 });
    
    // Click and wait for navigation to Keycloak
    await loginButton.click();
    
    // Step 3: Wait for redirect to Keycloak login page
    await page.waitForURL(/keycloak.*\/realms\/.*\/protocol\/openid-connect\/auth/i, { timeout: 15000 });
    
    // Step 4: Fill in Keycloak credentials
    await page.fill('input[name="username"]', KEYCLOAK_ADMIN_USER);
    await page.fill('input[name="password"]', KEYCLOAK_ADMIN_PASSWORD);
    
    // Step 5: Submit login form
    await page.click('input[type="submit"], button[type="submit"]');
    
    // Step 6: Wait for redirect back to Mattermost (any route under the configured site URL).
    // NOTE: This timeout is intentionally higher than typical navigation waits to account
    // for slower CI/dev Docker environments. If this frequently approaches the limit, treat it
    // as a regression in the auth/redirect path rather than increasing the timeout further.
    await expect.poll(async () => page.url(), { timeout: 15000 }).toContain(MM_SITE_URL);
    
    // Step 7: Verify successful login by checking for Mattermost auth cookie, using the same
    // bounded timeout as the redirect above to avoid masking slow auth flows.
    await expect.poll(async () => {
      const cookies = await page.context().cookies();
      return cookies.some(c => c.name === 'MMAUTHTOKEN' || c.name === 'MMUSERID');
    }, { timeout: 15000 }).toBe(true);
    const cookies = await page.context().cookies();
    const authCookie = cookies.find(c => c.name === 'MMAUTHTOKEN' || c.name === 'MMUSERID');
    expect(authCookie).toBeDefined();
    
    // Step 8: Verify user is logged in by checking the page content
    // Wait for Mattermost to complete loading after authentication
    await page.waitForLoadState('networkidle');
    const currentUrl = page.url();
    
    // Should not be on login page or error page
    expect(currentUrl).not.toContain('/login');
    expect(currentUrl).not.toContain('/error');
  });

  test('should handle invalid credentials gracefully', async ({ page }) => {
    // Navigate to plugin landing page
    await page.goto('/plugins/com.mm.oidc/');
    
    // Start login flow
    const loginButton = page.locator('a[href*="/login"], button:has-text("Start Login"), a:has-text("Start Login")').first();
    await loginButton.click();
    
    // Wait for Keycloak
    await page.waitForURL(/keycloak.*\/realms\/.*\/protocol\/openid-connect\/auth/i, { timeout: 15000 });
    
    // Use invalid credentials
    await page.fill('input[name="username"]', 'invalid-user');
    await page.fill('input[name="password"]', 'wrong-password');
    await page.click('input[type="submit"], button[type="submit"]');
    
    // Should see error message on Keycloak page
    await expect(page.locator('text=/invalid.*credentials|Invalid username or password/i')).toBeVisible({ timeout: 10000 });
    
    // Verify we're still on Keycloak and not redirected to Mattermost
    expect(page.url()).toMatch(/keycloak/i);
  });

  test('should preserve state parameter during auth flow', async ({ page }) => {
    // Navigate to login endpoint
    await page.goto('/plugins/com.mm.oidc/login');
    
    // Should redirect to Keycloak with state parameter
    await page.waitForURL(/keycloak.*\/realms\/.*\/protocol\/openid-connect\/auth/i, { timeout: 15000 });
    
    const url = page.url();
    const urlParams = new URL(url).searchParams;
    
    // Verify OIDC parameters
    expect(urlParams.get('response_type')).toBe('code');
    expect(urlParams.get('client_id')).toBeTruthy();
    expect(urlParams.get('redirect_uri')).toContain('/callback');
    expect(urlParams.get('state')).toBeTruthy();
    expect(urlParams.get('nonce')).toBeTruthy();
    expect(urlParams.get('code_challenge')).toBeTruthy(); // PKCE
    expect(urlParams.get('code_challenge_method')).toBe('S256');
  });
});
