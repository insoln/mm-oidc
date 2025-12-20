import { test, expect } from '@playwright/test';

test.describe('Plugin Health Check', () => {
  test('should return healthy status from health endpoint', async ({ page }) => {
    // Navigate to the plugin health endpoint
    const response = await page.goto('/plugins/com.mm.oidc/health');
    
    expect(response).not.toBeNull();
    expect(response?.status()).toBe(200);
    
    // Check response body
    const healthData = await response?.json();
    expect(healthData).toBeDefined();
    expect(healthData.status).toBe('healthy');
  });

  test('should display plugin landing page', async ({ page }) => {
    // Navigate to the plugin root
    await page.goto('/plugins/com.mm.oidc/');
    
    // Check that the page loads successfully
    await expect(page).toHaveURL(/\/plugins\/com\.mm\.oidc\//);
    
    // Look for key elements on the landing page
    await expect(page.locator('body')).toContainText(/OIDC|OpenID Connect|Keycloak/i);
  });
});
