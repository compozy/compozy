// Suite: cmd-palette cold-path section assembly
// Invariant: while rank signals are null, hyphenated queries still match
// spaced titles, a separators-only query is not an empty search, and equal
// titles sort by command id.
// Owning layer: web/src/systems/os/lib/cmd-palette-sections.ts
// Canonical suite: this file. No prior lib suite owned the signals-null path.
import { describe, expect, it } from "vitest";

import { PALETTE_AGENT_FALLBACK_VALUE, assemblePaletteResults } from "../cmd-palette-sections";
import type { PaletteRegistry, ResolvedPaletteCommand } from "../cmd-palette-types";

function command(
  overrides: Partial<ResolvedPaletteCommand> & Pick<ResolvedPaletteCommand, "id" | "title">
): ResolvedPaletteCommand {
  return {
    section: "Commands",
    icon: "command",
    source: "core",
    bindings: [],
    alias: null,
    destructive: false,
    availability_exempt: false,
    arguments: [],
    action: { kind: "client_op", op: overrides.id },
    execution: { retry_safe: true, single_flight: false },
    visible: true,
    available: true,
    reason: "",
    chords: [],
    ...overrides,
  } satisfies ResolvedPaletteCommand;
}

function registry(commands: readonly ResolvedPaletteCommand[]): PaletteRegistry {
  return {
    commands,
    byId: new Map(commands.map(entry => [entry.id, entry])),
    sources: [{ source: "core", status: "healthy" }],
    catalogRevision: "sha256:test",
    stale: false,
    daemonReachable: true,
  };
}

function assemble(
  commands: readonly ResolvedPaletteCommand[],
  query: string,
  fallbackAgentEnabled = false
) {
  return assemblePaletteResults({
    registry: registry(commands),
    query,
    destination: false,
    signals: null,
    fallbackAgentEnabled,
  });
}

describe("cmd-palette cold-path sections", () => {
  it("Should match a hyphenated query to a spaced title while rank signals load", () => {
    const result = assemble(
      [command({ id: "window.tab.new", title: "New tab", section: "Tabs" })],
      "new-tab"
    );
    expect(result.sections).toEqual([
      {
        title: "Tabs",
        commands: [expect.objectContaining({ id: "window.tab.new" })],
        total: 1,
      },
    ]);
    expect(result.fallback).toBeNull();
  });

  it("Should treat a separators-only query as a miss, not an empty catalog", () => {
    const result = assemble(
      [command({ id: "window.tab.new", title: "New tab", section: "Tabs" })],
      "---",
      true
    );
    expect(result.sections).toEqual([]);
    expect(result.fallback).toEqual({
      value: PALETTE_AGENT_FALLBACK_VALUE,
      query: "---",
    });
  });

  it("Should break title ties by command id on the cold path", () => {
    const result = assemble(
      [
        command({ id: "b-open", title: "Open", section: "Commands" }),
        command({ id: "a-open", title: "Open", section: "Commands" }),
      ],
      ""
    );
    expect(result.sections[0]?.commands.map(entry => entry.id)).toEqual(["a-open", "b-open"]);
  });
});
