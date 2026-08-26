import type { FileRouteTypes } from "@/routeTree.gen";

export type GeneratedRoutePath = FileRouteTypes["fullPaths"];

export interface RouteStoryRegistryEntry {
  system: string;
  routePath: GeneratedRoutePath;
  storybookPath: string;
  title: `systems/${string}/routes/${string}`;
  storyName: string;
}

export interface RouteStoryExclusion {
  routePath: GeneratedRoutePath;
  reason: string;
}
