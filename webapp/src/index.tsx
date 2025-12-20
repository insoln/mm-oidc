import type {PluginRegistry} from './types/mattermost';
import {pluginId} from './utils/routes';

class OIDCPlugin {
  initialize(_registry: PluginRegistry) {
    // Plugin functionality is provided by the Go backend at /plugins/com.mm.oidc/
    // Future admin console settings UI can be registered here via registerAdminConsoleCustomSetting
  }
}

const plugin = new OIDCPlugin();

if (typeof window !== 'undefined' && window.registerPlugin) {
  window.registerPlugin(pluginId, plugin);
}
