import type { RouteStoryRegistryEntry } from "./route-story-registry-types";

export const settingsRootRouteStories = [
  {
    system: "settings",
    routePath: "/settings",
    storybookPath: "/settings",
    title: "systems/settings/routes/SettingsShell",
    storyName: "Shell",
  },
  {
    system: "settings",
    routePath: "/settings/",
    storybookPath: "/settings/",
    title: "systems/settings/routes/SettingsShell",
    storyName: "IndexRedirect",
  },
  {
    system: "settings",
    routePath: "/sandbox",
    storybookPath: "/sandbox",
    title: "systems/settings/routes/Sandbox",
    storyName: "Default",
  },
] satisfies RouteStoryRegistryEntry[];

export const settingsDetailRouteStories = (
  [
    ["skills", "SettingsSkills"],
    ["providers", "SettingsProviders"],
    ["attention", "SettingsAttention"],
    ["observability", "SettingsObservability"],
    ["network", "SettingsNetwork"],
    ["gateway", "SettingsGateway"],
    ["memory", "SettingsMemory"],
    ["roles", "SettingsRoles", "Populated"],
    ["hooks", "SettingsHooks"],
    ["extensions", "SettingsExtensions"],
    ["general", "SettingsGeneral"],
    ["defaults", "SettingsDefaults"],
    ["appearance", "SettingsAppearance"],
    ["layouts", "SettingsLayouts"],
    ["profiles", "SettingsProfiles"],
    ["palette", "SettingsPalette"],
    ["automation", "SettingsAutomation"],
  ] as const
).map(([route, story, storyName = "Default"]) => ({
  system: "settings",
  routePath: `/settings/${route}`,
  storybookPath: `/settings/${route}`,
  title: `systems/settings/routes/${story}`,
  storyName,
})) satisfies RouteStoryRegistryEntry[];
