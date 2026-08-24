// Suite: cmd-palette registry projection
// Invariant: one projection turns the daemon's structural catalog plus this
// client's context into what every surface renders — chords formatted from the
// daemon keymap (never a TypeScript literal), hidden commands omitted, disabled
// commands kept with their verbatim reason, and one entry per id.
// Boundary IN: the projection's assembly and chord formatting.
// Boundary OUT: predicate semantics (availability suite), hydration, dispatch.
import { describe, expect, it } from "vitest";

import { buildPaletteRegistry, EMPTY_PALETTE_REGISTRY } from "../cmd-palette-registry";
import type { CmdPaletteStructuralCatalog } from "../cmd-palette-types";

function catalog(
  commands: readonly Partial<CmdPaletteStructuralCatalog["commands"][number]>[]
): CmdPaletteStructuralCatalog {
  return {
    catalogRevision: "sha256:test",
    sources: [{ source: "core", status: "healthy" }],
    commands: commands.map(command => ({
      id: "window.close",
      title: "Close window",
      section: "Window",
      icon: "x-square",
      source: "core",
      bindings: [],
      alias: null,
      destructive: false,
      availability_exempt: false,
      arguments: [],
      action: { kind: "client_op", op: "window.close" },
      execution: { retry_safe: false, single_flight: true },
      ...command,
    })) as CmdPaletteStructuralCatalog["commands"],
  };
}

const context = {
  "window.focused": true,
  "window.floating": false,
  "window.stacked": false,
  "desktop.windowCount": 1,
  "scope.global": false,
  "shell.desktop": false,
  "session.focused.state": "",
  "workspace.trusted": true,
} as const;

const base = { context, daemonReachable: true, stale: false, platform: "MacIntel" };

describe("cmd-palette registry projection", () => {
  it("Should format every chord from the daemon keymap", () => {
    const registry = buildPaletteRegistry({
      ...base,
      catalog: catalog([{ bindings: ["meta+KeyW", "control+shift+KeyW"] }]),
    });
    expect(registry.byId.get("window.close")?.chords).toEqual(["⌘W", "⌃⇧W"]);
  });

  it("Should follow the platform's primary modifier rather than hardcoding ⌘", () => {
    const registry = buildPaletteRegistry({
      ...base,
      platform: "Linux x86_64",
      catalog: catalog([{ bindings: ["meta+KeyW"] }]),
    });
    expect(registry.byId.get("window.close")?.chords).toEqual(["⌃W"]);
  });

  it("Should leave an unbound command without a chord instead of inventing one", () => {
    const registry = buildPaletteRegistry({ ...base, catalog: catalog([{}]) });
    expect(registry.byId.get("window.close")?.chords).toEqual([]);
  });

  it("Should omit hidden commands and keep disabled ones with their reason", () => {
    const registry = buildPaletteRegistry({
      ...base,
      catalog: catalog([
        {
          id: "window.move",
          when: [{ key: "shell.desktop", value: true, reason: "requires an attached shell" }],
        },
        {
          id: "window.merge_all",
          title: "Merge all windows",
          when: [
            {
              key: "desktop.windowCount",
              operator: "greater_than_or_equal",
              value: 2,
              reason: "needs two windows on this desktop",
            },
          ],
        },
      ]),
    });
    expect(registry.byId.has("window.move")).toBe(false);
    expect(registry.byId.get("window.merge_all")).toMatchObject({
      available: false,
      reason: "needs two windows on this desktop",
    });
    expect(registry.commands).toHaveLength(1);
  });

  it("Should report an unhydrated catalog as empty rather than guessing", () => {
    const registry = buildPaletteRegistry({ ...base, catalog: null, stale: true });
    expect(registry.commands).toEqual(EMPTY_PALETTE_REGISTRY.commands);
    expect(registry.stale).toBe(true);
    expect(registry.daemonReachable).toBe(true);
  });

  it("Should expose the Profiles view and the stable profile.* actions (UT-096)", () => {
    // Membership is the daemon's; this asserts the projection keeps the ids and
    // routes them by kind — switch is a client op, every mutation navigates to
    // the canonical lifecycle flow rather than executing here.
    const registry = buildPaletteRegistry({
      ...base,
      catalog: catalog([
        {
          id: "palette.view.profiles",
          title: "Profiles",
          section: "Views",
          action: { kind: "view", view: "profiles" },
        },
        {
          id: "profile.use",
          title: "Use profile",
          section: "Profiles",
          action: { kind: "client_op", op: "profile.use" },
        },
        {
          id: "profile.archive",
          title: "Archive profile",
          section: "Profiles",
          action: {
            kind: "navigate",
            app: "settings",
            args: { pathname: "/settings/profiles", flow: "archive" },
          },
        },
      ]),
    });
    expect(registry.byId.get("palette.view.profiles")).toMatchObject({
      available: true,
      action: { kind: "view", view: "profiles" },
    });
    expect(registry.byId.get("profile.use")).toMatchObject({
      available: true,
      action: { kind: "client_op", op: "profile.use" },
    });
    expect(registry.byId.get("profile.archive")?.action).toMatchObject({
      kind: "navigate",
      app: "settings",
      args: { pathname: "/settings/profiles", flow: "archive" },
    });
  });

  it("Should keep an unavailable profile row with its verbatim reason (UT-096)", () => {
    const registry = buildPaletteRegistry({
      ...base,
      catalog: catalog([
        {
          id: "profile.use",
          title: "Use profile",
          section: "Profiles",
          action: { kind: "client_op", op: "profile.use" },
          when: [
            {
              key: "profile.available",
              operator: "equals",
              value: true,
              reason: "needs setup",
            },
          ],
        },
      ]),
    });
    expect(registry.byId.get("profile.use")).toMatchObject({
      available: false,
      reason: "needs setup",
    });
  });
});
