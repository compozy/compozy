import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
// Suite: OS command palette root
// Invariant: every row is a projection of the one client registry, entity rows
// preserve the selected tab's identity, session landing goes through the shared
// attention jump exactly once (BR-20), destination mode offers only navigable
// targets, and a pushed view owns the surface completely — its own results, its
// own filters, and a keyboard selection that survives the catalog moving.
// Owning layer: palette root view-model and presentation boundary.
// Boundary OUT: the dispatch seam and client-op table (cmd-palette-dispatch),
// availability evaluation (cmd-palette-availability), overlay lifetime
// (use-desktop-overlays), stack and filter mechanics (palette-view-stack,
// palette-session-filters), and daemon command transport.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { act, render, renderHook, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  clearChooseSessionTerminalQuote,
  holdChooseSessionTerminalQuote,
  peekChooseSessionTerminalQuote,
} from "@/systems/terminal/parts";
import {
  clearSessionTerminalQuote,
  peekSessionTerminalQuote,
  stageSessionTerminalQuote,
} from "@/systems/session/lib/session-terminal-quote";

import type { LayoutDesktop } from "../../lib/window-manager-types";
import type { OsAppId, OsDesktopRuntimeStore, OsWindow } from "../../lib/os-types";
import { OsCommandPalette } from "../../components/os-command-palette";
import type { CmdPaletteRunOptions } from "../use-cmd-palette-dispatch";
import { useOsPaletteRoot } from "../use-os-palette-root";
import {
  cmdPaletteExecutionStore,
  requestPaletteArgs,
  requestPaletteConfirmation,
  resetPaletteExecutionEntry,
} from "../../stores/cmd-palette-execution-store";
import { CmdPaletteRegistryProvider } from "../../contexts/cmd-palette-registry-context";
import type {
  CmdPaletteRankSignals,
  PaletteRegistry,
  ResolvedPaletteCommand,
} from "../../lib/cmd-palette-types";
import { paletteCommand } from "../../lib/__tests__/cmd-palette-dispatch-fixtures";
import { PALETTE_SESSION_ROW_LIMIT } from "../use-os-palette-sessions-view";
import {
  isPaletteDomainSearchEnabled,
  projectVaultRows,
  type OsPaletteDomainSection,
} from "../use-os-palette-domain-search";

const TEST_WEIGHTS = JSON.parse(
  readFileSync(
    resolve(process.cwd(), "../internal/cmdpalette/testdata/ranking_weights_v1.json"),
    "utf8"
  )
) as CmdPaletteRankSignals["weights"];

const TEST_RANK_SIGNALS: CmdPaletteRankSignals = {
  profile_lens: { profile_lens_id: "00000000000000000000000000", profile_name: "default" },
  weights: TEST_WEIGHTS,
  usage: [],
  query_hits: [],
  pins: [],
  revision: "ps_test",
};

type PaletteSessionFixture = {
  id: string;
  name: string | null;
  agent_name: string;
  badge: string;
  workspace_id?: string;
  updated_at: string;
  attention_changed_at?: string;
  profile_id?: string;
  profile_name?: string;
  profile_color?: string;
  profile_icon?: string;
  profile_emoji?: string;
  profile_archived?: boolean;
};

// The view path is the one piece of palette state a click has to move, so the
// mock is a real subscribable store rather than a value: pushing a view in a
// test re-renders the palette exactly as it does in the shell.
const paletteMocks = vi.hoisted(() => {
  const listeners = new Set<() => void>();
  let viewStack: readonly { viewId: "sessions" }[] = [];
  return {
    activeWorkspaceId: "workspace:alpha" as string | null,
    closeWindow: vi.fn(async () => true),
    setActiveWorkspaceId: vi.fn(),
    coordinator: {
      userActivateWindow: vi.fn(async () => true),
      userOpen: vi.fn(async () => "window:new-tab" as string | null),
    },
    desktop: null as OsDesktopRuntimeStore | null,
    commandAvailable: false,
    isWaiting: vi.fn<(sessionId: string) => boolean>(() => false),
    jumpToSession: vi.fn(),
    notifyUser: vi.fn<(feedback: { message: string; tone: string }) => void>(),
    fallbackAgentEnabled: false,
    fallbackPending: false,
    runFallback: vi.fn(async () => undefined),
    openForAgent: vi.fn(),
    pending: false,
    profileAggregate: false,
    scope: "workspace" as "workspace" | "global",
    paletteIntent: null as { kind: "destination"; windowId: string } | null,
    paletteIntentCleared: vi.fn(),
    paletteIntentRequested: vi.fn(),
    sessionListScope: "workspace" as "workspace" | "all-workspaces",
    sessionListSaving: false,
    sessions: [] as PaletteSessionFixture[],
    sessionsLoading: false,
    sessionsWorkspaceId: vi.fn(),
    sessionsFilters: vi.fn(),
    sessionGroupWorkspaces: vi.fn(),
    setSessionListScope: vi.fn(),
    toggleLocked: false,
    windowCommands: {
      arrangeCommands: [],
      commandsAvailable: true,
      dispatchArrange: vi.fn(),
      dispatchPlacement: vi.fn(),
      focusedWindowActions: null,
      placementCommands: [],
      shortcutLabels: {},
    },
    windowSlots: new Map<string, { crumb?: string }>(),
    workspaceGroups: [] as Array<{
      workspaceId: string;
      workspaceName: string;
      sessions: PaletteSessionFixture[];
      total: number;
      loading: boolean;
      failed: boolean;
      retry: () => void;
    }>,
    rankSignals: null as CmdPaletteRankSignals | null,
    domainWorkspaceNames: vi.fn<(names: ReadonlyMap<string, string>) => void>(),
    domainSections: [] as OsPaletteDomainSection[],
    workspaces: [
      { id: "workspace:alpha", name: "Alpha" },
      { id: "workspace:beta", name: "Beta" },
    ],
    registeredWorkspaces: [
      { id: "workspace:home", name: "Home" },
      { id: "workspace:alpha", name: "Alpha" },
      { id: "workspace:beta", name: "Beta" },
    ],
    readViewStack: () => viewStack,
    subscribeViewStack: (listener: () => void) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    writeViewStack: (next: readonly { viewId: "sessions" }[]) => {
      viewStack = next;
      listeners.forEach(listener => listener());
    },
  };
});

vi.mock("@/systems/session", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/session")>()),
  getSessionDisplayTitle: (session: { id: string; name: string | null }) =>
    session.name ?? session.id,
  useSessionCreateActions: () => ({ openForAgent: paletteMocks.openForAgent }),
  useSessionPromptFallback: () => ({
    pending: paletteMocks.fallbackPending,
    run: paletteMocks.runFallback,
  }),
  useSessions: (workspaceId: string | null, options?: { filters?: Record<string, unknown> }) => {
    paletteMocks.sessionsWorkspaceId(workspaceId);
    paletteMocks.sessionsFilters(options?.filters ?? {});
    return { data: paletteMocks.sessions, isLoading: paletteMocks.sessionsLoading };
  },
  useSessionListPreferences: () => ({
    sort: "last_activity" as const,
    scope: paletteMocks.sessionListScope,
    setSort: vi.fn(),
    setScope: paletteMocks.setSessionListScope,
    loading: false,
    saving: paletteMocks.sessionListSaving,
  }),
  useWorkspaceSessionGroups: (input: { workspaces: ReadonlyArray<{ id: string }> }) => {
    paletteMocks.sessionGroupWorkspaces(input.workspaces);
    return paletteMocks.workspaceGroups;
  },
}));

vi.mock("@/systems/workspace", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/workspace")>()),
  useSelectedWorkspaceId: () => paletteMocks.activeWorkspaceId,
  useWorkspaceScopeMode: () => paletteMocks.scope,
  useWorktrees: () => ({ data: undefined }),
  useWorktreeListings: () => ({}),
  useScopedWorktreeFilter: () => ({ worktreeId: undefined, resolved: true }),
  useActiveWorktree: () => ({ selectedWorktreeId: null, activeWorktree: null, fallback: null }),
  useActiveWorkspace: () => ({
    activeWorkspaceId: paletteMocks.activeWorkspaceId,
    runtimeWorkspaceId: paletteMocks.activeWorkspaceId,
    setActiveWorkspaceId: paletteMocks.setActiveWorkspaceId,
    workspaces: paletteMocks.workspaces,
    registeredWorkspaces: paletteMocks.registeredWorkspaces,
    scope: paletteMocks.scope,
    pending: paletteMocks.pending,
    hasHydrated: true,
    toggleLocked: paletteMocks.toggleLocked,
    canDisableGlobal: true,
    toggleGlobalScope: vi.fn(),
  }),
}));

vi.mock("../../../profiles/hooks/use-profile-read-scope", () => ({
  useProfileReadScope: () => ({
    aggregate: paletteMocks.profileAggregate,
    destination: "default",
    destinationOwner: null,
    key: paletteMocks.profileAggregate ? "@all" : "default",
    lens: { scope: "workspace", workspaceId: "workspace:alpha" },
    ownerOf: (row: PaletteSessionFixture) => ({
      id: row.profile_id ?? "",
      name: row.profile_name ?? "",
      color: row.profile_color,
      icon: row.profile_icon ?? null,
      emoji: row.profile_emoji ?? null,
      archived: row.profile_archived === true,
    }),
    params: paletteMocks.profileAggregate ? { all_profiles: true } : { profile: "default" },
    scopeLabel: paletteMocks.profileAggregate ? null : "default",
    view: paletteMocks.profileAggregate
      ? { kind: "aggregate" }
      : { kind: "profile", profile: "default" },
  }),
  useAggregateDestination: () => (paletteMocks.profileAggregate ? "default" : null),
}));

vi.mock("../use-attention-jump", () => ({
  useAttentionJump: () => paletteMocks.jumpToSession,
}));

vi.mock("@/lib/user-feedback", () => ({
  notifyUser: (feedback: { message: string; tone: string }) => paletteMocks.notifyUser(feedback),
}));

vi.mock("../use-cmd-palette-rank-signals", () => ({
  useCmdPaletteRankSignals: () => ({
    data: paletteMocks.rankSignals,
    loading: false,
    failed: false,
  }),
}));

vi.mock("../use-cmd-palette-fallback-settings", () => ({
  useCmdPaletteFallbackSettings: () => paletteMocks.fallbackAgentEnabled,
}));

vi.mock("../use-os-palette-domain-search", async importOriginal => ({
  ...(await importOriginal<typeof import("../use-os-palette-domain-search")>()),
  useOsPaletteDomainSearch: (input: { workspaceNames: ReadonlyMap<string, string> }) => {
    paletteMocks.domainWorkspaceNames(input.workspaceNames);
    return paletteMocks.domainSections;
  },
}));

vi.mock("../../hooks/use-desktop", () => ({
  useDesktop: <T,>(selector: (state: OsDesktopRuntimeStore) => T) => {
    if (paletteMocks.desktop === null) throw new Error("Desktop fixture was not configured.");
    return selector(paletteMocks.desktop);
  },
}));

vi.mock("../../lib/window-manager-command-availability", () => ({
  windowManagerCommandsAvailable: () => paletteMocks.commandAvailable,
}));

vi.mock("../../hooks/use-os-shell", () => ({
  useOsShell: () => ({
    coordinator: paletteMocks.coordinator,
    manager: {
      closeWindow: paletteMocks.closeWindow,
      getState: desktopState,
      groupWindows: vi.fn(),
      reopenWindow: vi.fn(),
      toggleFloating: vi.fn(),
    },
  }),
}));

vi.mock("../../hooks/use-os-window-commands", () => ({
  useOsWindowCommands: () => paletteMocks.windowCommands,
}));

vi.mock("../../hooks/use-window-manager-store", async () => {
  const { useSyncExternalStore } = await import("react");
  return {
    useWindowPaletteIntent: () => paletteMocks.paletteIntent,
    useWindowPaletteViewStack: () =>
      useSyncExternalStore(
        paletteMocks.subscribeViewStack,
        paletteMocks.readViewStack,
        paletteMocks.readViewStack
      ),
  };
});

vi.mock("../../lib/attention-model", () => ({
  isNeedsYouSession: (session: { id: string }) => paletteMocks.isWaiting(session.id),
}));

vi.mock("../../lib/window-slot-registry", () => ({
  pruneWindowSlotStores: vi.fn(),
  subscribeWindowSlotRegistry: () => () => undefined,
  windowSlotRegistryVersion: () => 0,
  windowSlotSnapshot: (windowId: string) => paletteMocks.windowSlots.get(windowId) ?? null,
}));

vi.mock("../../stores/window-manager-store", async () => {
  const { popPaletteViewFrame, pushPaletteViewFrame } =
    await import("../../lib/palette-view-stack");
  return {
    windowManagerStore: {
      trigger: {
        paletteIntentCleared: paletteMocks.paletteIntentCleared,
        paletteIntentRequested: paletteMocks.paletteIntentRequested,
        paletteViewPushed: ({ viewId }: { viewId: "sessions" }) =>
          paletteMocks.writeViewStack(
            pushPaletteViewFrame(paletteMocks.readViewStack(), viewId) as Array<{
              viewId: "sessions";
            }>
          ),
        paletteViewPopped: () =>
          paletteMocks.writeViewStack(
            popPaletteViewFrame(paletteMocks.readViewStack()) as Array<{ viewId: "sessions" }>
          ),
        paletteViewStackSet: ({ stack }: { stack: readonly { viewId: "sessions" }[] }) =>
          paletteMocks.writeViewStack(stack),
      },
    },
  };
});

function windowFixture(overrides: Partial<OsWindow> = {}): OsWindow {
  return {
    app: "tasks",
    desktopId: "desktop:alpha",
    groupId: null,
    id: "window:tasks",
    instanceKey: null,
    layer: 1,
    minimized: false,
    zoomed: false,
    navStack: [],
    nodeId: null,
    parentAxis: null,
    pinned: false,
    placement: "floating",
    rect: { h: 400, w: 600, x: 20, y: 20 },
    route: { pathname: "/tasks", search: {} },
    stackActive: true,
    stackId: null,
    ...overrides,
  };
}

const DESKTOPS: readonly LayoutDesktop[] = [
  {
    floating: [],
    floatingStacks: [],
    groups: [],
    id: "desktop:alpha",
    name: "Alpha",
    order: 0,
  },
  {
    floating: [],
    floatingStacks: [],
    groups: [],
    id: "desktop:beta",
    name: "Beta",
    order: 1,
  },
];

function desktopState(): OsDesktopRuntimeStore {
  if (paletteMocks.desktop === null) throw new Error("Desktop fixture was not configured.");
  return paletteMocks.desktop;
}

function commandSnapshot(workspaceId: string): NonNullable<OsDesktopRuntimeStore["snapshot"]> {
  return { workspaceId } as NonNullable<OsDesktopRuntimeStore["snapshot"]>;
}

function desktopFixture(
  windows: Record<string, OsWindow>,
  focusedId: string | null
): OsDesktopRuntimeStore {
  return {
    activeDesktopId: "desktop:alpha",
    client: null,
    clientAttachmentToken: null,
    connectionStatus: "connected",
    desktopBounds: null,
    desktops: DESKTOPS,
    dockMagnify: false,
    focusedId,
    frames: {},
    hydration: "live",
    presentation: "floating",
    projections: {},
    reduceMotion: false,
    snapshot: null,
    viewportState: "ready",
    wallpaper: "ember",
    windowManagerConfig: null,
    windows,
  };
}

const PALETTE_REGISTRY: PaletteRegistry = (() => {
  const commands = [
    paletteCommand({
      id: "window.tab.new",
      title: "New tab",
      section: "Tabs",
      action: { kind: "client_op", op: "window.tab.new" },
      execution: { retry_safe: true, single_flight: false },
    }),
    paletteCommand({
      id: "app.open.tasks",
      title: "Open Tasks",
      section: "Apps",
      action: { kind: "navigate", app: "tasks" },
      execution: { retry_safe: true, single_flight: false },
    }),
    paletteCommand({
      id: "app.open.terminal",
      title: "Open Terminal",
      section: "Apps",
      action: { kind: "navigate", app: "terminal" },
      execution: { retry_safe: true, single_flight: false },
    }),
    paletteCommand({
      id: "app.open.agents",
      title: "Open Agents",
      section: "Apps",
      action: { kind: "navigate", app: "agents" },
      execution: { retry_safe: true, single_flight: false },
    }),
    paletteCommand({
      id: "palette.view.sessions",
      title: "Sessions",
      section: "Views",
      action: { kind: "view", view: "sessions" },
      execution: { retry_safe: true, single_flight: false },
    }),
  ];
  return {
    commands,
    byId: new Map(commands.map(command => [command.id, command])),
    sources: [{ source: "core", status: "healthy" }],
    catalogRevision: "sha256:test",
    stale: false,
    daemonReachable: true,
  };
})();

/** The palette resolves the active profile, which is a server read. */
const paletteQueryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });

function PaletteHarness({ children }: { children: ReactNode }) {
  return (
    <QueryClientProvider client={paletteQueryClient}>
      <CmdPaletteRegistryProvider registry={PALETTE_REGISTRY}>
        {children}
      </CmdPaletteRegistryProvider>
    </QueryClientProvider>
  );
}

/**
 * The execution surfaces need commands the root fixture deliberately lacks: a
 * bound toggle chord, a settings destination for the meta deep-links, an
 * argument-bearing command, a destructive one, and an unhealthy one. They live
 * in their own registry so the root cases above keep their exact result set.
 */
function executionCommand(
  overrides: Partial<ResolvedPaletteCommand> & Pick<ResolvedPaletteCommand, "id" | "title">
): ResolvedPaletteCommand {
  return paletteCommand({
    section: "Notes",
    icon: "command",
    source: "core",
    action: { kind: "client_op", op: overrides.id },
    execution: { retry_safe: true, single_flight: false },
    ...overrides,
  });
}

const CAPTURE_COMMAND = executionCommand({
  id: "ext.notes.capture",
  title: "Capture note",
  alias: "cap",
  source: "ext.notes",
  action: { kind: "tool", tool: "ext__notes__capture" },
  execution: { retry_safe: false, single_flight: true },
  arguments: [
    { name: "title", type: "text", required: true, placeholder: "Note title" },
    { name: "tag", type: "dropdown", required: false, options: ["inbox", "idea"] },
  ],
});

const PURGE_COMMAND = executionCommand({
  id: "ext.notes.purge",
  title: "Purge archived notes",
  source: "ext.notes",
  destructive: true,
  action: { kind: "tool", tool: "ext__notes__purge" },
  execution: { retry_safe: false, single_flight: true },
  confirmation: {
    title: "Purge archived notes?",
    body: "Permanently deletes every archived note in this workspace.",
    confirm: "Purge",
  },
});

const UNHEALTHY_COMMAND = executionCommand({
  id: "ext.notes.recent",
  title: "Recent notes",
  source: "ext.notes",
  available: false,
  reason: "extension notes is unhealthy (crash loop)",
  action: { kind: "view", view: "ext.notes.recent" },
});

const EXECUTION_REGISTRY: PaletteRegistry = (() => {
  const commands = [
    executionCommand({
      id: "palette.open",
      title: "Command palette",
      section: "Shell",
      bindings: ["meta+KeyK"],
      chords: ["⌘K"],
      availability_exempt: true,
    }),
    executionCommand({
      id: "settings.layouts",
      title: "Settings → Layouts",
      section: "Settings",
      action: { kind: "navigate", app: "settings", args: { pathname: "/settings/layouts" } },
    }),
    CAPTURE_COMMAND,
    PURGE_COMMAND,
    UNHEALTHY_COMMAND,
  ];
  return {
    commands,
    byId: new Map(commands.map(command => [command.id, command])),
    sources: [
      { source: "core", status: "healthy" },
      {
        source: "ext.notes",
        status: "unhealthy",
        reason: "extension notes is unhealthy (crash loop)",
      },
    ],
    catalogRevision: "sha256:execution",
    stale: false,
    daemonReachable: true,
  };
})();

function ExecutionHarness({ children }: { children: ReactNode }) {
  return (
    <CmdPaletteRegistryProvider registry={EXECUTION_REGISTRY}>
      {children}
    </CmdPaletteRegistryProvider>
  );
}

const paletteDispatch = {
  // Stands in for the seam: a `view` action pushes the stack, exactly as
  // `dispatchPaletteCommand` routes it through the shell's `pushView` port, and
  // an argument-bearing or destructive command reports the step it needs rather
  // than running.
  run: vi.fn(async (command: ResolvedPaletteCommand, options?: CmdPaletteRunOptions) => {
    // Availability, then declared steps, then routing — the same order as
    // dispatchPaletteCommand, so a view that needs args never pushes first.
    if (!command.available) {
      return { status: "refused", reason: command.reason } as const;
    }
    if (command.arguments.length > 0 && options?.args === undefined) {
      requestPaletteArgs(command);
      return { status: "needs_args" } as const;
    }
    if (command.confirmation != null && options?.confirmed !== true) {
      requestPaletteConfirmation(command, options?.args ?? {});
      return { status: "needs_confirmation" } as const;
    }
    const view = command.action.kind === "view" ? command.action.view : undefined;
    if (view) paletteMocks.writeViewStack([{ viewId: view as "sessions" }]);
    if (command.action.kind === "navigate" && options?.navigate !== undefined) {
      const pathname = command.action.args?.pathname;
      options.navigate(
        command.action.app as OsAppId,
        typeof pathname === "string" ? { pathname, search: {} } : null
      );
    }
    return { status: "ran" } as const;
  }),
  runById: vi.fn(async (_commandId: string) => ({ status: "ran" }) as const),
  executeClientOp: vi.fn(async (_op: string, _payload: unknown) => undefined),
  setPinned: vi.fn(async (_command: ResolvedPaletteCommand, _pinned: boolean) => undefined),
};

function renderPalette(onOpenChange = vi.fn()) {
  return render(
    <PaletteHarness>
      <OsCommandPalette open dispatch={paletteDispatch} onOpenChange={onOpenChange} />
    </PaletteHarness>
  );
}

function resetPaletteHarness() {
  paletteQueryClient.clear();
  resetPaletteExecutionEntry();
  paletteDispatch.run.mockClear();
  paletteDispatch.runById.mockClear();
  paletteDispatch.setPinned.mockClear();
  paletteMocks.activeWorkspaceId = "workspace:alpha";
  paletteMocks.closeWindow.mockClear();
  paletteMocks.setActiveWorkspaceId.mockClear();
  paletteMocks.coordinator.userActivateWindow.mockClear();
  paletteMocks.coordinator.userOpen.mockClear();
  paletteMocks.coordinator.userOpen.mockResolvedValue("window:new-tab");
  paletteMocks.isWaiting.mockReset();
  paletteMocks.isWaiting.mockReturnValue(false);
  paletteMocks.paletteIntent = null;
  paletteMocks.paletteIntentCleared.mockClear();
  paletteMocks.paletteIntentRequested.mockClear();
  paletteMocks.pending = false;
  paletteMocks.profileAggregate = false;
  paletteMocks.scope = "workspace";
  paletteMocks.rankSignals = null;
  paletteMocks.fallbackAgentEnabled = false;
  paletteMocks.fallbackPending = false;
  paletteMocks.runFallback.mockClear();
  paletteMocks.domainWorkspaceNames.mockClear();
  paletteMocks.domainSections = [];
  paletteMocks.sessions = [];
  paletteMocks.sessionsLoading = false;
  paletteMocks.sessionsWorkspaceId.mockClear();
  paletteMocks.sessionsFilters.mockClear();
  paletteMocks.sessionGroupWorkspaces.mockClear();
  paletteMocks.sessionListScope = "workspace";
  paletteMocks.sessionListSaving = false;
  paletteMocks.setSessionListScope.mockClear();
  paletteMocks.jumpToSession.mockClear();
  paletteMocks.notifyUser.mockClear();
  paletteMocks.workspaceGroups = [];
  paletteMocks.writeViewStack([]);
  paletteMocks.toggleLocked = false;
  paletteMocks.windowSlots.clear();
  paletteMocks.windowCommands.commandsAvailable = true;
  paletteMocks.desktop = desktopFixture({ "window:tasks": windowFixture() }, "window:tasks");
  paletteMocks.commandAvailable = false;
  clearChooseSessionTerminalQuote();
}

function renderRoot(open = true, onOpenChange = vi.fn()) {
  return renderHook(
    (props: { open: boolean }) =>
      useOsPaletteRoot({
        open: props.open,
        onOpenChange,
        dispatch: (command, query, navigate) =>
          paletteDispatch.run(command, {
            query,
            ...(navigate === undefined ? {} : { navigate }),
          }),
        setPinned: (command, pinned) => void paletteDispatch.setPinned(command, pinned),
      }),
    { initialProps: { open }, wrapper: PaletteHarness }
  );
}

describe("useOsPaletteRoot", () => {
  beforeEach(() => {
    resetPaletteHarness();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("Should render every row from the registry projection and dispatch through the seam", async () => {
    const user = userEvent.setup();
    renderPalette();

    expect(screen.getByTestId("os-palette-command-app.open.tasks")).toBeInTheDocument();
    await user.click(screen.getByTestId("os-palette-command-app.open.tasks"));

    expect(paletteDispatch.run).toHaveBeenCalledOnce();
    expect(paletteDispatch.run.mock.lastCall?.[0]).toMatchObject({ id: "app.open.tasks" });
  });

  it("Should keep session lookup on the runtime workspace while scope resolution is pending [RA0289]", () => {
    paletteMocks.pending = true;
    paletteMocks.toggleLocked = true;
    renderRoot();

    expect(paletteMocks.sessionsWorkspaceId).toHaveBeenCalledWith("workspace:alpha");
  });

  it("Should omit workspace-name indexes outside global scope [RA0303]", () => {
    renderRoot();
    expect(paletteMocks.domainWorkspaceNames).toHaveBeenLastCalledWith(new Map());

    paletteMocks.scope = "global";
    renderRoot();
    expect(paletteMocks.domainWorkspaceNames).toHaveBeenLastCalledWith(
      new Map([
        ["workspace:home", "Home"],
        ["workspace:alpha", "Alpha"],
        ["workspace:beta", "Beta"],
      ])
    );
  });

  it("Should collect declared arguments before pushing a view [RA0292]", async () => {
    const command = paletteCommand({
      id: "palette.view.notes",
      title: "Notes",
      action: { kind: "view", view: "sessions" },
      arguments: [{ name: "q", type: "text", required: true, placeholder: "Query" }],
    });

    await expect(paletteDispatch.run(command)).resolves.toEqual({ status: "needs_args" });
    expect(paletteMocks.readViewStack()).toEqual([]);
  });

  it("Should include the home workspace in global session search", async () => {
    paletteMocks.scope = "global";
    paletteMocks.rankSignals = TEST_RANK_SIGNALS;
    paletteMocks.workspaceGroups = [
      workspaceGroup("workspace:home", "Home", [
        paletteSession({
          id: "s-home",
          name: "Home target",
          workspace_id: "workspace:home",
        }),
      ]),
    ];
    const { result } = renderRoot();

    act(() => result.current.setQuery("Home target"));

    await waitFor(() => expect(result.current.entities.sessions).toHaveLength(1));
    expect(result.current.entities.sessions[0]?.sessionId).toBe("s-home");
    expect(paletteMocks.sessionGroupWorkspaces).toHaveBeenLastCalledWith(
      paletteMocks.registeredWorkspaces
    );
  });

  it("Should identify root session results by owner under the aggregate profile lens", async () => {
    const user = userEvent.setup();
    paletteMocks.profileAggregate = true;
    paletteMocks.rankSignals = TEST_RANK_SIGNALS;
    paletteMocks.sessions = [
      paletteSession({
        id: "s-marketing",
        name: "Campaign plan",
        profile_id: "01J9MARKETING00000000000000",
        profile_name: "marketing",
        profile_color: "#c26ad6",
        profile_icon: "megaphone",
      }),
    ];

    renderPalette();
    await user.type(
      screen.getByPlaceholderText("Search apps, sessions, and actions…"),
      "Campaign plan"
    );

    const row = await screen.findByTestId("os-palette-session-s-marketing");
    const ownerTag = within(row).getByTestId("profile-owner-tag");
    expect(ownerTag).toHaveAttribute("aria-label", "marketing");
    expect(ownerTag).toHaveAttribute("title", "marketing");
  });

  it("Should remove the agent fallback row when its effective setting is disabled [UT-151]", async () => {
    const user = userEvent.setup();
    paletteMocks.fallbackAgentEnabled = true;
    const rendered = renderPalette();

    await user.type(
      screen.getByPlaceholderText("Search apps, sessions, and actions…"),
      "gibberish-with-no-match"
    );
    expect(screen.getByTestId("os-palette-agent-fallback")).toHaveTextContent(
      "Ask agent: 'gibberish-with-no-match'"
    );

    paletteMocks.fallbackAgentEnabled = false;
    rendered.rerender(
      <PaletteHarness>
        <OsCommandPalette open dispatch={paletteDispatch} onOpenChange={vi.fn()} />
      </PaletteHarness>
    );
    expect(screen.queryByTestId("os-palette-agent-fallback")).not.toBeInTheDocument();
  });

  it("Should scope the picker to the destination tab and release it on close [UT-081, UT-082]", () => {
    paletteMocks.paletteIntent = { kind: "destination", windowId: "window:new-tab" };
    paletteMocks.desktop = desktopFixture({}, null);
    const { result, rerender } = renderRoot();

    expect(result.current.destinationWindowId).toBe("window:new-tab");
    rerender({ open: false });
    // Dismissing the palette abandons the intent without destroying the tab.
    expect(result.current.destinationWindowId).toBeNull();
    expect(paletteMocks.closeWindow).not.toHaveBeenCalled();
  });

  it("Should hand the empty tab's place to the picked session [UT-080]", async () => {
    paletteMocks.paletteIntent = { kind: "destination", windowId: "window:new-tab" };
    paletteMocks.desktop = desktopFixture({}, null);
    const { result } = renderRoot();

    await act(async () => {
      result.current.openSession({
        sessionId: "session:one",
        title: "One",
        agentName: "codex",
        workspaceId: "workspace:alpha",
        route: { pathname: "/agents/codex/sessions/session%3Aone", search: {} },
      });
    });

    expect(paletteMocks.paletteIntentCleared).toHaveBeenCalledOnce();
    expect(paletteMocks.coordinator.userOpen).toHaveBeenCalledWith({
      app: "session",
      instanceKey: "session:one",
      route: { pathname: "/agents/codex/sessions/session%3Aone", search: {} },
      stackTargetWindowId: "window:new-tab",
    });
    await waitFor(() => expect(paletteMocks.closeWindow).toHaveBeenCalledWith("window:new-tab"));
    // BR-20: destination picking is not a landing, so the shared jump stays out.
    expect(paletteMocks.jumpToSession).not.toHaveBeenCalled();
  });

  it("Should hand the empty tab's place to a fresh app instance [UT-080]", async () => {
    paletteMocks.paletteIntent = { kind: "destination", windowId: "window:new-tab" };
    paletteMocks.desktop = desktopFixture(
      { "window:tasks": windowFixture({ id: "window:tasks", app: "tasks" }) },
      "window:new-tab"
    );
    const { result } = renderRoot();
    const command = PALETTE_REGISTRY.byId.get("app.open.tasks");
    if (command === undefined) throw new Error("Expected the Tasks command fixture.");

    await act(async () => {
      result.current.runCommand(command);
    });

    expect(paletteMocks.paletteIntentCleared).toHaveBeenCalledOnce();
    expect(paletteMocks.coordinator.userOpen).toHaveBeenCalledWith({
      app: "tasks",
      stackTargetWindowId: "window:new-tab",
    });
    await waitFor(() => expect(paletteMocks.closeWindow).toHaveBeenCalledWith("window:new-tab"));
  });

  it("Should open Terminal through userOpen and not land a session", async () => {
    paletteMocks.paletteIntent = { kind: "destination", windowId: "window:new-tab" };
    paletteMocks.desktop = desktopFixture(
      { "window:tasks": windowFixture({ id: "window:tasks", app: "tasks" }) },
      "window:new-tab"
    );
    const { result } = renderRoot();
    const command = PALETTE_REGISTRY.byId.get("app.open.terminal");
    if (command === undefined) throw new Error("Expected the Terminal command fixture.");

    await act(async () => {
      result.current.runCommand(command);
    });

    expect(paletteMocks.paletteIntentCleared).toHaveBeenCalledOnce();
    expect(paletteMocks.coordinator.userOpen).toHaveBeenCalledExactlyOnceWith({
      app: "terminal",
      stackTargetWindowId: "window:new-tab",
    });
    expect(paletteMocks.jumpToSession).not.toHaveBeenCalled();
    await waitFor(() => expect(paletteMocks.closeWindow).toHaveBeenCalledWith("window:new-tab"));
  });

  it("Should land on a session through the shared attention jump [BR-20]", async () => {
    const { result } = renderRoot();

    await act(async () => {
      result.current.openSession({
        sessionId: "session:one",
        title: "One",
        agentName: "codex",
        workspaceId: "workspace:alpha",
        route: { pathname: "/agents/codex/sessions/session%3Aone", search: {} },
      });
    });

    expect(paletteMocks.jumpToSession).toHaveBeenCalledExactlyOnceWith({
      sessionId: "session:one",
      agentName: "codex",
      workspaceId: "workspace:alpha",
    });
    // The root must not keep a second landing implementation of its own.
    expect(paletteMocks.coordinator.userOpen).not.toHaveBeenCalled();
  });

  it("Should drop a choose-held quote when the session picker is dismissed", async () => {
    const held = stageSessionTerminalQuote({
      sessionId: "session:held",
      terminalId: "term-4f21c9a03b7e",
      fromLine: 1,
      lines: ["stale choose quote"],
    });
    clearSessionTerminalQuote("session:held");
    holdChooseSessionTerminalQuote(held);

    const first = renderRoot(true, vi.fn());
    first.rerender({ open: false });
    expect(peekChooseSessionTerminalQuote()).toBeNull();

    const later = renderRoot(true, vi.fn());
    await act(async () => {
      later.result.current.openSession({
        sessionId: "session:later",
        title: "Later",
        agentName: "codex",
        workspaceId: "workspace:alpha",
        route: { pathname: "/agents/codex/sessions/session%3Alater", search: {} },
      });
    });

    expect(peekSessionTerminalQuote("session:later")).toBeNull();
    expect(peekChooseSessionTerminalQuote()).toBeNull();
  });

  it("Should stage a choose-held quote exactly once onto the picked session", async () => {
    const held = stageSessionTerminalQuote({
      sessionId: "session:held",
      terminalId: "term-4f21c9a03b7e",
      fromLine: 1,
      lines: ["chosen quote"],
    });
    clearSessionTerminalQuote("session:held");
    holdChooseSessionTerminalQuote(held);

    const { result } = renderRoot();
    await act(async () => {
      result.current.openSession({
        sessionId: "session:one",
        title: "One",
        agentName: "codex",
        workspaceId: "workspace:alpha",
        route: { pathname: "/agents/codex/sessions/session%3Aone", search: {} },
      });
    });

    expect(peekSessionTerminalQuote("session:one")?.text).toBe(held.text);
    expect(peekChooseSessionTerminalQuote()).toBeNull();
    clearSessionTerminalQuote("session:one");
  });

  it("Should open a concrete domain row on its identity route", async () => {
    paletteMocks.commandAvailable = true;
    paletteMocks.desktop = {
      ...desktopFixture({ "window:tasks": windowFixture() }, "window:tasks"),
      snapshot: commandSnapshot("workspace:alpha"),
    };
    const onOpenChange = vi.fn();
    const { result } = renderRoot(true, onOpenChange);

    await act(async () => {
      result.current.openDomainRow({
        app: "tasks",
        key: "task:task-42",
        label: "Review release",
        route: { pathname: "/tasks/task-42", search: {} },
      });
    });

    expect(onOpenChange).toHaveBeenCalledExactlyOnceWith(false);
    expect(paletteMocks.coordinator.userOpen).toHaveBeenCalledExactlyOnceWith({
      app: "tasks",
      route: { pathname: "/tasks/task-42", search: {} },
    });
  });

  it("Should open a terminal catalog row as that instance, not a session", async () => {
    paletteMocks.commandAvailable = true;
    paletteMocks.desktop = {
      ...desktopFixture({ "window:tasks": windowFixture() }, "window:tasks"),
      snapshot: commandSnapshot("workspace:alpha"),
    };
    const { result } = renderRoot(true);

    await act(async () => {
      result.current.openDomainRow({
        app: "terminal",
        key: "terminal:term-4f21c9a03b7e",
        label: "dev server",
        detail: "running · agent claude-code",
        status: "running",
        route: { pathname: "/terminal/term-4f21c9a03b7e", search: {} },
      });
    });

    expect(paletteMocks.coordinator.userOpen).toHaveBeenCalledExactlyOnceWith({
      app: "terminal",
      instanceKey: "term-4f21c9a03b7e",
      route: { pathname: "/terminal/term-4f21c9a03b7e", search: {} },
    });
    expect(paletteMocks.jumpToSession).not.toHaveBeenCalled();
  });

  it("Should report when the coordinator refuses a concrete domain target", async () => {
    paletteMocks.commandAvailable = true;
    paletteMocks.desktop = {
      ...desktopFixture({ "window:tasks": windowFixture() }, "window:tasks"),
      snapshot: commandSnapshot("workspace:alpha"),
    };
    paletteMocks.coordinator.userOpen.mockResolvedValueOnce(null);
    const onOpenChange = vi.fn();
    const { result } = renderRoot(true, onOpenChange);

    await act(async () => {
      result.current.openDomainRow({
        app: "tasks",
        key: "task:unreachable",
        label: "Unavailable task",
        route: { pathname: "/tasks/unreachable", search: {} },
      });
    });

    await waitFor(() =>
      expect(paletteMocks.notifyUser).toHaveBeenCalledWith({
        message: "Couldn't open Unavailable task. Try again.",
        tone: "error",
      })
    );
  });

  it("Should switch to a global row's owning workspace before opening it", async () => {
    paletteMocks.scope = "global";
    paletteMocks.commandAvailable = true;
    paletteMocks.desktop = {
      ...desktopFixture({ "window:tasks": windowFixture() }, "window:tasks"),
      snapshot: commandSnapshot("workspace:alpha"),
    };
    const onOpenChange = vi.fn();
    const rendered = renderRoot(true, onOpenChange);

    act(() => {
      rendered.result.current.openDomainRow({
        app: "jobs",
        key: "job:job-beta",
        label: "Beta job",
        route: { pathname: "/jobs/job-beta", search: {} },
        workspaceId: "workspace:beta",
      });
    });

    expect(paletteMocks.setActiveWorkspaceId).toHaveBeenCalledExactlyOnceWith("workspace:beta");
    expect(paletteMocks.coordinator.userOpen).not.toHaveBeenCalled();

    paletteMocks.activeWorkspaceId = "workspace:beta";
    paletteMocks.desktop = {
      ...paletteMocks.desktop,
      snapshot: commandSnapshot("workspace:beta"),
    };
    rendered.rerender({ open: true });

    await waitFor(() =>
      expect(paletteMocks.coordinator.userOpen).toHaveBeenCalledExactlyOnceWith({
        app: "jobs",
        route: { pathname: "/jobs/job-beta", search: {} },
      })
    );
  });

  it("Should retain a foreign target after the pushed palette view closes", async () => {
    paletteMocks.scope = "global";
    paletteMocks.commandAvailable = true;
    paletteMocks.desktop = {
      ...desktopFixture({ "window:tasks": windowFixture() }, "window:tasks"),
      snapshot: commandSnapshot("workspace:alpha"),
    };
    const onOpenChange = vi.fn();
    const rendered = renderRoot(true, onOpenChange);

    act(() => {
      rendered.result.current.openDomainRow({
        app: "jobs",
        key: "job:job-beta",
        label: "Beta job",
        route: { pathname: "/jobs/job-beta", search: {} },
        workspaceId: "workspace:beta",
      });
    });

    expect(onOpenChange).toHaveBeenCalledExactlyOnceWith(false);
    expect(paletteMocks.coordinator.userOpen).not.toHaveBeenCalled();

    rendered.rerender({ open: false });
    paletteMocks.activeWorkspaceId = "workspace:beta";
    paletteMocks.desktop = {
      ...paletteMocks.desktop,
      snapshot: commandSnapshot("workspace:beta"),
    };
    rendered.rerender({ open: false });

    await waitFor(() =>
      expect(paletteMocks.coordinator.userOpen).toHaveBeenCalledExactlyOnceWith({
        app: "jobs",
        route: { pathname: "/jobs/job-beta", search: {} },
      })
    );
  });

  it("Should wait for the window manager workspace after the runtime is ready", async () => {
    paletteMocks.commandAvailable = true;
    paletteMocks.desktop = {
      ...desktopFixture({ "window:tasks": windowFixture() }, "window:tasks"),
      snapshot: commandSnapshot("workspace:stale"),
    };
    const onOpenChange = vi.fn();
    const rendered = renderRoot(true, onOpenChange);

    act(() => {
      rendered.result.current.openDomainRow({
        app: "tasks",
        key: "task:task-alpha",
        label: "Alpha task",
        route: { pathname: "/tasks/task-alpha", search: {} },
        workspaceId: "workspace:alpha",
      });
    });

    expect(paletteMocks.setActiveWorkspaceId).not.toHaveBeenCalled();
    expect(paletteMocks.coordinator.userOpen).not.toHaveBeenCalled();

    paletteMocks.desktop = {
      ...paletteMocks.desktop,
      snapshot: commandSnapshot("workspace:alpha"),
    };
    rendered.rerender({ open: true });

    await waitFor(() =>
      expect(paletteMocks.coordinator.userOpen).toHaveBeenCalledExactlyOnceWith({
        app: "tasks",
        route: { pathname: "/tasks/task-alpha", search: {} },
      })
    );
  });

  it("Should offer only navigable targets in destination mode and stay honest when none are eligible [UT-107, UT-108]", () => {
    paletteMocks.paletteIntent = { kind: "destination", windowId: "window:new-tab" };
    paletteMocks.desktop = desktopFixture({}, null);
    const { result } = renderRoot();

    expect(result.current.destination).toBe(true);
    const offered = result.current.sections.flatMap(section =>
      section.commands.map(command => command.action.kind)
    );
    expect(offered.length).toBeGreaterThan(0);
    expect(new Set(offered)).toEqual(new Set(["navigate"]));
    // Tabs and worktrees are absent in destination mode, not disabled.
    expect(result.current.entities.tabs).toHaveLength(0);
    expect(result.current.entities.worktrees).toHaveLength(0);

    act(() => result.current.setQuery("zzzz"));
    expect(result.current.sections).toHaveLength(0);
  });

  it("Should preserve the selected tab route by activating its existing window [UT-090]", async () => {
    paletteMocks.desktop = desktopFixture(
      {
        "window:deep-task": windowFixture({
          id: "window:deep-task",
          navStack: [{ pathname: "/tasks", search: {} }],
          route: { pathname: "/tasks/task-42", search: { pane: "activity" } },
        }),
      },
      "window:deep-task"
    );
    const { result } = renderRoot();

    await act(async () => {
      result.current.goToTab("window:deep-task");
    });

    expect(paletteMocks.coordinator.userActivateWindow).toHaveBeenCalledWith("window:deep-task");
  });

  it("Should render contextual Go to tab results, attention, and no empty group [UT-100]", () => {
    const alpha = windowFixture({ id: "window:alpha" });
    const beta = windowFixture({ desktopId: "desktop:beta", id: "window:beta" });
    const waiting = windowFixture({
      app: "session",
      desktopId: "desktop:beta",
      id: "window:waiting",
      instanceKey: "session:waiting",
      route: { pathname: "/agents/codex/sessions/session%3Awaiting", search: {} },
    });
    paletteMocks.sessions = [
      {
        agent_name: "codex",
        badge: "running",
        id: "session:waiting",
        name: "Needs input",
        updated_at: "2026-08-16T10:00:00Z",
      },
    ];
    paletteMocks.isWaiting.mockImplementation(
      (sessionId: string) => sessionId === "session:waiting"
    );
    paletteMocks.desktop = desktopFixture(
      { [alpha.id]: alpha, [beta.id]: beta, [waiting.id]: waiting },
      alpha.id
    );

    const rendered = renderPalette();

    expect(screen.getByText("Go to tab")).toBeInTheDocument();
    expect(screen.getByTestId("os-palette-tab-window:alpha")).toHaveTextContent("TasksAlpha");
    expect(screen.getByTestId("os-palette-tab-window:beta")).toHaveTextContent("TasksBeta");
    expect(screen.getByTestId("os-palette-tab-window:waiting")).toHaveTextContent(
      "Needs input1Beta"
    );

    paletteMocks.desktop = desktopFixture({}, null);
    rendered.rerender(
      <PaletteHarness>
        <OsCommandPalette open dispatch={paletteDispatch} onOpenChange={vi.fn()} />
      </PaletteHarness>
    );
    expect(screen.queryByText("Go to tab")).not.toBeInTheDocument();
  });

  it("Should restore a minimized tab on its desktop through the single activation action [UT-101]", async () => {
    paletteMocks.desktop = desktopFixture(
      {
        "window:minimized": windowFixture({
          desktopId: "desktop:beta",
          id: "window:minimized",
          minimized: true,
        }),
      },
      null
    );
    const { result } = renderRoot();

    await act(async () => {
      result.current.goToTab("window:minimized");
    });

    expect(paletteMocks.coordinator.userActivateWindow).toHaveBeenCalledWith("window:minimized");
  });
});

function paletteSession(
  overrides: Partial<PaletteSessionFixture> & { id: string }
): PaletteSessionFixture {
  return {
    name: null,
    agent_name: "claude",
    badge: "idle",
    workspace_id: "workspace:alpha",
    updated_at: "2026-08-16T10:00:00Z",
    ...overrides,
  };
}

function workspaceGroup(
  workspaceId: string,
  name: string,
  sessions: PaletteSessionFixture[],
  overrides: Partial<{ loading: boolean; failed: boolean }> = {}
) {
  return {
    workspaceId,
    workspaceName: name,
    sessions,
    total: sessions.length,
    loading: false,
    failed: false,
    retry: () => undefined,
    ...overrides,
  };
}

async function pushSessionsView(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByTestId("os-palette-command-palette.view.sessions"));
  return screen.getByPlaceholderText("Search sessions…");
}

describe("palette nested views", () => {
  beforeEach(() => {
    resetPaletteHarness();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("Should push a view, keep editing text, and pop only on an empty query [UT-059]", async () => {
    const user = userEvent.setup();
    paletteMocks.sessions = [paletteSession({ id: "s-1", name: "Refactor session store" })];
    renderPalette();
    expect(screen.queryByTestId("os-palette-breadcrumb")).not.toBeInTheDocument();

    const input = await pushSessionsView(user);
    expect(screen.getByTestId("os-palette-breadcrumb")).toHaveTextContent("Sessions");

    await user.type(input, "ref");
    expect(input).toHaveValue("ref");
    await user.keyboard("{Backspace}");
    expect(input).toHaveValue("re");
    expect(screen.getByTestId("os-palette-breadcrumb")).toBeInTheDocument();

    await user.clear(input);
    await user.keyboard("{Backspace}");
    expect(screen.queryByTestId("os-palette-breadcrumb")).not.toBeInTheDocument();
    expect(screen.getByTestId("os-palette-command-palette.view.sessions")).toBeInTheDocument();
  });

  it("Should render only the active view's results and name its own empty state [UT-060]", async () => {
    const user = userEvent.setup();
    paletteMocks.sessions = [paletteSession({ id: "s-1", name: "Refactor session store" })];
    const rendered = renderPalette();
    expect(screen.getByTestId("os-palette-command-app.open.tasks")).toBeInTheDocument();

    await pushSessionsView(user);
    expect(screen.getByTestId("os-palette-session-view-s-1")).toBeInTheDocument();
    // No root row bleeds into the level (BR-32): the view owns the surface.
    expect(screen.queryByTestId("os-palette-command-app.open.tasks")).not.toBeInTheDocument();
    expect(
      screen.queryByTestId("os-palette-command-palette.view.sessions")
    ).not.toBeInTheDocument();

    paletteMocks.sessions = [];
    rendered.rerender(
      <PaletteHarness>
        <OsCommandPalette open dispatch={paletteDispatch} onOpenChange={vi.fn()} />
      </PaletteHarness>
    );
    expect(screen.getByText("No sessions in this workspace yet.")).toBeInTheDocument();
    expect(screen.getByTestId("os-palette-breadcrumb")).toBeInTheDocument();
  });

  it("Should name each session owner under the aggregate profile lens", async () => {
    const user = userEvent.setup();
    paletteMocks.profileAggregate = true;
    paletteMocks.sessions = [
      paletteSession({
        id: "s-marketing",
        name: "Campaign plan",
        profile_id: "01J9MARKETING00000000000000",
        profile_name: "marketing",
        profile_color: "#c26ad6",
        profile_icon: "megaphone",
        profile_archived: false,
      }),
    ];

    renderPalette();
    await pushSessionsView(user);

    const ownerTag = screen.getByTestId("profile-owner-tag");
    expect(ownerTag).toHaveAttribute("aria-label", "marketing");
    expect(ownerTag).toHaveAttribute("title", "marketing");
    expect(ownerTag).not.toHaveAttribute("data-archived");
  });

  it("Should narrow by chip, name a zero match, and clear it in one keystroke [UT-061]", async () => {
    const user = userEvent.setup();
    paletteMocks.sessions = [
      paletteSession({ id: "s-needs", name: "Chargeback flow", badge: "waiting-for-input" }),
      paletteSession({ id: "s-running", name: "Fix payment retries", badge: "running" }),
    ];
    renderPalette();
    const input = await pushSessionsView(user);

    expect(screen.getByTestId("os-palette-session-filter-all")).toHaveTextContent(/All\s*2/);
    expect(screen.getByTestId("os-palette-session-filter-finished")).toHaveTextContent(
      /Finished\s*0/
    );

    await user.click(screen.getByTestId("os-palette-session-filter-needs-you"));
    expect(screen.getByTestId("os-palette-session-view-s-needs")).toBeInTheDocument();
    expect(screen.queryByTestId("os-palette-session-view-s-running")).not.toBeInTheDocument();

    await user.click(screen.getByTestId("os-palette-session-filter-finished"));
    expect(screen.getByText("Nothing finished right now.")).toBeInTheDocument();

    await user.click(input);
    await user.keyboard("{Backspace}");
    expect(screen.getByTestId("os-palette-session-view-s-needs")).toBeInTheDocument();
    expect(screen.getByTestId("os-palette-session-view-s-running")).toBeInTheDocument();
    expect(screen.getByTestId("os-palette-breadcrumb")).toBeInTheDocument();
  });

  it("Should widen through the operator's persisted session-list scope [UT-099]", async () => {
    const user = userEvent.setup();
    paletteMocks.sessions = [paletteSession({ id: "s-alpha", name: "Refactor session store" })];
    const rendered = renderPalette();
    await pushSessionsView(user);

    const scope = screen.getByTestId("os-palette-session-scope");
    expect(scope).toHaveAccessibleName("All workspaces");
    expect(scope).toHaveAttribute("aria-pressed", "false");
    await user.click(scope);
    expect(paletteMocks.setSessionListScope).toHaveBeenCalledWith("all-workspaces");

    paletteMocks.sessionListScope = "all-workspaces";
    paletteMocks.workspaceGroups = [
      workspaceGroup("workspace:alpha", "Alpha", [
        paletteSession({ id: "s-alpha", name: "Refactor session store" }),
      ]),
      workspaceGroup("workspace:beta", "Beta", [
        paletteSession({
          id: "s-beta",
          name: "Chargeback flow",
          workspace_id: "workspace:beta",
          badge: "waiting-for-input",
        }),
      ]),
      workspaceGroup("workspace:gamma", "Gamma", [], { failed: true }),
    ];
    rendered.rerender(
      <PaletteHarness>
        <OsCommandPalette open dispatch={paletteDispatch} onOpenChange={vi.fn()} />
      </PaletteHarness>
    );

    expect(screen.getByTestId("os-palette-session-scope")).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByTestId("os-palette-session-view-s-beta")).toHaveTextContent("Beta");
    expect(screen.getByText("Gamma couldn't be loaded.")).toBeInTheDocument();
    // Attention-first survives the union: the needs-you row from the foreign
    // workspace outranks the idle row from this one.
    const rows = screen.getAllByTestId(/^os-palette-session-view-/);
    expect(rows.map(row => row.dataset.testid)).toEqual([
      "os-palette-session-view-s-beta",
      "os-palette-session-view-s-alpha",
    ]);

    await user.click(screen.getByTestId("os-palette-session-scope"));
    expect(paletteMocks.setSessionListScope).toHaveBeenLastCalledWith("workspace");

    paletteMocks.workspaceGroups = [];
    rendered.rerender(
      <PaletteHarness>
        <OsCommandPalette open dispatch={paletteDispatch} onOpenChange={vi.fn()} />
      </PaletteHarness>
    );
    expect(screen.getByText("No sessions across workspaces yet.")).toBeInTheDocument();
  });

  it("Should read the archive through its own toggle without touching the breadth [UT-061]", async () => {
    const user = userEvent.setup();
    paletteMocks.sessions = [paletteSession({ id: "s-alpha", name: "Refactor session store" })];
    renderPalette();
    await pushSessionsView(user);

    const archived = screen.getByTestId("os-palette-session-archived");
    expect(archived).toHaveAccessibleName("Archived");
    expect(archived).toHaveAttribute("aria-pressed", "false");
    expect(paletteMocks.sessionsFilters).toHaveBeenLastCalledWith(
      expect.not.objectContaining({ archive: expect.anything() })
    );

    await user.click(archived);

    expect(screen.getByTestId("os-palette-session-archived")).toHaveAttribute(
      "aria-pressed",
      "true"
    );
    expect(paletteMocks.sessionsFilters).toHaveBeenLastCalledWith(
      expect.objectContaining({ archive: "only" })
    );
    // The archive is its own axis: flipping it never writes the persisted breadth.
    expect(paletteMocks.setSessionListScope).not.toHaveBeenCalled();
  });

  it("Should land on a session through the shared attention jump [UT-060]", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    paletteMocks.sessions = [
      paletteSession({ id: "s-1", name: "Refactor session store", agent_name: "codex" }),
    ];
    render(
      <PaletteHarness>
        <OsCommandPalette open dispatch={paletteDispatch} onOpenChange={onOpenChange} />
      </PaletteHarness>
    );
    await pushSessionsView(user);

    await user.click(screen.getByTestId("os-palette-session-view-s-1"));

    expect(onOpenChange).toHaveBeenCalledWith(false);
    expect(paletteMocks.jumpToSession).toHaveBeenCalledWith({
      sessionId: "s-1",
      agentName: "codex",
      workspaceId: "workspace:alpha",
    });
    expect(paletteMocks.coordinator.userOpen).not.toHaveBeenCalled();
  });

  it("Should keep the keyboard selection while the session catalog moves underneath it [UT-076]", async () => {
    const user = userEvent.setup();
    const stamped = (id: string, minute: string) =>
      paletteSession({ id, name: id, attention_changed_at: `2026-08-16T10:0${minute}:00Z` });
    paletteMocks.sessions = [stamped("s-a", "3"), stamped("s-b", "2"), stamped("s-c", "1")];
    const rendered = renderPalette();
    await pushSessionsView(user);

    await waitFor(() => {
      expect(screen.getByTestId("os-palette-session-view-s-a")).toHaveAttribute(
        "aria-selected",
        "true"
      );
    });
    await user.keyboard("{ArrowDown}");
    expect(screen.getByTestId("os-palette-session-view-s-b")).toHaveAttribute(
      "aria-selected",
      "true"
    );

    paletteMocks.sessions = [
      stamped("s-new", "4"),
      stamped("s-a", "3"),
      stamped("s-b", "2"),
      stamped("s-c", "1"),
    ];
    rendered.rerender(
      <PaletteHarness>
        <OsCommandPalette open dispatch={paletteDispatch} onOpenChange={vi.fn()} />
      </PaletteHarness>
    );
    expect(screen.getByTestId("os-palette-session-view-s-b")).toHaveAttribute(
      "aria-selected",
      "true"
    );

    paletteMocks.sessions = [stamped("s-new", "4"), stamped("s-a", "3"), stamped("s-c", "1")];
    rendered.rerender(
      <PaletteHarness>
        <OsCommandPalette open dispatch={paletteDispatch} onOpenChange={vi.fn()} />
      </PaletteHarness>
    );
    expect(screen.getByTestId("os-palette-session-view-s-c")).toHaveAttribute(
      "aria-selected",
      "true"
    );
  });

  it("Should keep the selected domain row when an earlier async domain arrives [UT-114]", async () => {
    const user = userEvent.setup();
    const taskRows = ["alpha", "beta"].map(id => ({
      key: `task:${id}`,
      label: `Task ${id}`,
      app: "tasks" as const,
      route: { pathname: "/tasks", search: { q: id } },
    }));
    paletteMocks.domainSections = [
      { title: "Tasks", rows: taskRows, total: taskRows.length, loading: false, error: null },
    ];
    const rendered = renderPalette();

    const selectedTask = screen.getByTestId("os-palette-domain-row-task:beta");
    await user.hover(selectedTask);
    expect(selectedTask).toHaveAttribute("aria-selected", "true");

    paletteMocks.domainSections = [
      {
        title: "Agents",
        rows: [
          {
            key: "agent:codex",
            label: "Codex",
            app: "agents",
            route: { pathname: "/agents", search: {} },
          },
        ],
        total: 1,
        loading: false,
        error: null,
      },
      { title: "Tasks", rows: taskRows, total: taskRows.length, loading: false, error: null },
    ];
    rendered.rerender(
      <PaletteHarness>
        <OsCommandPalette open dispatch={paletteDispatch} onOpenChange={vi.fn()} />
      </PaletteHarness>
    );

    expect(screen.getByTestId("os-palette-domain-row-task:beta")).toHaveAttribute(
      "aria-selected",
      "true"
    );
  });

  it("Should bound the rendered rows and say how many matches went unrendered", async () => {
    const user = userEvent.setup();
    paletteMocks.sessions = Array.from({ length: PALETTE_SESSION_ROW_LIMIT + 12 }, (_, index) =>
      paletteSession({ id: `s-${index}`, name: `Session ${index}` })
    );
    renderPalette();
    await pushSessionsView(user);

    expect(screen.getAllByTestId(/^os-palette-session-view-/)).toHaveLength(
      PALETTE_SESSION_ROW_LIMIT
    );
    const list = screen.getByTestId("os-command-palette");
    expect(
      within(list).getByText(
        `Showing ${PALETTE_SESSION_ROW_LIMIT} of ${PALETTE_SESSION_ROW_LIMIT + 12} — keep typing to narrow.`,
        { exact: false }
      )
    ).toBeInTheDocument();
  });

  it("Should gate every domain read by the daemon query bounds [UT-110, UT-111]", () => {
    expect(isPaletteDomainSearchEnabled(true, "ab", TEST_WEIGHTS)).toBe(true);
    expect(isPaletteDomainSearchEnabled(true, "a", TEST_WEIGHTS)).toBe(false);
    expect(
      isPaletteDomainSearchEnabled(
        true,
        "x".repeat(TEST_WEIGHTS.max_query_length + 1),
        TEST_WEIGHTS
      )
    ).toBe(false);
    expect(isPaletteDomainSearchEnabled(false, "ab", TEST_WEIGHTS)).toBe(false);
  });

  it("Should project Vault names and metadata without any secret value [UT-112]", () => {
    type HasValue = "value" extends keyof ReturnType<typeof projectVaultRows>[number]
      ? true
      : false;
    const hasValue: HasValue = false;
    const rows = projectVaultRows(
      [
        {
          ref: "vault:providers/codex/api_key",
          namespace: "providers",
          kind: "api_key",
          present: true,
          created_at: "2026-08-19T12:00:00Z",
          updated_at: "2026-08-19T12:00:00Z",
          value: "must-never-cross-the-projection",
        } as never,
      ],
      "workspace",
      new Map()
    );

    expect(hasValue).toBe(false);
    expect(rows).toEqual([
      expect.objectContaining({ label: "api_key", namespace: "providers", kind: "api_key" }),
    ]);
    expect(rows[0]).not.toHaveProperty("value");
  });

  it("Should render a prepared domain section's cap, overflow, error, and labels [UT-113, UT-115, UT-116]", () => {
    paletteMocks.domainSections = [
      {
        title: "Tasks",
        rows: Array.from({ length: TEST_WEIGHTS.entity_section_visible_cap }, (_, index) => ({
          key: `task:${index}`,
          label: `Task ${index}`,
          workspaceLabel: "Alpha",
          app: "tasks" as const,
          route: { pathname: "/tasks", search: { q: `Task ${index}` } },
        })),
        total: TEST_WEIGHTS.entity_section_visible_cap + 4,
        loading: false,
        error: null,
      },
      {
        title: "Bridges",
        rows: [],
        total: 0,
        loading: false,
        error: "Bridges: transport offline",
      },
    ];
    renderPalette();

    expect(screen.getAllByTestId(/^os-palette-domain-row-task:/)).toHaveLength(
      TEST_WEIGHTS.entity_section_visible_cap
    );
    expect(
      screen.getByText(
        `showing ${TEST_WEIGHTS.entity_section_visible_cap} of ${TEST_WEIGHTS.entity_section_visible_cap + 4}`
      )
    ).toBeInTheDocument();
    expect(screen.getByTestId("os-palette-domain-error-bridges")).toHaveTextContent(
      "Bridges: transport offline"
    );
    for (const row of screen.getAllByTestId(/^os-palette-domain-row-task:/)) {
      expect(within(row).getByText("Alpha")).toBeInTheDocument();
    }
  });

  it("Should keep a worktree owner tag under every profile lens", () => {
    paletteMocks.domainSections = [
      {
        title: "Worktrees",
        rows: [
          {
            key: "worktree:agent-comms",
            label: "agent-comms",
            detail: "agent-comms",
            status: "ready",
            owner: {
              id: "00000000000000000000000000",
              name: "default",
              color: "#8E8EB5",
              icon: "circle",
              emoji: null,
              archived: false,
            },
            app: "dashboard",
            route: { pathname: "/", search: {} },
          },
        ],
        total: 1,
        loading: false,
        error: null,
      },
    ];

    renderPalette();

    const row = screen.getByTestId("os-palette-domain-row-worktree:agent-comms");
    expect(within(row).getByTestId("profile-owner-tag")).toHaveTextContent("default");
  });

  it("Should keep matching commands and settings destinations in separate groups [UT-117]", async () => {
    const user = userEvent.setup();
    paletteMocks.rankSignals = TEST_RANK_SIGNALS;
    paletteMocks.domainSections = [
      {
        title: "Settings",
        rows: [
          {
            key: "settings:shortcuts",
            label: "Keyboard shortcuts",
            app: "settings",
            route: { pathname: "/settings/shortcuts", search: {} },
          },
        ],
        total: 1,
        loading: false,
        error: null,
      },
    ];
    const shortcut = paletteCommand({
      ...PALETTE_REGISTRY.commands[0],
      id: "cheatsheet.shortcuts",
      title: "Keyboard shortcuts",
      section: "Commands",
    });
    const registry = {
      ...PALETTE_REGISTRY,
      commands: [...PALETTE_REGISTRY.commands, shortcut],
      byId: new Map([...PALETTE_REGISTRY.byId, [shortcut.id, shortcut]]),
    };
    render(
      <CmdPaletteRegistryProvider registry={registry}>
        <OsCommandPalette open dispatch={paletteDispatch} onOpenChange={vi.fn()} />
      </CmdPaletteRegistryProvider>
    );

    await user.type(
      screen.getByPlaceholderText("Search apps, sessions, and actions…"),
      "shortcuts"
    );
    expect(screen.getByTestId("os-palette-section-commands")).toBeInTheDocument();
    expect(screen.getByTestId("os-palette-domain-settings")).toBeInTheDocument();
  });

  it("Should render and accept a casing-preserving ghost only at the input end [UT-118, UT-119]", async () => {
    const user = userEvent.setup();
    paletteMocks.rankSignals = TEST_RANK_SIGNALS;
    renderPalette();
    const input = screen.getByPlaceholderText(
      "Search apps, sessions, and actions…"
    ) as HTMLInputElement;

    await user.type(input, "Ne");
    expect(screen.getByTestId("os-palette-ghost")).toHaveTextContent("w tab");
    await user.keyboard("{ArrowRight}");
    expect(input).toHaveValue("New tab");

    await user.clear(input);
    await user.type(input, "Ne");
    input.setSelectionRange(1, 1);
    await user.keyboard("{ArrowRight}");
    expect(input).toHaveValue("Ne");
  });
});

describe("palette execution surfaces", () => {
  function renderExecutionPalette(onOpenChange = vi.fn()) {
    // A fresh element per pass: re-rendering the identical element lets React
    // bail out, and these cases are about the palette reacting to a catalog that
    // moved underneath it.
    const tree = () => (
      <ExecutionHarness>
        <OsCommandPalette open dispatch={paletteDispatch} onOpenChange={onOpenChange} />
      </ExecutionHarness>
    );
    const result = render(tree());
    return { ...result, onOpenChange, refresh: () => result.rerender(tree()) };
  }

  beforeEach(() => {
    resetPaletteHarness();
    paletteMocks.desktop = desktopFixture({}, null);
  });

  afterEach(() => {
    resetPaletteExecutionEntry();
    cmdPaletteExecutionStore.trigger.pendingSettled({ commandId: CAPTURE_COMMAND.id });
  });

  it("Should toggle the action panel on the selected row, filter it, and close it [UT-125]", async () => {
    const user = userEvent.setup();
    renderExecutionPalette();

    await user.keyboard("{Meta>}k{/Meta}");
    const panel = await screen.findByTestId("os-palette-action-panel");
    expect(within(panel).getByTestId("os-palette-action-primary.run")).toHaveTextContent(
      "Command palette"
    );
    expect(within(panel).getByTestId("os-palette-action-meta.pin")).toHaveTextContent("Pin");

    await user.type(screen.getByPlaceholderText("Filter actions…"), "pin");
    expect(within(panel).getByTestId("os-palette-action-meta.pin")).toBeInTheDocument();
    expect(within(panel).queryByTestId("os-palette-action-primary.run")).not.toBeInTheDocument();

    await user.keyboard("{Meta>}k{/Meta}");
    await waitFor(() =>
      expect(screen.queryByTestId("os-palette-action-panel")).not.toBeInTheDocument()
    );
  });

  it("Should render a command's alias beside its title [UT-149]", () => {
    // The alias is the operator's own word for the command, so the row shows it
    // where they will look for it rather than hiding it behind a search match.
    renderExecutionPalette();

    expect(screen.getByTestId("os-palette-command-ext.notes.capture")).toHaveTextContent(
      "Capture note (cap)"
    );
  });

  it("Should keep a filter that matches nothing open and honest [UT-125]", async () => {
    const user = userEvent.setup();
    renderExecutionPalette();
    await user.keyboard("{Meta>}k{/Meta}");
    await screen.findByTestId("os-palette-action-panel");

    await user.type(screen.getByPlaceholderText("Filter actions…"), "xyz");
    expect(screen.getByTestId("os-palette-action-empty")).toHaveTextContent("No actions match");
    expect(screen.getByTestId("os-palette-action-panel")).toBeInTheDocument();
  });

  it("Should pin through the seam and deep-link alias and shortcut to the settings table [UT-126]", async () => {
    const user = userEvent.setup();
    renderExecutionPalette();
    await user.keyboard("{Meta>}k{/Meta}");
    await screen.findByTestId("os-palette-action-panel");
    await user.click(screen.getByTestId("os-palette-action-meta.pin"));
    expect(paletteDispatch.setPinned).toHaveBeenCalledWith(
      expect.objectContaining({ id: "palette.open" }),
      true
    );

    await user.keyboard("{Meta>}k{/Meta}");
    await screen.findByTestId("os-palette-action-panel");
    await user.click(screen.getByTestId("os-palette-action-meta.alias"));
    // The deep link carries the command, so the whole-registry table lands on
    // that row instead of on a list the operator has to search again.
    expect(paletteMocks.coordinator.userOpen).toHaveBeenCalledWith({
      app: "settings",
      route: { pathname: "/settings/layouts", search: { command: "palette.open" } },
    });

    await user.keyboard("{Meta>}k{/Meta}");
    await screen.findByTestId("os-palette-action-panel");
    await user.click(screen.getByTestId("os-palette-action-meta.shortcut"));
    expect(paletteMocks.coordinator.userOpen).toHaveBeenLastCalledWith({
      app: "settings",
      route: { pathname: "/settings/layouts", search: { command: "palette.open" } },
    });
  });

  it("Should list only meta-actions plus the verbatim reason on an unavailable row [UT-128]", async () => {
    const user = userEvent.setup();
    renderExecutionPalette();
    const row = screen.getByTestId("os-palette-command-ext.notes.recent");
    expect(row).not.toHaveAttribute("aria-disabled", "true");
    expect(row).toHaveAccessibleName(/Recent notes/);
    expect(row).not.toHaveAccessibleName(/extension notes is unhealthy \(crash loop\)/);
    expect(row).toHaveAccessibleDescription("extension notes is unhealthy (crash loop)");

    // The unavailable primary action does not disable the composite option:
    // after filtering, Home reaches it through cmdk's keyboard model, then
    // Enter reaches the one dispatch seam that refuses the run with the
    // runtime's reason.
    await user.type(
      screen.getByPlaceholderText("Search apps, sessions, and actions…"),
      "Recent notes"
    );
    await user.keyboard("{Home}");
    expect(row).toHaveAttribute("data-selected", "true");
    await user.keyboard("{Enter}");
    await waitFor(() =>
      expect(paletteDispatch.run).toHaveBeenCalledWith(
        expect.objectContaining({ id: "ext.notes.recent", available: false }),
        { query: "Recent notes" }
      )
    );
    expect(row).toBeInTheDocument();
    await user.keyboard("{Meta>}k{/Meta}");
    const panel = await screen.findByTestId("os-palette-action-panel");

    expect(within(panel).queryByTestId("os-palette-action-primary.run")).not.toBeInTheDocument();
    expect(within(panel).getByTestId("os-palette-action-meta.pin")).toBeInTheDocument();
    expect(within(panel).getByTestId("os-palette-action-reason")).toHaveTextContent(
      "extension notes is unhealthy (crash loop)"
    );
  });

  it("Should close the panel and fire nothing when its row leaves the list [UT-127]", async () => {
    const user = userEvent.setup();
    paletteMocks.sessions = [
      paletteSession({ id: "session:one", name: "Refactor session store" }),
      paletteSession({ id: "session:two", name: "Fix payment retries" }),
    ];
    const { refresh } = renderExecutionPalette();

    await user.hover(screen.getByTestId("os-palette-session-session:one"));
    await user.keyboard("{Meta>}k{/Meta}");
    expect(await screen.findByTestId("os-palette-action-session.land")).toBeInTheDocument();

    paletteMocks.sessions = [paletteSession({ id: "session:two", name: "Fix payment retries" })];
    refresh();

    await waitFor(() =>
      expect(screen.queryByTestId("os-palette-action-panel")).not.toBeInTheDocument()
    );
    expect(paletteMocks.jumpToSession).not.toHaveBeenCalled();
    expect(screen.getByTestId("os-palette-session-session:two")).toHaveAttribute(
      "data-selected",
      "true"
    );
  });

  it("Should open in argument mode when the seam asks for arguments [UT-122]", async () => {
    requestPaletteArgs(CAPTURE_COMMAND);
    renderExecutionPalette();

    expect(screen.getByTestId("os-palette-args")).toHaveTextContent("Capture note");
    expect(screen.getByTestId("os-palette-arg-title")).toHaveValue("");
    expect(screen.getByTestId("os-palette-arg-title")).toHaveAttribute("placeholder", "Note title");
    expect(screen.queryByPlaceholderText("Search apps, sessions, and actions…")).toBeNull();
  });

  it("Should traverse fields and run once every required argument is filled [UT-120]", async () => {
    const user = userEvent.setup();
    requestPaletteArgs(CAPTURE_COMMAND);
    renderExecutionPalette();

    await user.type(screen.getByTestId("os-palette-arg-title"), "Standup follow-ups");
    await user.tab();
    expect(screen.getByTestId("os-palette-arg-tag")).toHaveFocus();
    await user.keyboard("inbox");
    // ⏎ with the option list open picks the highlighted option; the next one
    // submits the bar.
    await user.keyboard("{Enter}");
    await user.keyboard("{Enter}");

    expect(paletteDispatch.run).toHaveBeenCalledWith(
      expect.objectContaining({ id: "ext.notes.capture" }),
      { args: { title: "Standup follow-ups", tag: "inbox" } }
    );
  });

  it("Should block on the empty required field and restore search on Escape [UT-121]", async () => {
    const user = userEvent.setup();
    requestPaletteArgs(CAPTURE_COMMAND);
    renderExecutionPalette();

    await user.click(screen.getByTestId("os-palette-arg-title"));
    await user.keyboard("{Enter}");
    expect(screen.getByTestId("os-palette-arg-error-title")).toHaveTextContent("required");
    expect(screen.getByTestId("os-palette-arg-title")).toHaveFocus();
    expect(paletteDispatch.run).not.toHaveBeenCalled();

    await user.type(screen.getByTestId("os-palette-arg-title"), "Standup");
    await user.keyboard("{Escape}");
    expect(screen.queryByTestId("os-palette-args")).toBeNull();
    expect(screen.getByPlaceholderText("Search apps, sessions, and actions…")).toBeInTheDocument();

    // Re-entering starts clean: the discarded value never comes back.
    requestPaletteArgs(CAPTURE_COMMAND);
    await waitFor(() => expect(screen.getByTestId("os-palette-arg-title")).toHaveValue(""));
  });

  it("Should type-to-filter a dropdown argument [UT-120]", async () => {
    const user = userEvent.setup();
    requestPaletteArgs(CAPTURE_COMMAND);
    renderExecutionPalette();

    await user.click(screen.getByTestId("os-palette-arg-tag"));
    const options = screen.getByTestId("os-palette-arg-options-tag");
    expect(
      within(options)
        .getAllByRole("option")
        .map(node => node.textContent)
    ).toEqual(["inbox", "idea"]);
    await user.keyboard("in");
    expect(
      within(screen.getByTestId("os-palette-arg-options-tag"))
        .getAllByRole("option")
        .map(node => node.textContent)
    ).toEqual(["inbox"]);
  });

  it("Should keep dropdown options out of the Tab order while the field owns focus [UT-120]", async () => {
    const user = userEvent.setup();
    requestPaletteArgs(CAPTURE_COMMAND);
    renderExecutionPalette();

    const field = screen.getByTestId("os-palette-arg-tag");
    await user.click(field);
    for (const option of within(screen.getByTestId("os-palette-arg-options-tag")).getAllByRole(
      "option"
    )) {
      expect(option).toHaveAttribute("tabindex", "-1");
    }
    // ⇥ leaves the field entirely rather than stepping into its own option list;
    // options are reached with the arrows through aria-activedescendant.
    await user.tab();
    expect(field).not.toHaveFocus();
    expect(document.activeElement).not.toHaveAttribute("role", "option");
  });

  it("Should let Escape reach the argument step even with a dropdown open [UT-121]", async () => {
    const user = userEvent.setup();
    requestPaletteArgs(CAPTURE_COMMAND);
    const { onOpenChange } = renderExecutionPalette();

    await user.type(screen.getByTestId("os-palette-arg-title"), "Standup follow-ups");
    await user.click(screen.getByTestId("os-palette-arg-tag"));
    expect(screen.getByTestId("os-palette-arg-options-tag")).toBeInTheDocument();

    // The open list is not a rung of the ladder: one Escape leaves argument mode
    // outright and discards what was typed, rather than only dismissing options.
    await user.keyboard("{Escape}");
    expect(screen.queryByTestId("os-palette-args")).toBeNull();
    expect(screen.getByPlaceholderText("Search apps, sessions, and actions…")).toBeInTheDocument();
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
    expect(paletteDispatch.run).not.toHaveBeenCalled();
  });

  it("Should render the declared confirmation with Cancel focused and Esc backing out [UT-123]", async () => {
    const user = userEvent.setup();
    requestPaletteConfirmation(PURGE_COMMAND, {});
    renderExecutionPalette();

    expect(screen.getByTestId("os-palette-confirmation")).toHaveTextContent(
      "Purge archived notes?"
    );
    expect(screen.getByTestId("os-palette-confirmation")).toHaveTextContent(
      "Permanently deletes every archived note in this workspace."
    );
    await waitFor(() => expect(screen.getByTestId("os-palette-confirm-cancel")).toHaveFocus());
    expect(screen.getByTestId("os-palette-confirm-accept")).toHaveTextContent("Purge");

    await user.keyboard("{Escape}");
    expect(screen.queryByTestId("os-palette-confirmation")).toBeNull();
    expect(paletteDispatch.run).not.toHaveBeenCalled();
  });

  it("Should absorb the triggering keystroke instead of confirming with it [UT-124]", async () => {
    const user = userEvent.setup();
    renderExecutionPalette();
    await user.hover(screen.getByTestId("os-palette-command-ext.notes.purge"));
    await user.keyboard("{Enter}");
    expect(await screen.findByTestId("os-palette-confirmation")).toBeInTheDocument();
    expect(paletteDispatch.run).toHaveBeenCalledWith(
      expect.objectContaining({ id: "ext.notes.purge" }),
      expect.not.objectContaining({ confirmed: true })
    );
    await waitFor(() => expect(screen.getByTestId("os-palette-confirm-cancel")).toHaveFocus());
    // The Enter that opened the step must not also dismiss or confirm it.
    expect(screen.getByTestId("os-palette-confirmation")).toBeInTheDocument();

    await user.keyboard("{Enter}");
    expect(paletteDispatch.run).not.toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ confirmed: true })
    );
    expect(screen.queryByTestId("os-palette-confirmation")).toBeNull();
  });

  it("Should confirm only through a deliberate activation of the confirm control [UT-124]", async () => {
    const user = userEvent.setup();
    requestPaletteConfirmation(PURGE_COMMAND, { scope: "workspace" });
    renderExecutionPalette();

    await user.click(screen.getByTestId("os-palette-confirm-accept"));
    expect(paletteDispatch.run).toHaveBeenCalledWith(
      expect.objectContaining({ id: "ext.notes.purge" }),
      { args: { scope: "workspace" }, confirmed: true }
    );
  });

  it("Should show the honest message instead of executing when the target changed [UT-124]", async () => {
    const user = userEvent.setup();
    // The command the operator triggered is no longer the command the catalog
    // carries — the confirmation must not run against the difference.
    requestPaletteConfirmation(
      paletteCommand({ ...UNHEALTHY_COMMAND, confirmation: PURGE_COMMAND.confirmation }),
      {}
    );
    renderExecutionPalette();

    expect(screen.getByTestId("os-palette-confirm-invalid")).toHaveTextContent(
      "extension notes is unhealthy (crash loop)"
    );
    expect(screen.queryByTestId("os-palette-confirm-accept")).toBeNull();
    await user.click(screen.getByTestId("os-palette-confirm-cancel"));
    expect(paletteDispatch.run).not.toHaveBeenCalled();
  });

  it("Should close on a synchronous command and stay open while one is pending [UT-159]", async () => {
    const user = userEvent.setup();
    const { onOpenChange, refresh } = renderExecutionPalette();

    await user.click(screen.getByTestId("os-palette-command-settings.layouts"));
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false));

    cmdPaletteExecutionStore.trigger.pendingStarted({
      pending: { commandId: CAPTURE_COMMAND.id, title: CAPTURE_COMMAND.title },
    });
    refresh();
    const row = screen.getByTestId("os-palette-command-ext.notes.capture");
    expect(row).toHaveAttribute("aria-busy", "true");
    expect(within(row).getByTestId("os-palette-pending-ext.notes.capture")).toHaveTextContent(
      "pending"
    );
  });

  it("Should name the workspace a session landing switched to [UT-159]", async () => {
    const user = userEvent.setup();
    paletteMocks.sessions = [
      paletteSession({
        id: "session:foreign",
        name: "Fix payment retries",
        workspace_id: "workspace:beta",
      }),
    ];
    renderExecutionPalette();

    await user.click(screen.getByTestId("os-palette-session-session:foreign"));
    expect(paletteMocks.notifyUser).toHaveBeenCalledWith({
      message: "Switched to Beta to open Fix payment retries",
      tone: "info",
      retryable: false,
    });
    expect(paletteMocks.jumpToSession).toHaveBeenCalledWith({
      sessionId: "session:foreign",
      agentName: "claude",
      workspaceId: "workspace:beta",
    });
  });

  it("Should not announce a switch for a landing inside the current workspace [UT-159]", async () => {
    const user = userEvent.setup();
    paletteMocks.sessions = [
      paletteSession({ id: "session:local", name: "Refactor session store" }),
    ];
    renderExecutionPalette();

    await user.click(screen.getByTestId("os-palette-session-session:local"));
    expect(paletteMocks.notifyUser).not.toHaveBeenCalled();
    expect(paletteMocks.jumpToSession).toHaveBeenCalledOnce();
  });

  it("Should climb the Escape ladder panel first, then the step, then the palette [ladder]", async () => {
    const user = userEvent.setup();
    const { onOpenChange } = renderExecutionPalette();

    await user.keyboard("{Meta>}k{/Meta}");
    await screen.findByTestId("os-palette-action-panel");
    await user.keyboard("{Escape}");
    await waitFor(() =>
      expect(screen.queryByTestId("os-palette-action-panel")).not.toBeInTheDocument()
    );
    expect(onOpenChange).not.toHaveBeenCalledWith(false);

    requestPaletteArgs(CAPTURE_COMMAND);
    await waitFor(() => expect(screen.getByTestId("os-palette-args")).toBeInTheDocument());
    await user.keyboard("{Escape}");
    expect(screen.queryByTestId("os-palette-args")).toBeNull();
    expect(onOpenChange).not.toHaveBeenCalledWith(false);

    await user.keyboard("{Escape}");
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});
