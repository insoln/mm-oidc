import { test, expect } from '@playwright/test';

/**
 * Desktop Client OAuth Flow E2E Test
 * 
 * This test simulates a desktop client (Electron app) initiating OAuth:
 * 1. Desktop app opens /login?isMobile=true in system browser
 * 2. Plugin serves HTML with window.open() to trigger external browser for Keycloak
 * 3. User authenticates with Keycloak  
 * 4. Redirects to /complete page with session tokens
 * 5. Page automatically triggers mattermost:// deep link
 * 6. Desktop app intercepts protocol handler and completes login
 */

const KEYCLOAK_ADMIN_USER = process.env.KC_ADMIN || 'admin';
const KEYCLOAK_ADMIN_PASSWORD = process.env.KC_ADMIN_PASSWORD || 'Keycloak123!';
const MM_SITE_URL = process.env.MM_SITE_URL || 'http://localhost:8065';

test.describe('Desktop Client OAuth Flow', () => {
  test.beforeEach(async ({ page }) => {
    await page.context().clearCookies();
  });

  test('should render window.open() page for mobile client', async ({ page }) => {
    // Navigate to login with isMobile=true parameter (simulating desktop client)
    await page.goto('/plugins/com.mm.oidc/login?isMobile=true');
    
    // Should see HTML page with window.open() JavaScript, not an HTTP redirect
    await expect(page).toHaveTitle(/Opening.*Browser/i);
    
    // Verify the page contains the external browser trigger message
    await expect(page.locator('text=/Opening.*browser/i')).toBeVisible();
    
    // Verify window.open() is called by checking for the fallback link
    await expect(page.locator('text=/didn\'t open automatically/i')).toBeVisible({ timeout: 5000 });
  });

  test('should trigger deep link on complete page', async ({ page }) => {
    let deepLinkTriggered = false;
    let deepLinkUrl = '';

    // Intercept any navigation attempts to mattermost:// protocol
    page.on('framenavigated', (frame) => {
      const url = frame.url();
      if (url.startsWith('mattermost://')) {
        deepLinkTriggered = true;
        deepLinkUrl = url;
      }
    });

    // Also check for location.href assignments via page evaluate
    await page.addInitScript(() => {
      const originalSetter = Object.getOwnPropertyDescriptor(window.Location.prototype, 'href')!.set!;
      Object.defineProperty(window.location, 'href', {
        set: function(value) {
          if (typeof value === 'string' && value.startsWith('mattermost://')) {
            (window as any).__deepLinkCalled = value;
          }
          originalSetter.call(this, value);
        }
      });
    });

    // Step 1: Navigate to login with isMobile=true
    await page.goto('/plugins/com.mm.oidc/login?isMobile=true');
    
    // Step 2: In a real scenario, window.open() would launch browser
    // For testing, we need to manually navigate through the OAuth flow
    // Find and extract the OAuth URL from the page
    const pageContent = await page.content();
    const oauthUrlMatch = pageContent.match(/window\.open\(['"]([^'"]+)['"]/);
    
    if (oauthUrlMatch && oauthUrlMatch[1]) {
      const oauthUrl = oauthUrlMatch[1];
      
      // Navigate directly to the OAuth URL (simulating what window.open would do)
      await page.goto(oauthUrl);
      
      // Step 3: Complete Keycloak authentication
      await page.waitForURL(/keycloak.*\/realms\/.*\/protocol\/openid-connect\/auth/i, { timeout: 15000 });
      await page.fill('input[name="username"]', KEYCLOAK_ADMIN_USER);
      await page.fill('input[name="password"]', KEYCLOAK_ADMIN_PASSWORD);
      await page.click('input[type="submit"], button[type="submit"]');
      
      // Step 4: Should redirect to /complete page
      await page.waitForURL(/\/plugins\/com\.mm\.oidc\/complete/, { timeout: 15000 });
      
      // Step 5: Verify page shows "Redirecting to Mattermost"
      await expect(page.locator('text=/Redirecting to Mattermost/i')).toBeVisible();
      
      // Step 6: Wait a moment for JavaScript to execute
      await page.waitForTimeout(1000);
      
      // Step 7: Check if deep link was attempted
      const deepLinkCalled = await page.evaluate(() => (window as any).__deepLinkCalled);
      
      if (deepLinkCalled) {
        console.log('✓ Deep link triggered:', deepLinkCalled);
        expect(deepLinkCalled).toContain('mattermost://');
        expect(deepLinkCalled).toContain('MMAUTHTOKEN=');
        expect(deepLinkCalled).toContain('MMUSERID=');
      }
      
      // Step 8: Verify manual fallback link appears after timeout
      await expect(page.locator('text=/didn\'t open automatically/i')).toBeVisible({ timeout: 5000 });
      
      // Step 9: Verify the manual link has mattermost:// protocol
      const manualLink = page.locator('a[href^="mattermost://"]');
      await expect(manualLink).toBeVisible();
      
      const linkHref = await manualLink.getAttribute('href');
      expect(linkHref).toContain('mattermost://');
      expect(linkHref).toContain('MMAUTHTOKEN=');
      
      // Step 10: Verify diagnostics are shown
      await expect(page.locator('text=/Diagnostics payload/i')).toBeVisible();
      
      // Step 11: Verify auto-attempts counter is > 0
      const attemptCount = await page.locator('#attempt-count').textContent();
      expect(parseInt(attemptCount || '0')).toBeGreaterThan(0);
      
      console.log('✓ Desktop client flow completed successfully');
      console.log('✓ Deep link attempts:', attemptCount);
    } else {
      throw new Error('Could not find OAuth URL in window.open() call');
    }
  });

  test('should show diagnostics with session information', async ({ page }) => {
    // Navigate directly to a complete page (we'll fake the parameters for testing)
    const fakeToken = 'test_token_' + Math.random().toString(36).substring(7);
    const fakeUserId = 'test_user_id';
    
    await page.goto(`/plugins/com.mm.oidc/complete?MMAUTHTOKEN=${fakeToken}&MMUSERID=${fakeUserId}&MMREDIRECT=/`);
    
    // Should show the complete page (even with invalid tokens, the page should render)
    await expect(page.locator('text=/Redirecting to Mattermost/i')).toBeVisible();
    
    // Verify diagnostics section is present
    await expect(page.locator('text=/Diagnostics payload/i')).toBeVisible();
    
    // Verify we can see diagnostic information
    await expect(page.locator('text=/Token SHA256/i')).toBeVisible();
    await expect(page.locator('text=/CSRF SHA256/i')).toBeVisible();
    
    // Verify action buttons exist
    await expect(page.locator('button:has-text("Copy JSON snapshot")')).toBeVisible();
    await expect(page.locator('button:has-text("Retry deep link")')).toBeVisible();
    
    console.log('✓ Diagnostics UI rendered correctly');
  });

  test('should validate mattermost:// protocol on deep link', async ({ page }) => {
    // Add a console listener to capture validation errors
    const consoleMessages: string[] = [];
    page.on('console', msg => {
      consoleMessages.push(msg.text());
    });

    // Navigate to complete page with valid parameters
    const fakeToken = 'test_token_abc123';
    const fakeUserId = 'user123';
    
    await page.goto(`/plugins/com.mm.oidc/complete?MMAUTHTOKEN=${fakeToken}&MMUSERID=${fakeUserId}`);
    
    // Wait for page to load and JavaScript to execute
    await page.waitForTimeout(1000);
    
    // The deep link should be validated and triggered
    // Check diagnostics JSON for validation errors
    const diagButton = page.locator('button:has-text("Copy JSON")');
    await expect(diagButton).toBeVisible();
    
    // Click the diagnostics area to expand it if needed
    const diagPre = page.locator('pre#diagnostics-json');
    const diagContent = await diagPre.textContent();
    
    // Should not have deepLinkValidationError in diagnostics
    expect(diagContent).not.toContain('deepLinkValidationError');
    
    console.log('✓ Deep link protocol validation working');
  });
});
