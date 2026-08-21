// Suite: Loop run detail route composition
// Invariant: a run label identifies the same runtime workspace that loaded the run.
// Owning layer: LoopRunDetailLocation, which binds shell workspace state to the run page.
import { render } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useTopbarSlot } from "@compozy/ui";

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
const pageRun = vi.hoisted(() => ({
  current: null as { loop_name: string; pause_requested?: boolean; status: string } | null,
}));

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
      run: pageRun.current,
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

vi.mock("../use-loop-run-requests-state", () => ({
  useLoopRunRequestsState: () => ({
    engagedKey: undefined,
    isAnswerPending: false,
    fieldErrors: undefined,
    refusal: undefined,
    fullContext: undefined,
    isLoadingFullContext: false,
    onRequestFullContext: vi.fn(),
    onAnswer: vi.fn(),
  }),
}));

vi.mock("../use-loop-run-timetravel", () => ({
  useLoopRunTimetravel: () => ({
    forkGeneration: null,
    forkGenerations: [],
    forkGenerationsSourceInputs: undefined,
    forkgFieldErrors: undefined,
    onCloseFork: vi.fn(),
    onCompareGeneration: vi.fn(),
    onForkGeneration: vi.fn(),
    onOpenRun: vi.fn(),
    onSubmitFork: vi.fn(),
  }),
}));

vi.mock("@/systems/loops", () => ({
  LoopForkDialog: () => null,
  LoopNodeAmendDialog: () => null,
  LoopNodeControlDialog: () => null,
  LoopNodeRerunDialog: () => null,
  LoopNodeRowActions: () => null,
  LoopQuarantineSheet: () => null,
  LoopRunControlDialog: () => null,
  LoopRunControls: () => null,
  LoopRunOverflowMenu: () => null,
  LoopRunPageBody: (props: Record<string, unknown>) => loopRunPageBodySpy(props),
  LoopStatusPill: () => null,
  // The Events lane's `view=all` read. Composed in `useLoopRunDetail` because the
  // disclosure that gates it is owned there; stubbed idle so these route cases
  // keep asserting what they own — workspace naming and the drill-in trail.
  useLoopRunEventsRead: () => ({
    beats: [],
    hasOlder: false,
    isLoading: false,
    isError: false,
    isLoadingOlder: false,
    onLoadOlder: () => undefined,
  }),
}));

const { LoopRunDetailLocation } = await import("../loop-run-detail-location");

describe("LoopRunDetailLocation", () => {
  beforeEach(() => {
    workspace.activeWorkspace = { id: "ws_project", name: "Project workspace" };
    workspace.runtimeWorkspaceId = "ws_home";
    loopRunPageBodySpy.mockClear();
    loopRunPageSpy.mockClear();
    pageRun.current = null;
    vi.mocked(useTopbarSlot).mockClear();
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
    const requestFocus = { nodeId: "publish", itemIndex: 2 };
    render(
      <LoopRunDetailLocation
        requestFocus={requestFocus}
        routeWorkspaceId="ws_target"
        runId="run-1"
      />
    );

    expect(loopRunPageSpy).toHaveBeenCalledWith("ws_target", "run-1", {
      liveDataEnabled: true,
    });
    expect(loopRunPageBodySpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ requestFocus, workspaceLabel: "Target workspace" })
    );
  });

  it("Should keep Runs in the drill-in trail after the loop name loads", () => {
    pageRun.current = { loop_name: "implement-tasks", status: "running" };

    render(<LoopRunDetailLocation runId="run-1" />);

    const slot = vi.mocked(useTopbarSlot).mock.calls.at(-1)?.[0];
    expect(slot).toEqual(
      expect.objectContaining({
        crumb: "run-1",
        crumbs: [
          expect.objectContaining({ id: "loops", label: "Loops" }),
          expect.objectContaining({ id: "runs", label: "Runs" }),
          expect.objectContaining({ id: "loop", label: "implement-tasks" }),
        ],
      })
    );
  });
});
