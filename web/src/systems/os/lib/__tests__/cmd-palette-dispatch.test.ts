// Suite: cmd-palette dispatch seam
// Invariant: one seam routes every action kind to exactly one effect — client
// operations to the client-op table, tool actions to the daemon invoke,
// navigate/view/url to their shell owners — refuses an unavailable command with
// the runtime's verbatim reason, and surfaces a stale target honestly instead of
// half-executing. The client-op table itself resolves tab, focus, desktop and
// layout ids to exactly one controller call.
// Boundary IN: action-kind routing, refusal, and the client-op lookup.
// Boundary OUT: keyboard event matching (window-manager-shortcuts suite),
// availability evaluation (cmd-palette-availability suite), and transport.
import { describe, expect, it, vi } from "vitest";

import { paletteClientOp, type PaletteShellHandlers } from "../cmd-palette-client-ops";
import {
  ALREADY_RUNNING_CODE,
  canRetry,
  invokeCompletedFeedback,
  invokeFailedFeedback,
  workspaceSwitchFeedback,
} from "../cmd-palette-feedback";
import {
  dispatchPaletteCommand,
  STALE_TARGET_REASON,
  UNSUPPORTED_CLIENT_OP_REASON,
  type PaletteDispatchPorts,
} from "../cmd-palette-dispatch";
import type { ResolvedPaletteCommand } from "../cmd-palette-types";
import type { OsWindowFrameModel } from "../group-projection";
import type { OsDesktopRuntimeStore, WindowManagerController } from "../os-types";

function frameFixture(overrides: Partial<OsWindowFrameModel> = {}): OsWindowFrameModel {
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

function shellFixture(): PaletteShellHandlers {
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
    cycleWorkspace: vi.fn(),
    cycleSession: vi.fn(),
    focusAttention: vi.fn(),
    openNewTab: vi.fn(),
    activateWindow: vi.fn(),
    openPaletteExecution: vi.fn(),
  };
}

function opContext(frame: OsWindowFrameModel | null, focusedId: string | null) {
  const state = {
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
  } as unknown as OsDesktopRuntimeStore;
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
  } as unknown as WindowManagerController;
  return { manager, shell: shellFixture(), state };
}

function run(op: string, context: ReturnType<typeof opContext>) {
  const handler = paletteClientOp(op);
  if (handler === null) throw new Error(`no handler for ${op}`);
  return handler({ manager: context.manager, shell: context.shell }, {});
}

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

function portsFixture(overrides: Partial<PaletteDispatchPorts> = {}) {
  const context = opContext(frameFixture(), "w-2");
  const ports: PaletteDispatchPorts = {
    clientOps: { manager: context.manager, shell: context.shell },
    invoke: vi.fn(async () => ({ status: "ok" })),
    navigate: vi.fn(),
    pushView: vi.fn(),
    openUrl: vi.fn(),
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

describe("cmd-palette dispatch seam (UT-105)", () => {
  it("Should route a client_op to the client-op table and record usage", async () => {
    const { ports, context } = portsFixture();
    const outcome = await dispatchPaletteCommand({ command: command(), ports });
    expect(outcome).toEqual({ status: "ran" });
    expect(context.manager.closeWindow).toHaveBeenCalledWith("w-2");
    expect(ports.invoke).not.toHaveBeenCalled();
    expect(ports.reportUsage).toHaveBeenCalledWith("window.close", "");
  });

  it("Should route a tool action to the daemon invoke and leave usage to the daemon", async () => {
    const { ports } = portsFixture();
    const outcome = await dispatchPaletteCommand({
      command: command({
        id: "ext.notes.capture",
        action: { kind: "tool", tool: "ext.notes.capture" },
      }),
      args: { title: "Standup" },
      ports,
    });
    expect(outcome).toEqual({ status: "invoked", result: { status: "ok" } });
    expect(ports.invoke).toHaveBeenCalledWith("ext.notes.capture", { title: "Standup" });
    expect(ports.reportUsage).not.toHaveBeenCalled();
  });

  it("Should route navigate, view and url actions to their shell owners", async () => {
    const { ports } = portsFixture();
    await dispatchPaletteCommand({
      command: command({
        id: "settings.appearance",
        action: {
          kind: "navigate",
          app: "settings",
          args: { pathname: "/settings/appearance" },
        },
      }),
      ports,
    });
    expect(ports.navigate).toHaveBeenCalledWith("settings", "/settings/appearance");

    await dispatchPaletteCommand({
      command: command({
        id: "palette.view.sessions",
        action: { kind: "view", view: "sessions" },
      }),
      ports,
    });
    expect(ports.pushView).toHaveBeenCalledWith("sessions");

    await dispatchPaletteCommand({
      command: command({
        id: "help.docs",
        action: { kind: "url", url: "https://compozy.com/docs" },
      }),
      ports,
    });
    expect(ports.openUrl).toHaveBeenCalledWith("https://compozy.com/docs");
  });

  it("Should merge declared action args under caller-supplied values", async () => {
    const { ports } = portsFixture();
    await dispatchPaletteCommand({
      command: command({
        id: "ext.notes.capture",
        action: { kind: "tool", tool: "ext.notes.capture", args: { folder: "inbox" } },
      }),
      args: { title: "Standup" },
      ports,
    });
    expect(ports.invoke).toHaveBeenCalledWith("ext.notes.capture", {
      folder: "inbox",
      title: "Standup",
    });
  });
});

describe("cmd-palette dispatch refusals (UT-106)", () => {
  it("Should refuse an unavailable command with the runtime reason instead of running it", async () => {
    const { ports, context } = portsFixture();
    const unavailable = command({
      available: false,
      reason: "needs two windows on this desktop",
    });
    const outcome = await dispatchPaletteCommand({ command: unavailable, ports });
    expect(outcome).toEqual({
      status: "refused",
      reason: "needs two windows on this desktop",
    });
    expect(context.manager.closeWindow).not.toHaveBeenCalled();
    expect(ports.onFailure).toHaveBeenCalledWith(unavailable, "needs two windows on this desktop");
  });

  it("Should surface a stale target honestly when the invoke rejects", async () => {
    const { ports } = portsFixture({
      invoke: vi.fn(async () => {
        throw new Error("Session no longer exists");
      }),
    });
    const stale = command({
      id: "session.open",
      action: { kind: "tool", tool: "session.open" },
    });
    const outcome = await dispatchPaletteCommand({ command: stale, ports });
    expect(outcome).toEqual({ status: "refused", reason: "Session no longer exists" });
    // A plain transport failure carries no daemon code, so retry gating has
    // nothing to key on and the reason travels alone.
    expect(ports.onFailure).toHaveBeenCalledWith(stale, "Session no longer exists", undefined);
    expect(ports.refresh).toHaveBeenCalledOnce();
  });

  it("Should refuse a client operation this build cannot perform rather than failing silently", async () => {
    const { ports } = portsFixture();
    const outcome = await dispatchPaletteCommand({
      command: command({ id: "window.swap", action: { kind: "client_op", op: "window.swap" } }),
      ports,
    });
    expect(outcome).toEqual({ status: "refused", reason: UNSUPPORTED_CLIENT_OP_REASON });
  });

  it("Should fall back to an honest reason when an unavailable command carries none", async () => {
    const { ports } = portsFixture();
    const outcome = await dispatchPaletteCommand({
      command: command({ available: false, reason: "" }),
      ports,
    });
    expect(outcome).toEqual({ status: "refused", reason: STALE_TARGET_REASON });
  });
});

describe("cmd-palette pre-execution gates (UT-120, UT-123)", () => {
  const withArguments = command({
    id: "ext.notes.capture",
    title: "Capture note",
    action: { kind: "tool", tool: "ext__notes__capture" },
    arguments: [{ name: "title", type: "text", required: true, placeholder: "Note title" }],
  });
  const withConfirmation = command({
    id: "ext.notes.purge",
    title: "Purge archived notes",
    action: { kind: "tool", tool: "ext__notes__purge" },
    destructive: true,
    confirmation: {
      title: "Purge archived notes?",
      body: "Permanently deletes every archived note in this workspace.",
      confirm: "Purge",
    },
  });

  it("Should raise the argument step instead of running a command that declares arguments", async () => {
    const { ports } = portsFixture();
    const outcome = await dispatchPaletteCommand({ command: withArguments, ports });
    expect(outcome).toEqual({ status: "needs_args" });
    expect(ports.requestArgs).toHaveBeenCalledWith(withArguments);
    expect(ports.invoke).not.toHaveBeenCalled();
  });

  it("Should run once the arguments arrive", async () => {
    const { ports } = portsFixture();
    const outcome = await dispatchPaletteCommand({
      command: withArguments,
      args: { title: "Standup follow-ups" },
      ports,
    });
    expect(outcome).toEqual({ status: "invoked", result: { status: "ok" } });
    expect(ports.requestArgs).not.toHaveBeenCalled();
  });

  it("Should raise the confirmation step carrying the arguments already collected", async () => {
    const { ports } = portsFixture();
    const outcome = await dispatchPaletteCommand({
      command: withConfirmation,
      args: { scope: "workspace" },
      ports,
    });
    expect(outcome).toEqual({ status: "needs_confirmation" });
    expect(ports.requestConfirmation).toHaveBeenCalledWith(withConfirmation, {
      scope: "workspace",
    });
    expect(ports.invoke).not.toHaveBeenCalled();
  });

  it("Should run a confirmed destructive command exactly once", async () => {
    const { ports } = portsFixture();
    const outcome = await dispatchPaletteCommand({
      command: withConfirmation,
      args: { scope: "workspace" },
      confirmed: true,
      ports,
    });
    expect(outcome).toEqual({ status: "invoked", result: { status: "ok" } });
    expect(ports.invoke).toHaveBeenCalledOnce();
    expect(ports.requestConfirmation).not.toHaveBeenCalled();
  });

  it("Should refuse an unavailable command before asking for anything", async () => {
    const { ports } = portsFixture();
    await dispatchPaletteCommand({
      command: command({
        ...withArguments,
        available: false,
        reason: "extension notes is unhealthy (crash loop)",
      }),
      ports,
    });
    expect(ports.requestArgs).not.toHaveBeenCalled();
  });
});

describe("cmd-palette feedback lifecycle (UT-159, UT-160)", () => {
  const asyncCommand = command({
    id: "ext.notes.capture",
    title: "Capture note",
    action: { kind: "tool", tool: "ext__notes__capture" },
  });

  it("Should hold the command pending across the invoke and report its completion", async () => {
    const order: string[] = [];
    const { ports } = portsFixture({
      invoke: vi.fn(async () => {
        order.push("invoke");
        return { status: "ok" };
      }),
      onPendingStart: vi.fn(() => order.push("start")),
      onPendingSettle: vi.fn(() => order.push("settle")),
      onCompleted: vi.fn(() => order.push("completed")),
    });
    await dispatchPaletteCommand({ command: asyncCommand, ports });
    expect(order).toEqual(["start", "invoke", "completed", "settle"]);
  });

  it("Should release the pending state when the invoke fails mid-flight", async () => {
    const { ports } = portsFixture({
      invoke: vi.fn(async () => {
        throw new Error("runtime unavailable");
      }),
    });
    const outcome = await dispatchPaletteCommand({ command: asyncCommand, ports });
    expect(outcome).toEqual({ status: "refused", reason: "runtime unavailable" });
    expect(ports.onPendingSettle).toHaveBeenCalledWith(asyncCommand);
    expect(ports.onCompleted).not.toHaveBeenCalled();
  });

  it("Should carry the daemon's error code so retry can be gated on it", async () => {
    const rejection = Object.assign(new Error("ext.notes.purge is already in flight"), {
      code: "already_running",
    });
    const { ports } = portsFixture({
      invoke: vi.fn(async () => {
        throw rejection;
      }),
    });
    const outcome = await dispatchPaletteCommand({ command: asyncCommand, ports });
    expect(outcome).toEqual({
      status: "refused",
      reason: "ext.notes.purge is already in flight",
      code: "already_running",
    });
    expect(ports.onFailure).toHaveBeenCalledWith(
      asyncCommand,
      "ext.notes.purge is already in flight",
      "already_running"
    );
  });

  it("Should never report a synchronous client operation as pending", async () => {
    const { ports } = portsFixture();
    await dispatchPaletteCommand({ command: command(), ports });
    expect(ports.onPendingStart).not.toHaveBeenCalled();
    expect(ports.onCompleted).not.toHaveBeenCalled();
  });
});

describe("cmd-palette feedback copy and retry gating (UT-159, UT-160)", () => {
  const retrySafe = command({ id: "app.open.tasks", title: "Open Tasks" });
  const oneShot = command({
    id: "ext.notes.purge",
    title: "Purge archived notes",
    execution: { retry_safe: false, single_flight: true },
  });

  it("Should name the command on success", () => {
    expect(invokeCompletedFeedback(retrySafe, { status: "ok" })).toEqual({
      message: "Open Tasks finished",
      tone: "success",
      retryable: false,
    });
  });

  it("Should say an approval is pending rather than claiming the command ran", () => {
    const feedback = invokeCompletedFeedback(oneShot, {
      status: "approval_pending",
      approval_id: "apr_55e0c9",
    });
    expect(feedback).toEqual({
      message: "Purge archived notes needs approval before it runs",
      tone: "info",
      retryable: false,
    });
  });

  it("Should name the command and repeat the runtime reason verbatim on failure", () => {
    expect(invokeFailedFeedback(oneShot, "runtime unavailable")).toEqual({
      message: "Purge archived notes — runtime unavailable",
      tone: "error",
      retryable: false,
    });
  });

  it("Should offer retry only where re-running is declared safe", () => {
    const safe = command({
      ...retrySafe,
      execution: { retry_safe: true, single_flight: false },
    });
    expect(invokeFailedFeedback(safe, "runtime unavailable").retryable).toBe(true);
    expect(invokeFailedFeedback(oneShot, "runtime unavailable").retryable).toBe(false);
  });

  it("Should never offer retry for a single-flight rejection", () => {
    const safe = command({
      ...retrySafe,
      execution: { retry_safe: true, single_flight: true },
    });
    expect(canRetry(safe, ALREADY_RUNNING_CODE)).toBe(false);
    expect(
      invokeFailedFeedback(safe, "Open Tasks is already in flight", ALREADY_RUNNING_CODE).retryable
    ).toBe(false);
  });

  it("Should name the workspace a landing switched to", () => {
    expect(workspaceSwitchFeedback("payments", "Fix payment retries")).toEqual({
      message: "Switched to payments to open Fix payment retries",
      tone: "info",
      retryable: false,
    });
  });
});

describe("cmd-palette client-op table — tabs", () => {
  it("Should jump positionally, wrap next/previous from the active tab, and land last [UT-050]", () => {
    const frame = frameFixture();
    const jump = opContext(frame, "w-2");
    void run("window.tab.jump.3", jump);
    expect(jump.shell.activateWindow).toHaveBeenCalledWith("w-3");

    const next = opContext(frame, "w-2");
    void run("window.tab.next", next);
    expect(next.shell.activateWindow).toHaveBeenCalledWith("w-3");

    const wrap = opContext(frameFixture({ activeWindowId: "w-3" }), "w-3");
    void run("window.tab.next", wrap);
    expect(wrap.shell.activateWindow).toHaveBeenCalledWith("w-1");

    const previous = opContext(frameFixture({ activeWindowId: "w-1" }), "w-1");
    void run("window.tab.previous", previous);
    expect(previous.shell.activateWindow).toHaveBeenCalledWith("w-3");

    const last = opContext(frame, "w-2");
    void run("window.tab.last", last);
    expect(last.shell.activateWindow).toHaveBeenCalledWith("w-3");
  });

  it("Should no-op jumps and cycling on a single-tab frame while ⌘T still opens the deck path [UT-052]", () => {
    const solo = frameFixture({
      id: "w-1",
      members: ["w-1"],
      activeWindowId: "w-1",
      stackId: null,
    });
    for (const op of [
      "window.tab.next",
      "window.tab.previous",
      "window.tab.last",
      "window.tab.jump.2",
    ]) {
      const context = opContext(solo, "w-1");
      void run(op, context);
      expect(context.shell.activateWindow).not.toHaveBeenCalled();
    }

    const newTab = opContext(solo, "w-1");
    void run("window.tab.new", newTab);
    expect(newTab.shell.openNewTab).toHaveBeenCalledWith("w-1");
  });

  it("Should open a standalone new tab when no window is focused [UT-082]", () => {
    const context = opContext(null, null);
    void run("window.tab.new", context);
    expect(context.shell.openNewTab).toHaveBeenCalledWith(null);
  });

  it("Should dispatch reopen without requiring a focused window [UT-052]", () => {
    const context = opContext(null, null);
    void run("window.tab.reopen", context);
    expect(context.manager.reopenWindow).toHaveBeenCalledOnce();
  });

  it("Should ignore a jump slot beyond the frame's members", () => {
    const context = opContext(frameFixture(), "w-2");
    void run("window.tab.jump.8", context);
    expect(context.shell.activateWindow).not.toHaveBeenCalled();
  });
});

describe("cmd-palette client-op table — navigation v2", () => {
  it("Should focus the previous MRU window and ignore minimized entries [UT-072]", () => {
    const context = opContext(null, "w-2");
    const state = context.manager.getState() as unknown as {
      client: { focusOrder: string[] };
      windows: Record<string, { minimized: boolean }>;
    };
    state.client.focusOrder = ["w-2", "w-3", "w-1"];
    state.windows["w-3"]!.minimized = true;

    void run("window.focus.last", context);

    expect(context.shell.activateWindow).toHaveBeenCalledExactlyOnceWith("w-1");
  });

  it("Should share stable desktop order with numeric switching and no-op missing slots [UT-073]", () => {
    const context = opContext(null, null);
    const state = context.manager.getState() as unknown as {
      desktops: { id: string; order: number }[];
    };
    state.desktops = [
      { id: "desktop:c", order: 1 },
      { id: "desktop:b", order: 0 },
      { id: "desktop:a", order: 0 },
    ];

    void run("desktop.switch.2", context);
    expect(context.manager.switchDesktop).toHaveBeenCalledExactlyOnceWith("desktop:b");

    void run("desktop.switch.9", context);
    expect(context.manager.switchDesktop).toHaveBeenCalledOnce();
  });

  it("Should route session and attention actions through their shared shell owners [UT-074]", () => {
    const context = opContext(null, null);
    void run("session.cycle.next", context);
    void run("session.focus.attention", context);

    expect(context.shell.cycleSession).toHaveBeenCalledExactlyOnceWith("next");
    expect(context.shell.focusAttention).toHaveBeenCalledOnce();
  });
});

describe("cmd-palette client-op table — absorbed palette rows", () => {
  it("Should merge every visible peer into the focused window's frame", () => {
    const context = opContext(frameFixture(), "w-2");
    void run("window.merge_all", context);
    expect(context.manager.groupWindows).toHaveBeenCalledWith("w-2", ["w-1", "w-3"]);
  });

  it("Should float the focused tab out of its stack and follow it", async () => {
    const context = opContext(frameFixture(), "w-2");
    void run("window.tab.detach", context);
    expect(context.manager.toggleFloating).toHaveBeenCalledWith("w-2");
    await Promise.resolve();
    await Promise.resolve();
    expect(context.shell.activateWindow).toHaveBeenCalledWith("w-2");
  });

  it("Should leave a lone window in place rather than detaching nothing", () => {
    const solo = frameFixture({ members: ["w-2"], activeWindowId: "w-2", stackId: null });
    const context = opContext(solo, "w-2");
    void run("window.tab.detach", context);
    expect(context.manager.toggleFloating).not.toHaveBeenCalled();
  });

  it("Should raise the sessions catalog through its shell owner", () => {
    const context = opContext(null, null);
    void run("shell.sessions.toggle", context);
    expect(context.shell.toggleSessions).toHaveBeenCalledOnce();
  });
});
