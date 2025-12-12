export const pluginId = 'com.mm.oidc';
export const pluginBasePath = `/plugins/${pluginId}`;

export const pluginURL = (subPath = '/') => {
  const normalized = subPath.startsWith('/') ? subPath : `/${subPath}`;
  return `${pluginBasePath}${normalized}`;
};

export const loginURL = pluginURL('/login');
export const healthURL = pluginURL('/health');
