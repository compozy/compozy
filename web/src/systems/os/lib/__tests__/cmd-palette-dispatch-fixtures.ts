import { vi } from "vitest";

import type { PaletteShellHandlers } from "../cmd-palette-client-ops";
import type { PaletteDispatchPorts } from "../cmd-palette-dispatch";
import type { ResolvedPaletteCommand } from "../cmd-palette-types";
import type { OsWindowFrameModel } from "../group-projection";
import type { WindowManagerController } from "../os-types";

/**
 * The fields the client-op table actually reads. The full desktop store is
 * larger; this named fixture keeps that seam explicit instead of casting
 * through unknown.
 */
export interface PaletteClientOpRuntimeState {
  focusedId: string | null;
  activeDesktopId: string;
  frames: Record<string, OsWindowFrameModel[]>;
  windows: Record<string, { id: string; desktopId: string; minimized: boolean }>;
  client: { focusOrder: string[] };
  desktops: { id: string; order: number }[];
  windowManagerConfig: Record<string, never>;
}

export function frameFixture(overrides: Partial<OsWindowFrameModel> = {}): OsWindowFrameModel {
  return {
    id: "fs-1",
    desktopId: "desktop:a",
    kind: "floating",
    rect: { x: 0, y: 0, w: 600, h: 400 },
    members: ["w-1", "w-2", "w-3"],
    activeWindowId: "w-2",
    stackId: "fs-1",
    minimized: false,
    adapted: false,
    layer: 1,
    zone: null,
    resizableEdges: { left: true, right: true, top: true, bottom: true },
    ...overrides,
  };
}

export function shellFixture(): PaletteShellHandlers {
  return {
    openPalette: vi.fn(),
    openPaletteView: vi.fn(),
    openCheatsheet: vi.fn(),
    openDesktops: vi.fn(),
    openWorkspaces: vi.fn(),
    openNewSession: vi.fn(),
    toggleSessions: vi.fn(),
    toggleSidebar: vi.fn(),
    toggleGlobalScope: vi.fn(),
    useProfile: vi.fn(),
    cycleWorkspace: vi.fn(),
    cycleSession: vi.fn(),
    focusAttention: vi.fn(),
    openNewTab: vi.fn(),
    activateWindow: vi.fn(),
    openPaletteExecution: vi.fn(),
  };
}

export function paletteClientOpState(
  frame: OsWindowFrameModel | null,
  focusedId: string | null
): PaletteClientOpRuntimeState {
  return {
    focusedId,
    activeDesktopId: "desktop:a",
    frames: frame ? { [frame.desktopId]: [frame] } : {},
    windows: {
      "w-1": { id: "w-1", desktopId: "desktop:a", minimized: false },
      "w-2": { id: "w-2", desktopId: "desktop:a", minimized: false },
      "w-3": { id: "w-3", desktopId: "desktop:a", minimized: false },
    },
    client: { focusOrder: ["w-2", "w-1", "w-3"] },
    desktops: [],
    windowManagerConfig: {},
  };
}

export function opContext(frame: OsWindowFrameModel | null, focusedId: string | null) {
  const state = paletteClientOpState(frame, focusedId);
  const manager = {
    getState: () => state,
    reopenWindow: vi.fn(() => ({ accepted: true, completion: Promise.resolve(true) })),
    switchDesktop: vi.fn(),
    switchDesktopDirection: vi.fn(),
    createDesktop: vi.fn(),
    closeWindow: vi.fn(() => Promise.resolve(true)),
    groupWindows: vi.fn(),
    toggleFloating: vi.fn(() => ({ accepted: true, completion: Promise.resolve(true) })),
    tileWindow: vi.fn(),
    arrangeLayout: vi.fn(),
    focusDirection: vi.fn(),
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
  const context = opContext(frameFixture(), "w-2");
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
