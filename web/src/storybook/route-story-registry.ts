import { appRouteStories } from "./app-route-stories";
import {
  type GeneratedRoutePath,
  type RouteStoryExclusion,
  type RouteStoryRegistryEntry,
} from "./route-story-types";
import { settingsRouteStories } from "./settings-route-stories";
import { terminalRouteStories } from "./terminal-route-stories";

export type { GeneratedRoutePath, RouteStoryExclusion, RouteStoryRegistryEntry };

export const routeStoryExclusions: RouteStoryExclusion[] = [];

export const routeStoryRegistry = [
  ...terminalRouteStories,
  ...appRouteStories,
  ...settingsRouteStories,
] satisfies RouteStoryRegistryEntry[];
