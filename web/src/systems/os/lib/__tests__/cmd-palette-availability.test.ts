// Suite: cmd-palette client context evaluator
// Invariant: daemon-declared predicates resolve against the client's volatile
// snapshot with the same semantics the daemon uses; a missing snapshot never
// defaults to allow-all; reasons render verbatim with one honest generic
// fallback; and the hidden-vs-disabled split follows whether the failing key
// describes the surface or a passing state (US-037).
// Boundary IN: predicate matching, the availability verdict, snapshot building.
// Boundary OUT: hydration, debouncing, rendering, and dispatch.
import { describe, expect, it } from "vitest";

import {
  ATTACHED_SHELL_REASON,
  GENERIC_UNAVAILABLE_REASON,
  RUNTIME_UNAVAILABLE_REASON,
  predicateMatches,
  resolvePaletteAvailability,
} from "../cmd-palette-availability";
import {
  buildCmdPaletteContextSnapshot,
  type CmdPaletteContextSnapshot,
} from "../cmd-palette-context";
import type { CmdPaletteStructuralCommand } from "../cmd-palette-types";

function command(
  overrides: Partial<CmdPaletteStructuralCommand> = {}
): CmdPaletteStructuralCommand {
  return {
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
    ...overrides,
  } as CmdPaletteStructuralCommand;
}

function snapshot(overrides: Partial<Record<string, unknown>> = {}): CmdPaletteContextSnapshot {
  return {
    "window.focused": true,
    "window.floating": false,
    "window.stacked": false,
    "desktop.windowCount": 1,
    "scope.global": false,
    "shell.desktop": false,
    "session.focused.state": "",
    "workspace.trusted": true,
    ...overrides,
  } as CmdPaletteContextSnapshot;
}

const reachable = { daemonReachable: true };

describe("cmd-palette predicate matching", () => {
  it("Should default to equality and support the daemon's other two operators", () => {
    const context = snapshot({ "desktop.windowCount": 3 });
    expect(predicateMatches({ key: "window.focused", value: true }, context)).toBe(true);
    expect(
      predicateMatches({ key: "window.floating", operator: "not_equals", value: true }, context)
    ).toBe(true);
    expect(
      predicateMatches(
        { key: "desktop.windowCount", operator: "greater_than_or_equal", value: 2 },
        context
      )
    ).toBe(true);
    expect(
      predicateMatches(
        { key: "desktop.windowCount", operator: "greater_than_or_equal", value: 4 },
        context
      )
    ).toBe(false);
  });

  it("Should refuse an unknown operator, an absent key, and a missing snapshot", () => {
    expect(
      predicateMatches({ key: "window.focused", operator: "matches", value: true }, snapshot())
    ).toBe(false);
    expect(predicateMatches({ key: "nonsense.key", value: true }, snapshot())).toBe(false);
    expect(predicateMatches({ key: "window.focused", value: true }, null)).toBe(false);
  });
});

describe("cmd-palette availability (UT-096, UT-097, UT-098, UT-100, UT-101)", () => {
  it("Should disable action commands with the runtime reason when the daemon is unreachable, keeping exempt commands live [UT-096]", () => {
    const unreachable = { daemonReachable: false };
    expect(resolvePaletteAvailability(command(), snapshot(), unreachable)).toEqual({
      visible: true,
      available: false,
      reason: RUNTIME_UNAVAILABLE_REASON,
    });
    const cheatsheet = command({ id: "shortcuts.cheatsheet", availability_exempt: true });
    expect(resolvePaletteAvailability(cheatsheet, snapshot(), unreachable).available).toBe(true);
  });

  it("Should hide a command whose failing predicate describes the surface, not the moment [UT-097]", () => {
    const desktopOnly = command({
      id: "window.move",
      when: [{ key: "shell.desktop", value: true, reason: ATTACHED_SHELL_REASON }],
    });
    // A browser client will never grow a desktop shell mid-session, so the row
    // is fully irrelevant here rather than permanently disabled.
    expect(resolvePaletteAvailability(desktopOnly, snapshot(), reachable).visible).toBe(false);
    expect(
      resolvePaletteAvailability(desktopOnly, snapshot({ "shell.desktop": true }), reachable)
    ).toEqual({ visible: true, available: true, reason: "" });
  });

  it("Should keep a partially relevant command visible with the daemon's reason verbatim [UT-098]", () => {
    const mergeAll = command({
      id: "window.merge_all",
      when: [
        {
          key: "desktop.windowCount",
          operator: "greater_than_or_equal",
          value: 2,
          reason: "needs two windows on this desktop",
        },
      ],
    });
    expect(resolvePaletteAvailability(mergeAll, snapshot(), reachable)).toEqual({
      visible: true,
      available: false,
      reason: "needs two windows on this desktop",
    });
  });

  it("Should fall back to the honest generic reason rather than inventing a specific [UT-098]", () => {
    const unexplained = command({ when: [{ key: "window.focused", value: true }] });
    expect(
      resolvePaletteAvailability(unexplained, snapshot({ "window.focused": false }), reachable)
        .reason
    ).toBe(GENERIC_UNAVAILABLE_REASON);
  });

  it("Should flip availability when the focused window changes [UT-100]", () => {
    const focused = command({
      when: [{ key: "window.focused", value: true, reason: "requires a focused window" }],
    });
    expect(resolvePaletteAvailability(focused, snapshot(), reachable).available).toBe(true);
    expect(
      resolvePaletteAvailability(focused, snapshot({ "window.focused": false }), reachable)
    ).toEqual({
      visible: true,
      available: false,
      reason: "requires a focused window",
    });
  });

  it("Should never default to allow-all without a context snapshot [UT-101]", () => {
    // Client-executed commands need a client; tool commands run in the daemon
    // and are judged only by their own predicates.
    expect(resolvePaletteAvailability(command(), null, reachable)).toEqual({
      visible: true,
      available: false,
      reason: ATTACHED_SHELL_REASON,
    });
    const tool = command({
      id: "ext.notes.capture",
      action: { kind: "tool", tool: "ext.notes.capture" },
      when: [{ key: "window.focused", value: true, reason: "requires a focused window" }],
    });
    expect(resolvePaletteAvailability(tool, null, reachable)).toEqual({
      visible: true,
      available: false,
      reason: ATTACHED_SHELL_REASON,
    });
    const unconditionalTool = command({
      id: "ext.notes.list",
      action: { kind: "tool", tool: "ext.notes.list" },
    });
    expect(resolvePaletteAvailability(unconditionalTool, null, reachable).available).toBe(true);
  });
});

describe("cmd-palette context snapshot", () => {
  it("Should answer the closed key set from live window-manager state", () => {
    const built = buildCmdPaletteContextSnapshot(
      {
        activeDesktopId: "desk-1",
        focusedId: "win-a",
        frames: {
          "desk-1": [{ id: "frame-1", members: ["win-a", "win-b"], activeWindowId: "win-a" }],
        },
        windows: {
          "win-a": { id: "win-a", desktopId: "desk-1", minimized: false, placement: "floating" },
          "win-b": { id: "win-b", desktopId: "desk-1", minimized: false, placement: "tiled" },
          "win-c": { id: "win-c", desktopId: "desk-2", minimized: false, placement: "tiled" },
          "win-d": { id: "win-d", desktopId: "desk-1", minimized: true, placement: "tiled" },
        },
      } as never,
      {
        shellDesktop: false,
        scopeGlobal: true,
        workspaceTrusted: true,
        focusedSessionState: "waiting-for-input",
      }
    );
    expect(built).toEqual({
      "window.focused": true,
      "window.floating": true,
      "window.stacked": true,
      // Minimized windows and other desktops do not count, matching the daemon.
      "desktop.windowCount": 2,
      "scope.global": true,
      "shell.desktop": false,
      "session.focused.state": "waiting-for-input",
      "workspace.trusted": true,
    });
  });
});
