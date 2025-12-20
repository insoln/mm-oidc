import { test, expect } from '@playwright/test';

/**
 * OIDC Login Integration E2E Tests
 * 
 * These tests validate:
 * 1. Login button visibility on the /login page
 * 2. Login button click flow (redirect to OIDC)
 * 3. Configuration-based button display
 */

const KEYCLOAK_ADMIN_USER = process.env.KC_ADMIN || 'admin';
const KEYCLOAK_ADMIN_PASSWORD = process.env.KC_ADMIN_PASSWORD || 'Keycloak123!';
const MM_SITE_URL = process.env.MM_SITE_URL || 'http://localhost:8065';

test.describe('OIDC Login Integration', () => {
  test.beforeEach(async ({ page }) => {
    // Clear cookies to ensure fresh state
    await page.context().clearCookies();
  });

  test('should display OIDC login button on login page', async ({ page }) => {
    // Navigate to Mattermost login page
    await page.goto('/login');
    
    // Wait for page to load
    await page.waitForLoadState('networkidle');
    
    // Check if OIDC button is visible
    // The button should appear after the standard login form
    const oidcButton = page.locator('button:has-text("Sign in with OIDC")');
    
    // Wait for button to be visible (it fetches config asynchronously)
    await expect(oidcButton).toBeVisible({ timeout: 10000 });
    
    // Verify button text
    await expect(oidcButton).toContainText('Sign in with OIDC');
    
    // Verify the OR divider is present
    const divider = page.locator('text=/OR/i').first();
    await expect(divider).toBeVisible();
  });

  test('should not display OIDC button on non-login pages', async ({ page }) => {
    // Navigate to plugin landing page
    await page.goto('/plugins/com.mm.oidc/');
    
    // Wait for page to load
    await page.waitForLoadState('networkidle');
    
    // OIDC button should NOT be visible on this page
    const oidcButton = page.locator('button:has-text("Sign in with OIDC")');
    await expect(oidcButton).not.toBeVisible({ timeout: 5000 });
  });

  test('should redirect to OIDC login when button is clicked', async ({ page }) => {
    // Navigate to login page
    await page.goto('/login');
    
    // Wait for OIDC button to appear
    const oidcButton = page.locator('button:has-text("Sign in with OIDC")');
    await expect(oidcButton).toBeVisible({ timeout: 10000 });
    
    // Click the button
    await oidcButton.click();
    
    // Should redirect to plugin's login endpoint first
    // Then to Keycloak
    await page.waitForURL(/keycloak.*\/realms\/.*\/protocol\/openid-connect\/auth/i, { 
      timeout: 15000 
    });
    
    // Verify we're on Keycloak login page
    const url = page.url();
    expect(url).toMatch(/keycloak/i);
    expect(url).toMatch(/openid-connect/i);
  });

  test('should complete full login flow from login button', async ({ page }) => {
    // Step 1: Navigate to Mattermost login page
    await page.goto('/login');
    
    // Step 2: Wait for and click OIDC button
    const oidcButton = page.locator('button:has-text("Sign in with OIDC")');
    await expect(oidcButton).toBeVisible({ timeout: 10000 });
    await oidcButton.click();
    
    // Step 3: Wait for Keycloak login page
    await page.waitForURL(/keycloak.*\/realms\/.*\/protocol\/openid-connect\/auth/i, { 
      timeout: 15000 
    });
    
    // Step 4: Fill in Keycloak credentials
    await page.fill('input[name="username"]', KEYCLOAK_ADMIN_USER);
    await page.fill('input[name="password"]', KEYCLOAK_ADMIN_PASSWORD);
    
    // Step 5: Submit login form
    await page.click('input[type="submit"], button[type="submit"]');
    
    // Step 6: Wait for redirect back to Mattermost
    await page.waitForURL(new RegExp(MM_SITE_URL), { timeout: 15000 });
    
    // Step 7: Verify successful login
    const cookies = await page.context().cookies();
    const authCookie = cookies.find(c => c.name === 'MMAUTHTOKEN' || c.name === 'MMUSERID');
    expect(authCookie).toBeDefined();
    
    // Should not be on login page anymore
    const currentUrl = page.url();
    expect(currentUrl).not.toContain('/login');
  });

  test('should verify button styling and accessibility', async ({ page }) => {
    // Navigate to login page
    await page.goto('/login');
    
    // Wait for button
    const oidcButton = page.locator('button:has-text("Sign in with OIDC")');
    await expect(oidcButton).toBeVisible({ timeout: 10000 });
    
    // Check button is enabled
    await expect(oidcButton).toBeEnabled();
    
    // Check button has proper attributes
    const buttonType = await oidcButton.getAttribute('type');
    expect(buttonType).toBe('button');
    
    // Verify icon is present
    const icon = oidcButton.locator('svg');
    await expect(icon).toBeVisible();
    
    // Verify hint text is present
    const hint = page.locator('text=/authenticate using your organization/i');
    await expect(hint).toBeVisible();
  });

  test('should handle button click multiple times gracefully', async ({ page }) => {
    // Navigate to login page
    await page.goto('/login');
    
    // Wait for button
    const oidcButton = page.locator('button:has-text("Sign in with OIDC")');
    await expect(oidcButton).toBeVisible({ timeout: 10000 });
    
    // Click button
    await oidcButton.click();
    
    // Should redirect to Keycloak
    await page.waitForURL(/keycloak/i, { timeout: 15000 });
    
    // Go back to login page
    await page.goto('/login');
    
    // Button should still be visible and clickable
    await expect(oidcButton).toBeVisible({ timeout: 10000 });
    await expect(oidcButton).toBeEnabled();
  });

  test('should fetch and respect plugin configuration', async ({ page }) => {
    // Intercept config request
    let configFetched = false;
    page.on('response', response => {
      if (response.url().includes('/plugins/com.mm.oidc/config')) {
        configFetched = true;
      }
    });
    
    // Navigate to login page
    await page.goto('/login');
    
    // Wait for button (which should trigger config fetch)
    const oidcButton = page.locator('button:has-text("Sign in with OIDC")');
    await expect(oidcButton).toBeVisible({ timeout: 10000 });
    
    // Verify config was fetched
    expect(configFetched).toBe(true);
  });
});

test.describe('OIDC Login Integration - Configuration Tests', () => {
  test('should verify config endpoint returns expected data', async ({ request }) => {
    // Call config endpoint directly
    const response = await request.get('/plugins/com.mm.oidc/config');
    
    expect(response.ok()).toBeTruthy();
    
    const data = await response.json();
    expect(data).toHaveProperty('show_login_button');
    expect(data).toHaveProperty('issuer_url');
    
    // In dev environment, button should be enabled by default
    expect(data.show_login_button).toBe(true);
  });
});
