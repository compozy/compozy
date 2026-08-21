// Suite: cmd-palette action-panel row model
// Invariant: the panel offers exactly what the selected row can actually do —
// meta-actions on command rows (the only rows carrying an id to pin, alias or
// bind), domain actions on entity rows, nothing runnable on an unavailable
// command beyond its verbatim reason, and chords borrowed from the registry
// rather than spelled in TypeScript. Filtering narrows without inventing a
// fallback, and the highlighted value always resolves back to its own row.
// Owning layer: the pure row-action model.
// Boundary OUT: rendering and keyboard dispatch (palette-root and
// os-interaction-hooks suites), and running an intent (dispatch seam).
import { describe, expect, it } from "vitest";

import type { OsPaletteDomainRow } from "../../hooks/use-os-palette-domain-search";
import type {
  OsPaletteSessionResult,
  OsPaletteTabResult,
  OsPaletteWorktreeResult,
} from "../../hooks/use-os-palette-entities";
import {
  PALETTE_META_SECTION,
  filterRowActions,
  flattenRowActions,
  paletteRowActions,
  primaryRowAction,
  resolvePaletteRowSubject,
} from "../cmd-palette-row-actions";
import type { PaletteRegistry, ResolvedPaletteCommand } from "../cmd-palette-types";

function command(overrides: Partial<ResolvedPaletteCommand> = {}): ResolvedPaletteCommand {
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
    visible: true,
    available: true,
    reason: "",
    chords: [],
    ...overrides,
  } as ResolvedPaletteCommand;
}

function registryFixture(commands: readonly ResolvedPaletteCommand[]): PaletteRegistry {
  return {
    commands,
    byId: new Map(commands.map(entry => [entry.id, entry])),
    sources: [{ source: "core", status: "healthy" }],
    catalogRevision: "sha256:test",
    stale: false,
    daemonReachable: true,
  };
}

const SETTINGS_LAYOUTS = command({
  id: "settings.layouts",
  title: "Settings → Layouts",
  section: "Settings",
  icon: "panel-top",
  action: { kind: "navigate", app: "settings", args: { pathname: "/settings/layouts" } },
});

const SESSION: OsPaletteSessionResult = {
  sessionId: "session:refactor",
  title: "Refactor session store",
  agentName: "claude",
  workspaceId: "workspace:alpha",
  route: { pathname: "/agents/claude/sessions/session:refactor", search: {} },
};

const TAB: OsPaletteTabResult = {
  windowId: "window:tasks",
  app: "tasks",
  label: "Tasks",
  desktopName: "Alpha",
  needsInput: false,
  minimized: false,
};

const DOMAIN_ROW: OsPaletteDomainRow = {
  key: "task:tk_1",
  label: "Ship the palette",
  app: "tasks",
  route: { pathname: "/tasks/tk_1", search: {} },
};

function worktree(overrides: Partial<OsPaletteWorktreeResult> = {}): OsPaletteWorktreeResult {
  return {
    key: "worktree:acme",
    kind: "worktree",
    displayState: "ready",
    name: "acme",
    branch: "main",
    path: "/tmp/acme",
    activityAt: null,
    selectable: true,
    inertReason: null,
    adoptable: false,
    worktree: { id: "wt_1" },
    ...overrides,
  } as OsPaletteWorktreeResult;
}

describe("cmd-palette row actions — command rows (UT-125, UT-126)", () => {
  it("Should carry the meta-actions on every command row", () => {
    const target = command({ id: "session.new", title: "New session" });
    const model = paletteRowActions({
      subject: { kind: "command", command: target },
      registry: registryFixture([target, SETTINGS_LAYOUTS]),
      pins: [],
    });
    const titles = flattenRowActions(model.sections).map(action => action.title);
    expect(titles).toContain("Pin");
    expect(titles).toContain("Set alias…");
    expect(titles).toContain("Set shortcut…");
    expect(model.sections.at(-1)?.title).toBe(PALETTE_META_SECTION);
  });

  it("Should mark the row's own action primary and lend it the registry chord", () => {
    const target = command({
      id: "window.tab.new",
      title: "New tab",
      chords: ["⌘T"],
      bindings: ["meta+KeyT"],
    });
    const model = paletteRowActions({
      subject: { kind: "command", command: target },
      registry: registryFixture([target]),
      pins: [],
    });
    const primary = primaryRowAction(model);
    expect(primary?.title).toBe("New tab");
    expect(primary?.chords).toEqual(["⌘T"]);
    expect(primary?.bindings).toEqual(["meta+KeyT"]);
    expect(primary?.intent).toEqual({ kind: "run-command", commandId: "window.tab.new" });
  });

  it("Should offer Unpin once the command is pinned", () => {
    const target = command({ id: "session.new" });
    const pinned = paletteRowActions({
      subject: { kind: "command", command: target },
      registry: registryFixture([target]),
      pins: ["session.new"],
    });
    const action = flattenRowActions(pinned.sections).find(entry => entry.id === "meta.pin");
    expect(action?.title).toBe("Unpin");
    expect(action?.intent).toEqual({ kind: "pin", commandId: "session.new", pinned: false });
  });

  it("Should drop the settings deep-links when the catalog cannot serve that destination", () => {
    const target = command({ id: "session.new" });
    const model = paletteRowActions({
      subject: { kind: "command", command: target },
      registry: registryFixture([target]),
      pins: [],
    });
    const ids = flattenRowActions(model.sections).map(action => action.id);
    expect(ids).toContain("meta.pin");
    expect(ids).not.toContain("meta.alias");
    expect(ids).not.toContain("meta.shortcut");
  });
});

describe("cmd-palette row actions — unavailable command (UT-128)", () => {
  it("Should list meta-actions and the verbatim reason with nothing runnable", () => {
    const target = command({
      id: "ext.notes.capture",
      title: "Capture note",
      available: false,
      reason: "extension notes is unhealthy (crash loop)",
    });
    const model = paletteRowActions({
      subject: { kind: "command", command: target },
      registry: registryFixture([target, SETTINGS_LAYOUTS]),
      pins: [],
    });
    expect(model.available).toBe(false);
    expect(model.reason).toBe("extension notes is unhealthy (crash loop)");
    expect(primaryRowAction(model)).toBeNull();
    expect(model.sections.map(section => section.title)).toEqual([PALETTE_META_SECTION]);
  });
});

describe("cmd-palette row actions — entity rows (UT-126)", () => {
  it("Should give a session row its landing action and no command meta-actions", () => {
    const model = paletteRowActions({
      subject: { kind: "session", session: SESSION },
      registry: registryFixture([SETTINGS_LAYOUTS]),
      pins: [],
    });
    expect(model.key).toBe("session:session:refactor");
    expect(flattenRowActions(model.sections).map(action => action.title)).toEqual(["Land session"]);
    expect(primaryRowAction(model)?.intent).toEqual({ kind: "land-session", session: SESSION });
  });

  it("Should style removing a worktree as destructive and borrow no chord for it", () => {
    const entry = worktree();
    const model = paletteRowActions({
      subject: { kind: "worktree", entry },
      registry: registryFixture([SETTINGS_LAYOUTS]),
      pins: [],
    });
    const remove = flattenRowActions(model.sections).find(
      action => action.id === "worktree.remove"
    );
    expect(remove?.destructive).toBe(true);
    expect(remove?.chords).toEqual([]);
    expect(remove?.intent).toEqual({ kind: "remove-worktree", entry });
  });

  it("Should not offer removing a worktree that was never adopted", () => {
    const entry = worktree({ kind: "discovered", worktree: null });
    const model = paletteRowActions({
      subject: { kind: "worktree", entry },
      registry: registryFixture([]),
      pins: [],
    });
    expect(flattenRowActions(model.sections).map(action => action.id)).toEqual(["worktree.scope"]);
  });

  it("Should lend a tab's close action the window-close chord from the registry", () => {
    const closeCommand = command({ chords: ["⌘W"], bindings: ["meta+KeyW"] });
    const model = paletteRowActions({
      subject: { kind: "tab", tab: TAB },
      registry: registryFixture([closeCommand]),
      pins: [],
    });
    const close = flattenRowActions(model.sections).find(action => action.id === "tab.close");
    expect(close?.chords).toEqual(["⌘W"]);
    expect(close?.bindings).toEqual(["meta+KeyW"]);
  });
});

describe("cmd-palette row action filtering (UT-125)", () => {
  const target = command({ id: "session.new", title: "New session" });
  const model = paletteRowActions({
    subject: { kind: "command", command: target },
    registry: registryFixture([target, SETTINGS_LAYOUTS]),
    pins: [],
  });

  it("Should narrow to matching actions and collapse the sections they left", () => {
    const filtered = filterRowActions(model.sections, "pin");
    expect(filtered).toHaveLength(1);
    expect(flattenRowActions(filtered).map(action => action.title)).toEqual(["Pin"]);
  });

  it("Should return nothing rather than a fallback when the filter matches no action", () => {
    expect(filterRowActions(model.sections, "xyz")).toEqual([]);
  });

  it("Should keep every action when the filter is blank", () => {
    expect(filterRowActions(model.sections, "  ")).toBe(model.sections);
  });
});

describe("cmd-palette row subject resolution", () => {
  const sources = {
    commands: [command({ id: "session.new" })],
    sessions: [SESSION],
    tabs: [TAB],
    worktrees: [worktree()],
    domainRows: [DOMAIN_ROW],
  };

  it("Should resolve every row kind back from its list value", () => {
    expect(resolvePaletteRowSubject(sources, "session.new")?.kind).toBe("command");
    expect(resolvePaletteRowSubject(sources, "session:session:refactor")?.kind).toBe("session");
    expect(resolvePaletteRowSubject(sources, "tab:window:tasks")?.kind).toBe("tab");
    expect(resolvePaletteRowSubject(sources, "worktree:worktree:acme")?.kind).toBe("worktree");
    expect(resolvePaletteRowSubject(sources, "task:tk_1")?.kind).toBe("domain");
  });

  it("Should resolve a value the lists no longer carry to nothing", () => {
    expect(resolvePaletteRowSubject(sources, "session:gone")).toBeNull();
  });
});
