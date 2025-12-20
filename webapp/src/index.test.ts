import {describe, it, expect, vi, beforeEach} from 'vitest';

// Mock window.registerPlugin before importing the module
const mockRegisterPlugin = vi.fn();
(global as any).window = {
  registerPlugin: mockRegisterPlugin,
};

describe('OIDCPlugin initialization', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should not call registerRootComponent when initialize is called', async () => {
    // Import the actual module to get the real OIDCPlugin class
    const {default: OIDCPlugin} = await import('./index');
    
    // Create mock registry with registerRootComponent spy
    const mockRegisterRootComponent = vi.fn();
    const mockRegistry = {
      registerRootComponent: mockRegisterRootComponent,
    };

    // Create instance and call initialize
    const plugin = new (OIDCPlugin as any)();
    plugin.initialize(mockRegistry);

    // Verify that registerRootComponent is NOT called
    // This is the key fix - we removed the registerRootComponent call that was
    // causing the LoginPanel to render on every page and trigger health check API calls
    expect(mockRegisterRootComponent).not.toHaveBeenCalled();
  });
});
