// Suite: Loop run detail route composition
// Invariant: a run label identifies the same runtime workspace that loaded the run.
// Owning layer: LoopRunDetailLocation, which binds shell workspace state to the run page.
import { render } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const workspace = vi.hoisted(() => ({
  activeWorkspace: { id: "ws_project", name: "Project workspace" },
  runtimeWorkspaceId: "ws_home",
  workspaces: [
    { id: "ws_home", name: "Home workspace" },
    { id: "ws_project", name: "Project workspace" },
    { id: "ws_target", name: "Target workspace" },
  ],
}));
const loopRunPageBodySpy = vi.hoisted(() => vi.fn((_props: Record<string, unknown>) => null));
const loopRunPageSpy = vi.hoisted(() => vi.fn());

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => vi.fn(),
}));

vi.mock("@compozy/ui", () => ({
  Empty: () => null,
  Spinner: () => null,
  useTopbarSlot: vi.fn(),
}));

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => workspace,
}));

vi.mock("../../../hooks/use-window-live-data-enabled", () => ({
  useCurrentWindowLiveDataEnabled: () => true,
}));

vi.mock("../use-loop-run-page", () => ({
  useLoopRunPage: (...args: unknown[]) => {
    loopRunPageSpy(...args);
    return {
      effectiveRun: { generation: 1, status: "running", workspace_id: "ws_home" },
      goalTurnsQuery: {
        fetchNextPage: vi.fn(),
        hasNextPage: false,
        isFetchingNextPage: false,
      },
      live: { failure: null, frames: [], needsApproval: null },
      materializedContract: {},
      elapsedLabel: "",
      nodeLifecycles: [],
      nodesById: new Map(),
      progress: {},
      waitingNodes: [],
      resetRunControlErrors: vi.fn(),
      run: null,
      runQuery: { error: null, isLoading: false },
    };
  },
}));

vi.mock("../use-loop-node-controls", () => ({
  useLoopNodeControls: () => ({
    closeDialog: vi.fn(),
    closeQuarantine: vi.fn(),
    commit: vi.fn(),
    onVerb: vi.fn(),
    openQuarantine: vi.fn(),
    quarantineNodeId: null,
  }),
}));

vi.mock("@/systems/loops", () => ({
  LoopNodeControlDialog: () => null,
  LoopNodeRowActions: () => null,
  LoopQuarantineSheet: () => null,
  LoopRunControlDialog: () => null,
  LoopRunControls: () => null,
  LoopRunOverflowMenu: () => null,
  LoopRunPageBody: (props: Record<string, unknown>) => loopRunPageBodySpy(props),
  LoopStatusPill: () => null,
}));

const { LoopRunDetailLocation } = await import("../loop-run-detail-location");

describe("LoopRunDetailLocation", () => {
  beforeEach(() => {
    workspace.activeWorkspace = { id: "ws_project", name: "Project workspace" };
    workspace.runtimeWorkspaceId = "ws_home";
    loopRunPageBodySpy.mockClear();
    loopRunPageSpy.mockClear();
  });

  it("Should show the registered runtime workspace name when the selected project differs", () => {
    render(<LoopRunDetailLocation runId="run-1" />);

    expect(loopRunPageBodySpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ workspaceLabel: "Home workspace" })
    );
  });

  it("Should show the project name when it owns the runtime binding", () => {
    workspace.runtimeWorkspaceId = "ws_project";

    render(<LoopRunDetailLocation runId="run-1" />);

    expect(loopRunPageBodySpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ workspaceLabel: "Project workspace" })
    );
  });

  it("Should bind a trigger deep link to its explicit target workspace", () => {
    render(<LoopRunDetailLocation routeWorkspaceId="ws_target" runId="run-1" />);

    expect(loopRunPageSpy).toHaveBeenCalledWith("ws_target", "run-1", {
      liveDataEnabled: true,
    });
    expect(loopRunPageBodySpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ workspaceLabel: "Target workspace" })
    );
  });
});
