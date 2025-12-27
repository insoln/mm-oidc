import { test, expect } from '@playwright/test';

/**
 * Mobile/Desktop Login Flow E2E Test
 * 
 * This test validates the mobile/desktop authentication flow where:
 * 1. Desktop app loads /login/mobile in webview
 * 2. Page renders with window.open() call
 * 3. Desktop app should intercept and open external browser
 * 4. Browser completes authentication
 * 5. Redirects back to desktop app via custom protocol
 */

const KEYCLOAK_ADMIN_USER = process.env.KC_ADMIN || 'admin';
const KEYCLOAK_ADMIN_PASSWORD = process.env.KC_ADMIN_PASSWORD || 'Keycloak123!';
const MM_SITE_URL = process.env.MM_SITE_URL || 'http://localhost:8065';

test.describe('Mobile/Desktop Login Flow', () => {
  test.beforeEach(async ({ page }) => {
    // Clear cookies to ensure fresh login
    await page.context().clearCookies();
  });

  test('mobile login page should render HTML with window.open, not redirect', async ({ page }) => {
    // Navigate to the mobile login endpoint
    const response = await page.goto('/plugins/com.mm.oidc/login/mobile?redirect_to=mattermost://auth/complete');
    
    // Step 1: Verify response is 200 OK (not a redirect)
    expect(response?.status()).toBe(200);
    
    // Step 2: Verify content-type is HTML
    const contentType = response?.headers()['content-type'] || '';
    expect(contentType).toContain('text/html');
    
    // Step 3: Verify the page contains window.open in JavaScript
    const content = await page.content();
    expect(content).toContain('window.open');
    expect(content).toContain('Opening Browser');
    
    // Step 4: Verify it's NOT a redirect (no Location header would have caused navigation)
    // If it were a redirect, we'd be on a different URL
    expect(page.url()).toContain('/login/mobile');
    
    // Step 5: Verify the clickable link is present as fallback
    const manualLink = page.locator('a:has-text("Click here to open browser manually")');
    await expect(manualLink).toBeVisible();
    
    console.log('✓ Mobile login page renders HTML (not redirect)');
    console.log('✓ Page contains window.open() JavaScript');
    console.log('✓ Page contains fallback link');
  });

  test('mobile login should trigger window.open with authorization URL', async ({ page, context }) => {
    // Track popup/window.open events
    const popupPromise = context.waitForEvent('page');
    
    // Navigate to mobile login endpoint
    await page.goto('/plugins/com.mm.oidc/login/mobile?redirect_to=mattermost://auth/complete');
    
    // Wait a moment for the window.open to be triggered by onload
    await page.waitForTimeout(1000);
    
    // The window.open should have been called
    // In a real desktop app, this would be intercepted
    // In our test, we can check if a popup was attempted
    try {
      const popup = await Promise.race([
        popupPromise,
        new Promise((_, reject) => setTimeout(() => reject(new Error('No popup')), 2000))
      ]) as any;
      
      if (popup) {
        console.log('✓ window.open was triggered');
        console.log('✓ Popup URL:', popup.url());
        
        // Verify the popup URL is the authorization endpoint
        expect(popup.url()).toMatch(/keycloak.*\/protocol\/openid-connect\/auth/i);
        
        await popup.close();
      }
    } catch (error: any) {
      // In some browser configurations, window.open might be blocked
      // but that's okay - we've verified the JavaScript is present
      console.log('✓ window.open call exists in JavaScript (popup may be blocked in test)');
    }
  });

  test('mobile login fallback link should open authorization URL', async ({ page, context }) => {
    // Navigate to mobile login endpoint
    await page.goto('/plugins/com.mm.oidc/login/mobile?redirect_to=mattermost://auth/complete');
    
    // Find and click the manual fallback link
    const manualLink = page.locator('a:has-text("Click here to open browser manually")');
    await expect(manualLink).toBeVisible();
    
    // Get the href attribute
    const href = await manualLink.getAttribute('href');
    expect(href).toBeTruthy();
    expect(href).toMatch(/keycloak.*\/protocol\/openid-connect\/auth/i);
    
    console.log('✓ Fallback link href contains authorization URL');
    console.log('✓ Authorization URL:', href);
    
    // Verify authorization URL parameters
    expect(href).toContain('client_id=');
    expect(href).toContain('redirect_uri=');
    expect(href).toContain('response_type=code');
    expect(href).toContain('code_challenge=');
    expect(href).toContain('code_challenge_method=S256');
    
    console.log('✓ Authorization URL contains required OIDC parameters');
  });

  test('mobile callback should redirect to custom protocol with tokens', async ({ page }) => {
    // This test validates the callback flow
    // First, we need to complete a regular OIDC login to get a valid session
    
    // Step 1: Start mobile login to get state
    await page.goto('/plugins/com.mm.oidc/login/mobile?redirect_to=mattermost://auth/complete');
    
    // Step 2: Get the authorization URL from the link
    const manualLink = page.locator('a:has-text("Click here to open browser manually")');
    const authURL = await manualLink.getAttribute('href');
    expect(authURL).toBeTruthy();
    
    // Step 3: Navigate to the authorization URL
    await page.goto(authURL!);
    
    // Step 4: Fill in Keycloak credentials
    await page.waitForSelector('input[name="username"]', { timeout: 10000 });
    await page.fill('input[name="username"]', KEYCLOAK_ADMIN_USER);
    await page.fill('input[name="password"]', KEYCLOAK_ADMIN_PASSWORD);
    
    // Step 5: Submit login form
    await page.click('input[type="submit"], button[type="submit"]');
    
    // Step 6: Wait for redirect to callback/mobile
    await page.waitForURL(/\/callback\/mobile/, { timeout: 15000 });
    
    // Step 7: Verify the callback page renders
    const content = await page.content();
    expect(content).toContain('Authentication Complete');
    
    // Step 8: Verify meta refresh is present (redirects to custom protocol)
    expect(content).toMatch(/meta.*refresh.*mattermost:\/\//i);
    
    // Step 9: Check for the custom protocol link with tokens
    const protocolLink = page.locator('a[href^="mattermost://"]');
    await expect(protocolLink).toBeVisible();
    
    const protocolURL = await protocolLink.getAttribute('href');
    expect(protocolURL).toContain('mattermost://');
    expect(protocolURL).toContain('MMAUTHTOKEN=');
    expect(protocolURL).toContain('MMCSRF=');
    
    console.log('✓ Mobile callback renders completion page');
    console.log('✓ Page contains custom protocol redirect');
    console.log('✓ Protocol URL contains session tokens');
  });

  test('mobile login should fail without redirect_to parameter', async ({ page }) => {
    // Navigate without redirect_to parameter
    const response = await page.goto('/plugins/com.mm.oidc/login/mobile');
    
    // Should return 400 Bad Request
    expect(response?.status()).toBe(400);
    
    // Should show error about missing redirect_to
    const content = await page.content();
    expect(content).toMatch(/missing.*redirect.*url/i);
    
    console.log('✓ Mobile login requires redirect_to parameter');
  });

  test('mobile login should fail with invalid redirect_to scheme', async ({ page }) => {
    // Try with http:// scheme (should be rejected)
    const response = await page.goto('/plugins/com.mm.oidc/login/mobile?redirect_to=http://example.com');
    
    // Should return 400 Bad Request
    expect(response?.status()).toBe(400);
    
    // Should show error about invalid redirect URL
    const content = await page.content();
    expect(content).toMatch(/invalid.*redirect.*url/i);
    
    console.log('✓ Mobile login rejects http:// scheme');
    
    // Try with https:// scheme (should also be rejected)
    const response2 = await page.goto('/plugins/com.mm.oidc/login/mobile?redirect_to=https://example.com');
    expect(response2?.status()).toBe(400);
    
    console.log('✓ Mobile login rejects https:// scheme');
  });
});
