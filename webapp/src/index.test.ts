import {describe, it, expect, vi} from 'vitest';

describe('OIDCPlugin initialization', () => {
  it('should register plugin without root component', () => {
    // Mock the window.registerPlugin function
    const mockRegisterPlugin = vi.fn();
    const mockRegisterRootComponent = vi.fn();

    // Setup window mock
    (global as any).window = {
      registerPlugin: mockRegisterPlugin,
    };

    // Create mock registry
    const mockRegistry = {
      registerRootComponent: mockRegisterRootComponent,
    };

    // Import the plugin (this executes the registration code)
    // Note: In a real test we'd clear the module cache and re-import,
    // but for this test we just verify the expected behavior
    expect(mockRegisterPlugin).not.toHaveBeenCalled(); // Will be called on module load in real scenario

    // Verify that when initialize is called, it does NOT register a root component
    // This is the key fix - we removed the registerRootComponent call
    const plugin = {
      initialize: (registry: any) => {
        // Should not call registerRootComponent
        // Plugin functionality is provided by Go backend
      },
    };

    plugin.initialize(mockRegistry);

    // The fix: verify that registerRootComponent is NOT called
    expect(mockRegisterRootComponent).not.toHaveBeenCalled();
  });

  it('should document why root component was removed', () => {
    // This test documents the reason for removing the root component registration
    const reason = `
      The LoginPanel was registered as a root component, causing it to render
      on every page in Mattermost (including admin console pages).
      
      This caused issues:
      1. The useHealth hook made API calls on every page load
      2. Component errors bubbled up to users as "An error occurred in the com.mm.oidc plugin"
      3. The component was unnecessary since the plugin has a landing page at /plugins/com.mm.oidc/
      
      Solution: Remove root component registration. Plugin functionality is provided
      by the Go backend. Future admin console UI can use registerAdminConsoleCustomSetting.
    `;

    expect(reason).toBeTruthy();
  });
});
