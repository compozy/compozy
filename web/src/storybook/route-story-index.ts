import {
  routeStoryRegistry,
  type GeneratedRoutePath,
  type RouteStoryRegistryEntry,
} from "./route-story-registry";

export const routeStoryRegistryByRoutePath = new Map<GeneratedRoutePath, RouteStoryRegistryEntry>(
  routeStoryRegistry.map(entry => [entry.routePath, entry])
);

export const routeStorySystems = Array.from(
  new Set(routeStoryRegistry.map(entry => entry.system))
).sort();
