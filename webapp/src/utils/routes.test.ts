import {pluginBasePath, pluginId, pluginURL} from './routes';

describe('routes', () => {
  it('creates the expected base path', () => {
    expect(pluginBasePath).toBe(`/plugins/${pluginId}`);
  });

  it('normalizes paths', () => {
    expect(pluginURL('login')).toBe(`${pluginBasePath}/login`);
    expect(pluginURL('/health')).toBe(`${pluginBasePath}/health`);
  });
});
