import type {ComponentType} from 'react';

declare global {
  interface PluginRegistry {
    registerRootComponent(component: ComponentType): void;
  }

  interface Window {
    registerPlugin?: (id: string, plugin: {initialize(registry: PluginRegistry): void}) => void;
  }
}

export type {PluginRegistry};
