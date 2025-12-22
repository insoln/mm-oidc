import { test, expect } from '@playwright/test';

/**
 * NGINX Proxy OIDC Redirect E2E Tests
 * 
 * These tests validate the automatic redirect behavior when using NGINX proxy:
 * 1. Unauthenticated users are redirected to OIDC login
 * 2. API endpoints don't redirect
 * 3. After login, users can access the site normally
 * 4. WebSocket connections work
 */

const PROXY_BASE_URL = process.env.PROXY_BASE_URL || 'http://localhost';
const KEYCLOAK_ADMIN_USER = process.env.KC_ADMIN || 'admin';
const KEYCLOAK_ADMIN_PASSWORD = process.env.KC_ADMIN_PASSWORD || 'Keycloak123!';

test.describe('NGINX Proxy OIDC Redirect', () => {
  test.beforeEach(async ({ page }) => {
    // Clear cookies to ensure fresh state
    await page.context().clearCookies();
  });

  test('should redirect unauthenticated user to OIDC login', async ({ page }) => {
    // Navigate to root URL without auth cookie
    const response = await page.goto(PROXY_BASE_URL + '/');
    
    // Should be redirected to OIDC login page
    // Either we follow the redirect and land on the login page,
    // or we check the redirect response
    const finalUrl = page.url();
    
    expect(finalUrl).toMatch(/\/plugins\/com\.mm\.oidc\/login/);
  });

  test('should redirect with original URL as redirect_to parameter', async ({ page }) => {
    // Navigate to a specific path
    await page.goto(PROXY_BASE_URL + '/channels/town-square');
    
    // Should be redirected with the original URL
    const finalUrl = page.url();
    
    expect(finalUrl).toMatch(/\/plugins\/com\.mm\.oidc\/login/);
    expect(finalUrl).toMatch(/redirect_to=.*channels.*town-square/);
  });

  test('should allow access after successful OIDC login', async ({ page }) => {
    // Navigate to root - will be redirected to OIDC login
    await page.goto(PROXY_BASE_URL + '/');
    
    // Should land on OIDC login page or be redirected to Keycloak
    await page.waitForURL(/\/plugins\/com\.mm\.oidc\/login|keycloak.*\/auth/i, { timeout: 10000 });
    
    // If on plugin login page, click the login button
    const currentUrl = page.url();
    if (currentUrl.includes('/plugins/com.mm.oidc/login')) {
      const loginButton = page.locator('a[href*="/login"], button:has-text("Start Login"), a:has-text("Start Login")').first();
      await loginButton.click();
    }
    
    // Wait for Keycloak login page
    await page.waitForURL(/keycloak.*\/realms\/.*\/protocol\/openid-connect\/auth/i, { timeout: 15000 });
    
    // Fill in Keycloak credentials
    await page.fill('input[name="username"]', KEYCLOAK_ADMIN_USER);
    await page.fill('input[name="password"]', KEYCLOAK_ADMIN_PASSWORD);
    
    // Submit login
    await page.click('input[type="submit"], button[type="submit"]');
    
    // Wait for redirect back to Mattermost
    await page.waitForURL(/localhost|mattermost/, { timeout: 15000 });
    
    // Verify we have MMAUTHTOKEN cookie
    const cookies = await page.context().cookies();
    const authCookie = cookies.find(c => c.name === 'MMAUTHTOKEN');
    expect(authCookie).toBeDefined();
    
    // Now accessing root should work without redirect
    await page.goto(PROXY_BASE_URL + '/');
    
    // Should stay on Mattermost, not redirect to login
    const finalUrl = page.url();
    expect(finalUrl).not.toMatch(/\/plugins\/com\.mm\.oidc\/login/);
    expect(finalUrl).toMatch(/localhost|mattermost/);
  });

  test('API endpoint should not redirect, should return 401 or 200', async ({ page, request }) => {
    // Make API request without auth
    const response = await request.get(PROXY_BASE_URL + '/api/v4/system/ping');
    
    // Should NOT be a redirect (302)
    expect(response.status()).not.toBe(302);
    
    // Should be 200 (public endpoint) or 401 (protected endpoint)
    expect([200, 401]).toContain(response.status());
  });

  test('plugin health endpoint should be accessible without redirect', async ({ page, request }) => {
    // Access plugin health endpoint
    const response = await request.get(PROXY_BASE_URL + '/plugins/com.mm.oidc/health');
    
    // Should NOT redirect
    expect(response.status()).not.toBe(302);
    
    // Should return success or plugin response
    expect([200, 404]).toContain(response.status());
  });

  test('static files should be accessible without redirect', async ({ page, request }) => {
    // Try to access a static file path
    const response = await request.get(PROXY_BASE_URL + '/static/', {
      maxRedirects: 0,
      failOnStatusCode: false
    });
    
    // Should NOT be a redirect to OIDC
    if (response.status() === 302) {
      const location = response.headers()['location'];
      expect(location).not.toMatch(/\/plugins\/com\.mm\.oidc\/login/);
    }
  });

  test('POST request should not redirect', async ({ page, request }) => {
    // Make POST request without auth
    const response = await request.post(PROXY_BASE_URL + '/api/v4/users/login', {
      data: {
        login_id: 'test',
        password: 'test'
      },
      maxRedirects: 0,
      failOnStatusCode: false
    });
    
    // Should NOT be a redirect (302)
    expect(response.status()).not.toBe(302);
    
    // Should return 401 or other error, but not redirect
    expect([400, 401, 403, 404, 405]).toContain(response.status());
  });

  test('callback URL should not redirect', async ({ page, request }) => {
    // Access callback URL (without valid code, but should not redirect)
    const response = await request.get(PROXY_BASE_URL + '/plugins/com.mm.oidc/callback', {
      maxRedirects: 0,
      failOnStatusCode: false
    });
    
    // Should NOT be a redirect loop
    expect(response.status()).not.toBe(302);
  });

  test('health check endpoint should be accessible', async ({ page, request }) => {
    const response = await request.get(PROXY_BASE_URL + '/health');
    
    // Should return 200 OK
    expect(response.status()).toBe(200);
  });
});

test.describe('NGINX Proxy Security Headers', () => {
  test('should include security headers in response', async ({ page, request }) => {
    const response = await request.get(PROXY_BASE_URL + '/plugins/com.mm.oidc/health');
    
    const headers = response.headers();
    
    // Check for security headers
    expect(headers['x-frame-options']).toBeDefined();
    expect(headers['x-content-type-options']).toBeDefined();
    expect(headers['x-xss-protection']).toBeDefined();
  });
});

test.describe('NGINX Proxy Redirect Behavior', () => {
  test('should preserve query parameters in redirect', async ({ page }) => {
    // Navigate with query params
    await page.goto(PROXY_BASE_URL + '/?param1=value1&param2=value2');
    
    const finalUrl = page.url();
    
    // Should redirect to OIDC login
    expect(finalUrl).toMatch(/\/plugins\/com\.mm\.oidc\/login/);
    
    // Should preserve original URL with query params in redirect_to
    expect(finalUrl).toMatch(/redirect_to=.*param1=value1/);
  });

  test('should not redirect requests with Accept: application/json', async ({ page, request }) => {
    // Make request with JSON accept header
    const response = await request.get(PROXY_BASE_URL + '/', {
      headers: {
        'Accept': 'application/json'
      },
      maxRedirects: 0,
      failOnStatusCode: false
    });
    
    // Should NOT redirect to OIDC (API client)
    if (response.status() === 302) {
      const location = response.headers()['location'];
      expect(location).not.toMatch(/\/plugins\/com\.mm\.oidc\/login/);
    }
  });
});
