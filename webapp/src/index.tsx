import type {PluginRegistry} from './types/mattermost';
import LoginPanel from './components/LoginPanel';
import {pluginId} from './utils/routes';

class OIDCPlugin {
  initialize(registry: PluginRegistry) {
    registry.registerRootComponent(LoginPanel);
  }
}

const plugin = new OIDCPlugin();

if (typeof window !== 'undefined' && window.registerPlugin) {
  window.registerPlugin(pluginId, plugin);
}
