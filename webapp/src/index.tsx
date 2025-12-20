import type {PluginRegistry} from './types/mattermost';
import {pluginId} from './utils/routes';
import LoginButton from './components/LoginButton';

class OIDCPlugin {
  initialize(registry: PluginRegistry) {
    // Register the login button as a root component
    // This will render it globally, but the component itself
    // will only display on the /login page
    registry.registerRootComponent(LoginButton);

    // Plugin functionality is provided by the Go backend at /plugins/com.mm.oidc/
    // Future admin console settings UI can be registered here via registerAdminConsoleCustomSetting
  }
}

const plugin = new OIDCPlugin();

if (typeof window !== 'undefined' && window.registerPlugin) {
  window.registerPlugin(pluginId, plugin);
}

// Export for testing
export default OIDCPlugin;
