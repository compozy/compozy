// Suite: shortcut source grouping
// Invariant: core reads first, extensions follow alphabetically, curated
// sections keep their published order, and unknown sections are preserved.
// Owning layer: web/src/systems/os/lib/shortcut-source-groups.ts
// Canonical suite: this file.
import { describe, expect, it } from "vitest";

import {
  groupShortcutRowsBySource,
  orderShortcutSections,
  orderShortcutSources,
  shortcutSourceLabel,
} from "../shortcut-source-groups";
import type { ShortcutCheatsheetRow } from "../window-manager-shortcuts";

function row(
  overrides: Pick<ShortcutCheatsheetRow, "id" | "label" | "section" | "source"> &
    Partial<ShortcutCheatsheetRow>
): ShortcutCheatsheetRow {
  return {
    actionIds: [overrides.id],
    alias: null,
    bindings: [],
    overridden: false,
    ...overrides,
  };
}

describe("shortcut source grouping", () => {
  it("Should label core and extension sources", () => {
    expect(shortcutSourceLabel("core")).toBe("Core areas");
    expect(shortcutSourceLabel("ext.notes")).toBe("notes");
    expect(shortcutSourceLabel("custom")).toBe("custom");
  });

  it("Should keep core first and sort remaining sources alphabetically", () => {
    expect(orderShortcutSources(["ext.zeta", "core", "ext.alpha"])).toEqual([
      "core",
      "ext.alpha",
      "ext.zeta",
    ]);
    expect(orderShortcutSources(["ext.zeta", "ext.alpha"])).toEqual(["ext.alpha", "ext.zeta"]);
  });

  it("Should keep curated section order and append unknown sections", () => {
    expect(orderShortcutSections(["Notes", "Tabs", "Window", "Custom"])).toEqual([
      "Window",
      "Tabs",
      "Notes",
      "Custom",
    ]);
  });

  it("Should group extension-only rows and preserve unknown sections", () => {
    const groups = groupShortcutRowsBySource([
      row({ id: "ext.notes.capture", label: "Capture", section: "Notes", source: "ext.notes" }),
      row({ id: "ext.notes.window", label: "Pin note", section: "Window", source: "ext.notes" }),
    ]);
    expect(groups).toEqual([
      {
        source: "ext.notes",
        label: "notes",
        sections: [
          {
            title: "Window",
            rows: [expect.objectContaining({ id: "ext.notes.window", section: "Window" })],
          },
          {
            title: "Notes",
            rows: [expect.objectContaining({ id: "ext.notes.capture", section: "Notes" })],
          },
        ],
      },
    ]);
  });
});
