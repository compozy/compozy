import { describe, expect, it } from "vitest";

import {
  filterSettingsSections,
  findSettingsSection,
  SETTINGS_ROOT_PATH,
  SETTINGS_SECTIONS,
  settingsSectionPath,
} from "../sections";

describe("settings sections metadata", () => {
  it("exposes the Settings screen order", () => {
    expect(SETTINGS_SECTIONS.map(section => section.slug)).toEqual([
      "general",
      "terminal",
      "defaults",
      "appearance",
      "layouts",
      "profiles",
      "palette",
      "providers",
      "memory",
      "roles",
      "skills",
      "automation",
      "network",
      "gateway",
      "attention",
      "observability",
      "hooks",
      "extensions",
    ]);
  });

  it("groups operator-facing sections together", () => {
    // Attention and Observability are about how the operator works, not how the
    // workspace or the runtime is configured.
    const operator = SETTINGS_SECTIONS.filter(section => section.group === "operator").map(
      section => section.slug
    );
    expect(operator).toEqual(["attention", "observability"]);
  });

  it("provides nested paths rooted under the settings shell", () => {
    expect(SETTINGS_ROOT_PATH).toBe("/settings");
    expect(settingsSectionPath("providers")).toBe("/settings/providers");
    expect(settingsSectionPath("layouts")).toBe("/settings/layouts");
    expect(settingsSectionPath("attention")).toBe("/settings/attention");
    expect(settingsSectionPath("hooks")).toBe("/settings/hooks");
    expect(settingsSectionPath("extensions")).toBe("/settings/extensions");
  });

  it("looks sections up by slug", () => {
    expect(findSettingsSection("memory")?.label).toBe("Memory");
    expect(findSettingsSection("terminal")?.label).toBe("Terminal");
    expect(findSettingsSection("nope")).toBeUndefined();
    expect(findSettingsSection(null)).toBeUndefined();
  });

  it("Should find Terminal by the keys an administrator would search", () => {
    expect(filterSettingsSections("terminal").map(section => section.slug)).toContain("terminal");
    expect(filterSettingsSections("scrollback").map(section => section.slug)).toEqual(["terminal"]);
    expect(filterSettingsSections("recording").map(section => section.slug)).toEqual(["terminal"]);
  });
});
