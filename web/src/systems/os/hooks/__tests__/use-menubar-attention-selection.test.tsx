// Suite: menubar bell row selection (task and Loop kinds of openTarget)
// Invariant: under All projects a bell task or Loop row opens in place in the `global` desktop,
// bound to its own scope (a project, or `@user` for an item without one), and the destination
// never changes. With a project focused, a foreign item focuses its destination first and opens
// only once that partition's window manager is live.
// Owning layer: OS navigation hook. No prior suite owned bell row selection.
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { OsAttentionRow } from "../../lib/attention-model";

const { focusDestination, setActiveWorkspaceId, notifyUser } = vi.hoisted(() => ({
  focusDestination: vi.fn(),
  setActiveWorkspaceId: vi.fn(),
  notifyUser: vi.fn(),
}));
type TestDataScope = { kind: "project"; workspaceId: string } | { kind: "user" } | { kind: "all" };
let dataScope: TestDataScope = { kind: "all" };
let commandWorkspaceId: string | null = "global";
const userOpen = vi.fn(() => Promise.resolve("window-tasks"));

vi.mock("@/lib/user-feedback", () => ({ notifyUser }));

// Only the Loop request route builder; the terminal barrel is not exercised by these rows.
vi.mock("@/systems/loops", async () => await import("@/systems/loops/lib/loop-request-location"));
vi.mock("@/systems/terminal", () => ({ terminalAttentionLocation: vi.fn() }));

vi.mock("@/systems/workspace", async () => {
  // The pure destination helpers, without the barrel's hook graph.
  const destination = await import("@/systems/workspace/lib/destination");
  const binding = await import("@/systems/workspace/lib/window-data-scope");
  const partition = () =>
    dataScope.kind === "all"
      ? "global"
      : dataScope.kind === "user"
        ? "user"
        : dataScope.workspaceId;
  return {
    ...destination,
    ...binding,
    focusDestination,
    useActiveWorkspace: () => ({
      dataScope,
      desktopWorkspaceId: partition(),
      runtimeWorkspaceId: dataScope.kind === "project" ? dataScope.workspaceId : null,
      setActiveWorkspaceId,
    }),
  };
});

vi.mock("../use-desktop", () => ({
  useDesktop: (selector: (state: unknown) => unknown) =>
    selector({
      snapshot: commandWorkspaceId === null ? null : { workspaceId: commandWorkspaceId },
      client: commandWorkspaceId === null ? null : {},
      hydration: commandWorkspaceId === null ? "pending" : "live",
      connectionStatus: commandWorkspaceId === null ? "disconnected" : "connected",
    }),
}));

vi.mock("../use-os-shell", () => ({
  useOsShell: () => ({ coordinator: { userOpen, userOpenCreatedSession: userOpen } }),
}));

vi.mock("../use-attention-jump", () => ({ useAttentionJump: () => vi.fn() }));

import { useMenubarAttentionSelection } from "../use-menubar-attention-selection";

function taskRow(workspaceId: string): OsAttentionRow {
  return { kind: "task", id: "task-42", title: "Approve the release", workspaceId };
}

describe("useMenubarAttentionSelection", () => {
  beforeEach(() => {
    dataScope = { kind: "all" };
    commandWorkspaceId = "global";
    vi.clearAllMocks();
  });

  it("Should open another project's task in place under All projects", () => {
    const onOverlayOpenChange = vi.fn();
    const { result } = renderHook(() => useMenubarAttentionSelection(onOverlayOpenChange));

    act(() => result.current(taskRow("ws-beta")));

    expect(onOverlayOpenChange).toHaveBeenCalledWith("bell", false);
    expect(userOpen).toHaveBeenCalledExactlyOnceWith({
      app: "tasks",
      route: { pathname: "/tasks/task-42", search: { workspace: "ws-beta" } },
    });
    expect(focusDestination).not.toHaveBeenCalled();
    expect(setActiveWorkspaceId).not.toHaveBeenCalled();
  });

  it("Should bind a task without a project to User", () => {
    const { result } = renderHook(() => useMenubarAttentionSelection(vi.fn()));

    act(() => result.current(taskRow("")));

    expect(userOpen).toHaveBeenCalledExactlyOnceWith({
      app: "tasks",
      route: { pathname: "/tasks/task-42", search: { workspace: "@user" } },
    });
  });

  it("Should focus a foreign task's project first in project mode", () => {
    dataScope = { kind: "project", workspaceId: "ws-alpha" };
    commandWorkspaceId = "ws-alpha";
    const { result, rerender } = renderHook(() => useMenubarAttentionSelection(vi.fn()));

    act(() => result.current(taskRow("ws-beta")));

    expect(focusDestination).toHaveBeenCalledExactlyOnceWith({
      kind: "project",
      workspaceId: "ws-beta",
    });
    expect(userOpen).not.toHaveBeenCalled();

    dataScope = { kind: "project", workspaceId: "ws-beta" };
    commandWorkspaceId = "ws-beta";
    rerender();

    expect(userOpen).toHaveBeenCalledExactlyOnceWith({
      app: "tasks",
      route: { pathname: "/tasks/task-42", search: {} },
    });
  });
});

describe("useMenubarAttentionSelection (Loop rows)", () => {
  beforeEach(() => {
    dataScope = { kind: "all" };
    commandWorkspaceId = "global";
    vi.clearAllMocks();
  });

  const requestRow = (workspaceId: string): OsAttentionRow => ({
    kind: "loop-request",
    id: `${workspaceId}:run-1:review:0`,
    title: "review",
    workspaceId,
    workspaceLabel: "beta",
    runId: "run-1",
    loopName: "triage",
    nodeId: "review",
    itemIndex: 0,
    requestKind: "ask",
    openedAt: "2026-10-21T10:00:00Z",
    stale: false,
  });

  it("Should open another project's Loop request in place under All projects", () => {
    const { result } = renderHook(() => useMenubarAttentionSelection(vi.fn()));

    act(() => result.current(requestRow("ws-beta")));

    expect(userOpen).toHaveBeenCalledExactlyOnceWith({
      app: "loops",
      route: {
        pathname: "/loop-runs/run-1",
        search: { workspace: "ws-beta", request_node: "review", request_item: 0 },
      },
    });
    expect(focusDestination).not.toHaveBeenCalled();
    expect(setActiveWorkspaceId).not.toHaveBeenCalled();
  });

  it("Should bind a User Loop node row to @user", () => {
    const { result } = renderHook(() => useMenubarAttentionSelection(vi.fn()));

    act(() =>
      result.current({
        kind: "loop-node",
        id: "node-1",
        title: "build",
        workspaceId: "",
        runId: "run-u",
        state: "waiting",
      })
    );

    expect(userOpen).toHaveBeenCalledExactlyOnceWith({
      app: "loops",
      route: { pathname: "/loop-runs/run-u", search: { workspace: "@user" } },
    });
  });

  it("Should focus a foreign Loop request's project first in project mode", () => {
    dataScope = { kind: "project", workspaceId: "ws-alpha" };
    commandWorkspaceId = "ws-alpha";
    const { result } = renderHook(() => useMenubarAttentionSelection(vi.fn()));

    act(() => result.current(requestRow("ws-beta")));

    expect(focusDestination).toHaveBeenCalledExactlyOnceWith({
      kind: "project",
      workspaceId: "ws-beta",
    });
    expect(userOpen).not.toHaveBeenCalled();
  });
});
