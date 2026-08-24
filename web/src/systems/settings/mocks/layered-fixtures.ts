import type { SettingsHookCollection, SettingsPersonaSection } from "@/systems/settings";

import { settingsHookFixtures } from "./fixtures";

type LayeredFixtureTarget =
  | { scope: "user" }
  | { scope: "profile"; profile: string; workspace_id?: string }
  | { scope: "workspace"; workspace_id: string };

export function settingsPersonaSectionFixtureFor(
  target: LayeredFixtureTarget
): SettingsPersonaSection {
  const profile = target.scope === "profile" ? target.profile : undefined;
  return {
    section: "persona",
    available_scopes: ["user", "profile", "workspace"],
    config: {
      agent: profile === "marketing" ? "campaigns" : "general",
      provider: profile === "marketing" ? "openai" : "claude",
      sandbox: "local",
    },
    ...target,
  };
}

export function settingsHooksCollectionFixtureFor(
  target: LayeredFixtureTarget
): SettingsHookCollection {
  return {
    available_scopes: ["user", "profile", "workspace"],
    collection: "hooks",
    hooks: settingsHookFixtures,
    ...target,
  };
}

export const settingsPersonaSectionFixture = settingsPersonaSectionFixtureFor({ scope: "user" });
export const settingsHooksCollectionFixture = settingsHooksCollectionFixtureFor({ scope: "user" });
