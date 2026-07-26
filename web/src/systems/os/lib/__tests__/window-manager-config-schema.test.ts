// Suite: window-manager settings contract
// Invariant: daemon settings enter the shared Query cache only after strict runtime validation.
// Boundary IN: global window-manager settings wire response parsing.
// Boundary OUT: HTTP transport, Settings editor mutation, and workspace overrides.
import { describe, expect, it } from "vitest";

import { parseSettingsWindowManagerConfig } from "../window-manager-config-schema";

function settingsResponse() {
  return {
    section: "window-manager",
    scope: "global",
    available_scopes: ["global"],
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
      } as Record<string, string>,
    },
  };
}

describe("parseSettingsWindowManagerConfig", () => {
  it("Should project the complete validated global config", () => {
    expect(parseSettingsWindowManagerConfig(settingsResponse())).toEqual({
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
      desktopTransition: "slide",
      gaps: { inner: 8, top: 8, right: 8, bottom: 8, left: 8 },
      snap: {
        edgeBand: 24,
        cornerReach: 96,
        exitSlack: 16,
        repeatRatios: [0.5, 0.33, 0.67],
      },
      bindings: { topCenter: "zoom", bottomCenter: "none" },
      shortcuts: { "desktop.switch.next": "control+alt+BracketRight" },
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
      label: "unknown shortcut action",
      mutate: (response: ReturnType<typeof settingsResponse>) => {
        response.config.shortcuts = { "desktop.teleport": "Meta+KeyT" };
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

    expect(() => parseSettingsWindowManagerConfig(response)).toThrow();
  });
});
