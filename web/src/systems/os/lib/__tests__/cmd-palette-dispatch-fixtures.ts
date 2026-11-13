import { vi } from "vitest";

import type { PaletteShellHandlers } from "../cmd-palette-client-ops";
import type { PaletteDispatchPorts } from "../cmd-palette-dispatch";
import type { ResolvedPaletteCommand } from "../cmd-palette-types";
import type { WindowManagerController } from "../os-types";
import type { LayoutDesktop } from "../window-manager-types";

/**
 * The fields the client-op table actually reads. The full desktop store is
 * larger; this named fixture keeps that seam explicit instead of casting
 * through unknown.
 */
export interface PaletteClientOpRuntimeState {
  focusedId: string | null;
  activeDesktopId: string;
  activeTabIds: Record<string, string>;
  windows: Record<string, { id: string; desktopId: string }>;
  client: { focusOrder: string[] };
  /** Desktop-order tests read only `id` and `order`; tab ops read the active desktop's bar. */
  desktops: (Pick<LayoutDesktop, "id" | "order"> &
    Partial<Pick<LayoutDesktop, "tabs" | "activeTabId">>)[];
  windowManagerConfig: Record<string, never>;
}

/** The active desktop's tab bar: one single-window tab per id, in bar order. */
export interface TabBarFixture {
  tabs: readonly string[];
  activeWindowId: string;
}

export function tabBarFixture(overrides: Partial<TabBarFixture> = {}): TabBarFixture {
  return { tabs: ["w-1", "w-2", "w-3"], activeWindowId: "w-2", ...overrides };
}

function barDesktop(
  bar: TabBarFixture
): Pick<LayoutDesktop, "id" | "order" | "tabs" | "activeTabId"> {
  return {
    id: "desktop:a",
    order: 0,
    activeTabId: `tab:${bar.activeWindowId}`,
    tabs: bar.tabs.map(windowId => ({
      id: `tab:${windowId}`,
      pinned: false,
      root: { id: `leaf:${windowId}`, kind: "leaf" as const, windowId },
    })),
  };
}

export function shellFixture(): PaletteShellHandlers {
  return {
    openPalette: vi.fn(),
    openPaletteView: vi.fn(),
    openCheatsheet: vi.fn(),
    openLeftRunning: vi.fn(),
    openDesktops: vi.fn(),
    openWorkspaces: vi.fn(),
    openNewSession: vi.fn(),
    toggleSessions: vi.fn(),
    toggleSidebar: vi.fn(),
    toggleAllProjects: vi.fn(),
    useProfile: vi.fn(),
    cycleWorkspace: vi.fn(),
    cycleSession: vi.fn(),
    focusAttention: vi.fn(),
    openNewTab: vi.fn(),
    activateWindow: vi.fn(),
    focusDirection: vi.fn(),
    openPaletteExecution: vi.fn(),
  };
}

export function paletteClientOpState(
  bar: TabBarFixture | null,
  focusedId: string | null
): PaletteClientOpRuntimeState {
  return {
    focusedId,
    activeDesktopId: "desktop:a",
    activeTabIds: bar ? { "desktop:a": `tab:${bar.activeWindowId}` } : {},
    windows: {
      "w-1": { id: "w-1", desktopId: "desktop:a" },
      "w-2": { id: "w-2", desktopId: "desktop:a" },
      "w-3": { id: "w-3", desktopId: "desktop:a" },
    },
    client: { focusOrder: ["w-2", "w-1", "w-3"] },
    desktops: bar ? [barDesktop(bar)] : [],
    windowManagerConfig: {},
  };
}

export function opContext(bar: TabBarFixture | null, focusedId: string | null) {
  const state = paletteClientOpState(bar, focusedId);
  const manager = {
    getState: () => state,
    reopenWindow: vi.fn(() => ({ accepted: true, completion: Promise.resolve(true) })),
    switchDesktop: vi.fn(),
    switchDesktopDirection: vi.fn(),
    moveWindowToDesktop: vi.fn(),
    createDesktop: vi.fn(),
    closeWindow: vi.fn(() => Promise.resolve(true)),
    arrangeLayout: vi.fn(),
    focusDirection: vi.fn(),
    swapDirection: vi.fn(),
    unsplitWindow: vi.fn(),
    tileWindow: vi.fn(),
    moveTab: vi.fn(),
  } as unknown as WindowManagerController & { getState: () => PaletteClientOpRuntimeState };
  return {
    manager,
    shell: shellFixture(),
    state,
    navigate: vi.fn<(app: string, pathname: string | null) => void>(),
    openUrl: vi.fn<(url: string) => void>(),
  };
}

export function paletteCommand(
  overrides: Partial<ResolvedPaletteCommand> = {}
): ResolvedPaletteCommand {
  const command: ResolvedPaletteCommand = {
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
  };
  return command;
}

export function portsFixture(overrides: Partial<PaletteDispatchPorts> = {}) {
  const context = opContext(tabBarFixture(), "w-2");
  const ports: PaletteDispatchPorts = {
    clientOps: {
      manager: context.manager,
      shell: context.shell,
      navigate: context.navigate,
      openUrl: context.openUrl,
    },
    invoke: vi.fn(async () => ({
      status: "ok",
      invocation_id: "inv-fixture",
      profile_lens: { profile_lens_id: "00000000000000000000000000", profile_name: "default" },
    })),
    navigate: vi.fn(),
    pushView: vi.fn(),
    openUrl: vi.fn(),
    copyToClipboard: vi.fn(async () => undefined),
    reportUsage: vi.fn(),
    refresh: vi.fn(),
    onFailure: vi.fn(),
    requestArgs: vi.fn(),
    requestConfirmation: vi.fn(),
    onPendingStart: vi.fn(),
    onPendingSettle: vi.fn(),
    onCompleted: vi.fn(),
    ...overrides,
  };
  return { ports, context };
}
