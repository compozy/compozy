// Suite: window-manager settings contract
// Invariant: daemon settings enter the shared Query cache only after strict runtime validation.
// Boundary IN: window-manager settings section parsing — keymap, registry, aliases, diagnostics.
// Boundary OUT: HTTP transport and Settings editor mutation.
import { describe, expect, it } from "vitest";

import {
  parseSettingsWindowManagerSection,
  type WindowManagerSettingsWire,
} from "../window-manager-settings-section";

function settingsResponse(): WindowManagerSettingsWire {
  return {
    section: "window-manager",
    scope: "workspace",
    workspace_id: "workspace:alpha",
    available_scopes: ["global", "workspace"],
    aliases: { "ext.notes.capture": "cap" },
    commands: [
      {
        id: "desktop.switch.next",
        title: "Next desktop",
        section: "Desktops",
        source: "core",
      },
      {
        id: "ext.notes.capture",
        title: "Capture note",
        section: "Notes",
        source: "ext.notes",
      },
    ],
    extension_defaults: [
      {
        command: "ext.notes.recent",
        binding: ["meta+KeyN"],
        dormant: true,
        conflict_with: "session.new",
      },
    ],
    config: {
      new_window_policy: "floating",
      small_viewport_policy: "stack",
      focus_policy: "click_directional",
      focus_wrap: true,
      focus_follows_pointer: false,
      raise_on_focus: true,
      drag_away_policy: "window",
      group_move_modifier: "alt",
      swap_modifier: "shift",
      history_limit: 100,
      nav_stack_limit: 75,
      closed_entry_limit: 30,
      desktop_transition: "slide",
      gaps: { inner: 8, top: 8, right: 8, bottom: 8, left: 8 },
      snap: {
        edge_band: 24,
        corner_reach: 96,
        exit_slack: 16,
        repeat_ratios: [0.5, 0.33, 0.67],
      },
      bindings: { top_center: "zoom", bottom_center: "none" },
      shortcuts: {
        "desktop.switch.next": "control+alt+BracketRight",
      },
      global_shortcuts: { "palette.summon.global": "meta+shift+Space" },
    },
    defaults: {
      "desktop.switch.next": ["control+shift+ArrowRight"],
      "window.focus.left": "control+ArrowLeft",
    },
    effective_shortcuts: {
      "desktop.switch.next": ["control+alt+BracketRight", "alt+KeyL"],
      "window.focus.left": "control+ArrowLeft",
    },
    global_shortcuts: [
      {
        command_id: "palette.summon.global",
        intended_chord: "meta+shift+Space",
        active_chord: "meta+shift+Space",
        status: "registered",
      },
    ],
  };
}

describe("parseSettingsWindowManagerSection", () => {
  it("Should project the registry, aliases and withheld extension defaults", () => {
    const section = parseSettingsWindowManagerSection(settingsResponse());

    expect(section.scope).toBe("workspace");
    expect(section.workspaceId).toBe("workspace:alpha");
    expect(section.availableScopes).toEqual(["global", "workspace"]);
    expect(section.commands.map(command => command.id)).toEqual([
      "desktop.switch.next",
      "ext.notes.capture",
    ]);
    expect(section.commands[1]).toEqual({
      id: "ext.notes.capture",
      title: "Capture note",
      section: "Notes",
      source: "ext.notes",
    });
    expect(section.aliases).toEqual({ "ext.notes.capture": "cap" });
    expect(section.extensionDefaults).toEqual([
      {
        commandId: "ext.notes.recent",
        binding: ["meta+KeyN"],
        dormant: true,
        conflictWith: "session.new",
      },
    ]);
    expect(section.diagnostics).toEqual([]);
    expect(section.globalShortcuts).toEqual([
      {
        commandId: "palette.summon.global",
        intendedChord: "meta+shift+Space",
        activeChord: "meta+shift+Space",
        status: "registered",
        reason: null,
        settingsUrl: null,
      },
    ]);
  });

  it("Should carry the daemon's diagnostic for an override it could not resolve [UT-074]", () => {
    // A stored override naming a command that no longer exists is reported, not
    // dropped in silence and not turned into a parse failure (US-022.EC-3).
    const response = settingsResponse();
    response.diagnostics = [{ command_id: "ext.gone.command", message: "unknown command id" }];

    expect(parseSettingsWindowManagerSection(response).diagnostics).toEqual([
      { commandId: "ext.gone.command", message: "unknown command id" },
    ]);
  });

  it("Should preserve configured global shortcut intent before the shell reports a status", () => {
    const response = settingsResponse();
    const { status: _status, ...globalShortcut } = response.global_shortcuts[0];

    expect(
      parseSettingsWindowManagerSection({
        ...response,
        global_shortcuts: [globalShortcut],
      }).globalShortcuts
    ).toEqual([
      {
        commandId: "palette.summon.global",
        intendedChord: "meta+shift+Space",
        activeChord: "meta+shift+Space",
        status: "pending",
        reason: null,
        settingsUrl: null,
      },
    ]);
  });

  it("Should reject an envelope from another settings section", () => {
    const response = { ...settingsResponse(), section: "attention" as const };

    expect(() => parseSettingsWindowManagerSection(response)).toThrow();
  });
});

describe("parseSettingsWindowManagerSection.config", () => {
  it("Should project the complete validated global config", () => {
    expect(parseSettingsWindowManagerSection(settingsResponse()).config).toEqual({
      newWindowPolicy: "floating",
      smallViewportPolicy: "stack",
      focusPolicy: "click_directional",
      focusWrap: true,
      focusFollowsPointer: false,
      raiseOnFocus: true,
      dragAwayPolicy: "window",
      groupMoveModifier: "alt",
      swapModifier: "shift",
      historyLimit: 100,
      navStackLimit: 75,
      closedEntryLimit: 30,
      desktopTransition: "slide",
      gaps: { inner: 8, top: 8, right: 8, bottom: 8, left: 8 },
      snap: {
        edgeBand: 24,
        cornerReach: 96,
        exitSlack: 16,
        repeatRatios: [0.5, 0.33, 0.67],
      },
      bindings: { topCenter: "zoom", bottomCenter: "none" },
      shortcuts: { "desktop.switch.next": ["control+alt+BracketRight"] },
      shortcutDefaults: {
        "desktop.switch.next": ["control+shift+ArrowRight"],
        "window.focus.left": ["control+ArrowLeft"],
      },
      effectiveShortcuts: {
        "desktop.switch.next": ["control+alt+BracketRight", "alt+KeyL"],
        "window.focus.left": ["control+ArrowLeft"],
      },
      globalShortcuts: { "palette.summon.global": "meta+shift+Space" },
    });
  });

  it.each([
    {
      label: "history beyond the runtime limit",
      mutate: (response: ReturnType<typeof settingsResponse>) => {
        response.config.history_limit = 501;
      },
    },
    {
      label: "navigation stack beyond the runtime limit",
      mutate: (response: ReturnType<typeof settingsResponse>) => {
        response.config.nav_stack_limit = 201;
      },
    },
    {
      label: "closed entry retention beyond the runtime limit",
      mutate: (response: ReturnType<typeof settingsResponse>) => {
        response.config.closed_entry_limit = 101;
      },
    },
    {
      label: "non-canonical shortcut chord",
      mutate: (response: ReturnType<typeof settingsResponse>) => {
        response.config.shortcuts = { "desktop.switch.next": "BracketRight" };
      },
    },
  ])("Should reject $label", ({ mutate }) => {
    const response = settingsResponse();
    mutate(response);

    expect(() => parseSettingsWindowManagerSection(response)).toThrow();
  });

  it("Should accept a binding for an id this client has not hydrated yet", () => {
    // The bindable id space is open — core plus `ext.*` — so membership is the
    // registry's judgement, not the schema's. Grammar is still enforced above.
    const response = settingsResponse();
    response.config.shortcuts = { "ext.notes.capture": "meta+shift+KeyN" };

    expect(() => parseSettingsWindowManagerSection(response)).not.toThrow();
  });
});
