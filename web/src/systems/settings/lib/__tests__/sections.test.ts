import { describe, expect, it } from "vitest";

import {
  findSettingsSection,
  SETTINGS_ROOT_PATH,
  SETTINGS_SECTIONS,
  settingsSectionPath,
} from "../sections";

describe("settings sections metadata", () => {
  it("exposes the Settings screen order", () => {
    expect(SETTINGS_SECTIONS.map(section => section.slug)).toEqual([
      "general",
      "appearance",
      "layouts",
      "providers",
      "memory",
      "roles",
      "skills",
      "automation",
      "network",
      "observability",
      "hooks",
      "extensions",
    ]);
  });

  it("provides nested paths rooted under the settings shell", () => {
    expect(SETTINGS_ROOT_PATH).toBe("/settings");
    expect(settingsSectionPath("providers")).toBe("/settings/providers");
    expect(settingsSectionPath("layouts")).toBe("/settings/layouts");
    expect(settingsSectionPath("hooks")).toBe("/settings/hooks");
    expect(settingsSectionPath("extensions")).toBe("/settings/extensions");
  });

  it("looks sections up by slug", () => {
    expect(findSettingsSection("memory")?.label).toBe("Memory");
    expect(findSettingsSection("nope")).toBeUndefined();
    expect(findSettingsSection(null)).toBeUndefined();
  });
});
