import { describe, expect, it } from "vitest";

import {
  retiredSettingsSectionReplacement,
  settingsRouteWithMovedShortcutCommand,
} from "../section-paths";
import {
  filterSettingsSections,
  findSettingsSection,
  SETTINGS_ROOT_PATH,
  SETTINGS_SECTIONS,
  settingsSectionPath,
  settingsSectionSlugFromPathname,
} from "../sections";

describe("settings sections metadata", () => {
  it("exposes the Settings screen order", () => {
    expect(SETTINGS_SECTIONS.map(section => section.slug)).toEqual([
      "general",
      "terminal",
      "appearance",
      "layouts",
      "profiles",
      "providers",
      "provider-usage",
      "roles",
      "skills",
      "mcp",
      "automation",
      "gateway",
      "shortcuts",
      "palette",
      "attention",
      "observability",
      "hooks",
      "extensions",
      "marketplace",
    ]);
  });

  it("groups operator-facing sections together", () => {
    // Shortcuts, Palette, Attention and Observability are about how the operator
    // works, not how the workspace or the runtime is configured.
    const operator = SETTINGS_SECTIONS.filter(section => section.group === "operator").map(
      section => section.slug
    );
    expect(operator).toEqual(["shortcuts", "palette", "attention", "observability"]);
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
    expect(findSettingsSection("roles")?.label).toBe("Roles");
    expect(findSettingsSection("terminal")?.label).toBe("Terminal");
    expect(findSettingsSection("nope")).toBeUndefined();
    expect(findSettingsSection(null)).toBeUndefined();
  });

  it("Should find Terminal by the keys an administrator would search", () => {
    expect(filterSettingsSections("terminal").map(section => section.slug)).toContain("terminal");
    expect(filterSettingsSections("scrollback").map(section => section.slug)).toEqual(["terminal"]);
    expect(filterSettingsSections("recording").map(section => section.slug)).toEqual(["terminal"]);
  });

  it("Should land every session-defaults search on General", () => {
    // Invariant: the retired Defaults page's search terms now find General.
    for (const query of [
      "defaults",
      "agent",
      "provider",
      "model",
      "reasoning",
      "smooth streaming",
    ]) {
      expect(filterSettingsSections(query).map(section => section.slug)).toContain("general");
    }
    expect(findSettingsSection("defaults")).toBeUndefined();
  });

  it("Should resolve the retired Defaults slug to General and nothing else", () => {
    expect(retiredSettingsSectionReplacement("defaults")).toBe("general");
    expect(retiredSettingsSectionReplacement("general")).toBeNull();
    expect(retiredSettingsSectionReplacement("toString")).toBeNull();
  });

  it("Should resolve a settings window route to its section, following retired and unknown slugs", () => {
    expect(settingsSectionSlugFromPathname("/settings/providers")).toBe("providers");
    expect(settingsSectionSlugFromPathname("/settings/defaults")).toBe("general");
    expect(settingsSectionSlugFromPathname("/settings")).toBe("general");
    expect(settingsSectionSlugFromPathname("/settings/not-a-section")).toBe("general");
    expect(settingsSectionSlugFromPathname("/settings/constructor")).toBe("general");
  });

  it("Should move a Layouts route that names a command to Shortcuts and keep plain Layouts", () => {
    // Invariant: the shortcut table moved; old command links land on it, Layouts stays Layouts.
    expect(
      settingsRouteWithMovedShortcutCommand({
        pathname: "/settings/layouts",
        search: { command: "palette.open", extra: 1 },
      })
    ).toEqual({ pathname: "/settings/shortcuts", search: { command: "palette.open", extra: 1 } });
    expect(
      settingsRouteWithMovedShortcutCommand({ pathname: "/settings/layouts", search: {} })
    ).toBeNull();
    expect(
      settingsRouteWithMovedShortcutCommand({
        pathname: "/settings/layouts",
        search: { command: " " },
      })
    ).toBeNull();
    expect(
      settingsRouteWithMovedShortcutCommand({
        pathname: "/settings/shortcuts",
        search: { command: "palette.open" },
      })
    ).toBeNull();
  });
});
