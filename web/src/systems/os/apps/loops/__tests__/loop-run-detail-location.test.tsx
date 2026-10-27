// Suite: Loop run detail route composition
// Invariant: a run loads through its window's data scope — the serialized binding wins, a User
// run loads through `@user`, and an unbound All projects window reads nothing — and the run label
// names the same project that loaded it.
// Owning layer: LoopRunDetailLocation, which binds the window's data scope to the run page.
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useTopbarSlot } from "@compozy/ui";

const workspace = vi.hoisted(() => ({
  activeWorkspace: { id: "ws_project", name: "Project workspace" },
  dataScope: { kind: "project", workspaceId: "ws_home" } as
    | { kind: "project"; workspaceId: string }
    | { kind: "user" }
    | { kind: "all" },
  registeredWorkspaces: [
    { id: "ws_home", name: "Home workspace" },
    { id: "ws_project", name: "Project workspace" },
    { id: "ws_target", name: "Target workspace" },
  ],
}));
const loopRunPageBodySpy = vi.hoisted(() => vi.fn((_props: Record<string, unknown>) => null));
const quarantineSheetSpy = vi.hoisted(() => vi.fn((_props: Record<string, unknown>) => null));
const loopRunPageSpy = vi.hoisted(() => vi.fn());
const runQueryState = vi.hoisted(() => ({ pending: false }));
const pageRun = vi.hoisted(() => ({
  current: null as {
    historical?: boolean;
    loop_name: string;
    pause_requested?: boolean;
    status: string;
  } | null,
}));

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => vi.fn(),
}));

vi.mock("@compozy/ui", () => ({
  cn: (...classes: unknown[]) => classes.filter(Boolean).join(" "),
  Empty: () => null,
  formatRelativeTime: () => "5m ago",
  PAGE_CONTENT_GUTTER: "",
  Pill: ({ children }: { children: unknown }) => children,
  Skeleton: () => null,
  SkeletonRows: () => null,
  useTopbarSlot: vi.fn(),
}));

vi.mock("@/systems/workspace", async () => ({
  ...(await import("@/systems/workspace/lib/destination")),
  ...(await import("@/systems/workspace/lib/workspace-scope-copy")),
  useActiveWorkspace: () => workspace,
  // The window's data scope; the binding is folded into it by the hook under test elsewhere.
  useWindowDataWorkspace: () => workspace.dataScope,
  useWindowBindingSearch: () => ({}),
}));

vi.mock("../../../hooks/use-window-live-data-enabled", () => ({
  useCurrentWindowLiveDataEnabled: () => true,
}));

vi.mock("../use-loop-run-page", () => ({
  useLoopRunPage: (...args: unknown[]) => {
    loopRunPageSpy(...args);
    return {
      effectiveRun: {
        generation: 1,
        historical: pageRun.current?.historical ?? false,
        status: pageRun.current?.status ?? "running",
        workspace_id: "ws_home",
      },
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
      runQuery: { error: null, isLoading: false, isPending: runQueryState.pending },
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

vi.mock("@/systems/loops", async () => ({
  // The real status vocabulary: the route decides "ended" with it, and a stub
  // that answered from a hand-written list would only test the list.
  ...(await vi.importActual<typeof import("@/systems/loops/lib/loop-formatters")>(
    "@/systems/loops/lib/loop-formatters"
  )),
  LoopForkDialog: () => null,
  LoopNodeAmendDialog: () => null,
  LoopNodeControlDialog: () => null,
  LoopNodeRerunDialog: () => null,
  LoopNodeRowActions: () => null,
  LoopQuarantineSheet: (props: Record<string, unknown>) => quarantineSheetSpy(props),
  LoopRunControlDialog: () => null,
  LoopRunControls: () => null,
  LoopRunOverflowMenu: () => null,
  LoopRunPageBody: (props: Record<string, unknown>) => loopRunPageBodySpy(props),
  LoopRunPageSkeleton: () => <div data-testid="loop-run-page-skeleton" />,
  LoopStatusMark: () => null,
  /** Keeps Goal history idle while these cases exercise route composition. */
  useGoalTurns: () => ({
    turns: [],
    hasMore: false,
    isLoading: false,
    isError: false,
    isLoadingMore: false,
  }),
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
    workspace.dataScope = { kind: "project", workspaceId: "ws_home" };
    loopRunPageBodySpy.mockClear();
    quarantineSheetSpy.mockClear();
    loopRunPageSpy.mockClear();
    pageRun.current = null;
    runQueryState.pending = false;
    vi.mocked(useTopbarSlot).mockClear();
  });

  // Invariant: a run that has no data yet and no failure is waiting, not missing. "No data yet" is
  // the query's `isPending` (a window outside the live-data budget has not started its read), so
  // the page's skeleton shows for it; a refresh behind a run already read shows the page.
  it("Should hold the page's skeleton, not a missing run, while the run has not been read", () => {
    runQueryState.pending = true;

    render(<LoopRunDetailLocation runId="run-1" />);

    expect(screen.getByTestId("loop-run-page-skeleton")).toBeInTheDocument();
    expect(loopRunPageBodySpy).not.toHaveBeenCalled();
  });

  it("Should show the page, not the skeleton, once the run has been read", () => {
    render(<LoopRunDetailLocation runId="run-1" />);

    expect(screen.queryByTestId("loop-run-page-skeleton")).not.toBeInTheDocument();
    expect(loopRunPageBodySpy).toHaveBeenCalled();
  });

  it("Should show the registered runtime workspace name when the selected project differs", () => {
    render(<LoopRunDetailLocation runId="run-1" />);

    expect(loopRunPageBodySpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ workspaceLabel: "Home workspace" })
    );
  });

  it("Should load a User run through @user", () => {
    workspace.dataScope = { kind: "user" };

    render(<LoopRunDetailLocation runId="run-1" />);

    expect(loopRunPageSpy).toHaveBeenCalledWith("@user", "run-1", expect.anything());
  });

  it("Should show the project name when it owns the runtime binding", () => {
    workspace.dataScope = { kind: "project", workspaceId: "ws_project" };

    render(<LoopRunDetailLocation runId="run-1" />);

    expect(loopRunPageBodySpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ workspaceLabel: "Project workspace" })
    );
  });

  it("Should load a bound run from its binding's project on the All projects desktop", () => {
    workspace.dataScope = { kind: "project", workspaceId: "ws_target" };
    const requestFocus = { nodeId: "publish", itemIndex: 2 };
    render(<LoopRunDetailLocation requestFocus={requestFocus} runId="run-1" />);

    expect(loopRunPageSpy).toHaveBeenCalledWith("ws_target", "run-1", {
      liveDataEnabled: true,
    });
    expect(loopRunPageBodySpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ requestFocus, workspaceLabel: "Target workspace" })
    );
  });

  it("Should read nothing in an unbound All projects window", () => {
    workspace.dataScope = { kind: "all" };

    render(<LoopRunDetailLocation runId="run-1" />);

    expect(loopRunPageSpy).not.toHaveBeenCalled();
  });

  it("Should keep Runs in the drill-in trail after the loop name loads", () => {
    pageRun.current = { loop_name: "implement-tasks", status: "running" };

    render(<LoopRunDetailLocation runId="run-1" />);

    const slot = vi.mocked(useTopbarSlot).mock.calls.at(-1)?.[0];
    // The run leaf names the run by when it started; the id stays in About.
    expect(slot).toEqual(
      expect.objectContaining({
        crumb: "Run from 5m ago",
        crumbs: [
          expect.objectContaining({ id: "loops", label: "Loops" }),
          expect.objectContaining({ id: "runs", label: "Runs" }),
          expect.objectContaining({ id: "loop", label: "implement-tasks" }),
        ],
      })
    );
  });

  it("Should make historical runs read-only even when their stored status is live", () => {
    pageRun.current = {
      historical: true,
      loop_name: "implement-tasks",
      status: "running",
    };

    render(<LoopRunDetailLocation runId="run-history" />);

    const slot = vi.mocked(useTopbarSlot).mock.calls.at(-1)?.[0];
    expect(slot?.actions).toBeUndefined();
    expect(loopRunPageBodySpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ renderNodeActions: undefined })
    );
  });

  // The quarantine sheet offers requeue only while the daemon would accept it.
  // The route owns that scoping, from the run's own status: live stays live.
  it("Should keep the quarantine sheet live while the run is still running", () => {
    pageRun.current = { loop_name: "implement-tasks", status: "running" };

    render(<LoopRunDetailLocation runId="run-1" />);

    expect(quarantineSheetSpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ runEnded: false })
    );
  });

  it("Should mark the quarantine sheet ended once the run reaches a terminal status", () => {
    pageRun.current = { loop_name: "implement-tasks", status: "failed" };

    render(<LoopRunDetailLocation runId="run-1" />);

    expect(quarantineSheetSpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ runEnded: true })
    );
  });
});
