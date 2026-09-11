// Suite: task detail page body composition
// Invariant: the page body offers exactly the affordances the latched tier's
// route matrix registers — task/run lifecycle mutations (including clear-block
// and the task PATCH behind the rail's priority/auto-enqueue editors) register
// only on the local surface set, so the Now-strip lifecycle actions, the
// properties-rail approval buttons and inline editors, and Start run are
// absent (never disabled) on remote tiers (BR-1), while the read states
// (attention bands, approval rows, editor read rows) stay on every tier; the
// local latch keeps the full shape (BR-5).
// Boundary IN: the detail location's handler wiring and affordance presence.
// Boundary OUT: the head chrome (task-detail-topbar suite), the presentational
// components (task-overview-panel / task-properties-rail / task-runs-panel
// suites), and the page hook verbs (use-task-detail-page suite).
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactElement, ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { latchGatewayTierForTest } from "@/test/gateway-tier";
import { renderWithTopbar } from "@/test/render-with-topbar";

vi.mock("@tanstack/react-router", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({ to, children, ...props }: Record<string, unknown>) => (
      <a href={typeof to === "string" ? to : "#"} {...(props as Record<string, unknown>)}>
        {children as ReactNode}
      </a>
    ),
  };
});

vi.mock("../task-detail-overlays", () => ({ TaskDetailOverlays: () => null }));
vi.mock("../task-detail-topbar", () => ({ TaskDetailTopbar: () => null }));

const mocks = vi.hoisted(() => ({
  controller: null as unknown,
}));

vi.mock("../use-task-detail-location", () => ({
  useTaskDetailLocation: () => mocks.controller,
}));

import { buildDetailFixture } from "@/systems/tasks/mocks/fixtures";
import type { TaskCommandState } from "@/systems/tasks";
import { TaskDetailLocation } from "../task-detail-location";
import type { TaskDetailLocationController } from "../use-task-detail-location";

/** A start-able draft: the Runs tab can offer Start run on the local tier. */
const START_COMMAND: TaskCommandState = {
  canFanOut: false,
  overflow: {
    cancel: false,
    delete: false,
    edit: false,
    pause: false,
    resume: false,
    startNewRun: false,
  },
  primary: { kind: "start" },
  secondary: { edit: false, pause: false, reject: false },
};

function buildController({ tab = "overview" }: { tab?: "overview" | "runs" } = {}) {
  const detail = buildDetailFixture({
    runs: [],
    summary: { active_run: null, status: "needs_attention" },
    task: {
      approval_state: "pending",
      blocked_reasons: [
        { reason: "Paused by operator", source: "paused" },
        { block_id: "block_001", reason: "Waiting on creator input", source: "block" },
      ],
      status: "needs_attention",
    },
  } as never);
  return {
    activeElapsed: "",
    backToTasks: vi.fn(),
    command: START_COMMAND,
    copyTaskId: vi.fn(),
    detail,
    openRun: vi.fn(),
    openTask: vi.fn(),
    page: {
      detailError: null,
      detailLoading: false,
      fatalError: null,
      handleApproveTask: vi.fn(),
      handleClearBlock: vi.fn(),
      handleEnqueueRun: vi.fn(),
      handleRejectTask: vi.fn(),
      handleRecoverTask: vi.fn(),
      handleResumeTask: vi.fn(),
      isApprovePending: false,
      isClearBlockPending: false,
      isEnqueuePending: false,
      isLive: false,
      isRejectPending: false,
      isRecoverPending: false,
      isResumePending: false,
      isTimelineSaturated: false,
      notFound: false,
      profile: null,
      reviews: [],
      runs: [],
      runsError: null,
      runsLoading: false,
      timeline: [],
      timelineError: null,
      timelineLoading: false,
    },
    record: detail.task,
    runDurations: new Map<string, string>(),
    scrollToResult: vi.fn(),
    search: { tab },
    setInspectOpen: vi.fn(),
    setSetupOpen: vi.fn(),
    setTab: vi.fn(),
    updatePending: false,
  } as unknown as TaskDetailLocationController;
}

const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });

function detailUi(tab: "overview" | "runs"): ReactElement {
  return (
    <QueryClientProvider client={queryClient}>
      <TaskDetailLocation search={{ tab }} taskId="task_001" />
    </QueryClientProvider>
  );
}

function renderDetail(controller: TaskDetailLocationController) {
  mocks.controller = controller;
  return renderWithTopbar(detailUi("overview"));
}

/** Switches to the Runs tab by swapping the controller fixture and rerendering. */
function rerenderRunsTab(view: { rerender: (ui: ReactNode) => void }) {
  mocks.controller = buildController({ tab: "runs" });
  view.rerender(detailUi("runs"));
}

describe("TaskDetailLocation", () => {
  let unlatch: () => void;
  beforeEach(() => {
    // Task/run lifecycle affordances render on the local tier (BR-5).
    unlatch = latchGatewayTierForTest("local");
  });
  afterEach(() => unlatch());

  it("Should keep the lifecycle affordances and the read states on the local tier", () => {
    renderDetail(buildController());

    expect(screen.getByTestId("tasks-detail-now-approval")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-detail-now-approve")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-detail-now-reject")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-detail-now-resume")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-detail-now-recover")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-detail-now-clear-block-block_001")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-rail-approve")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-rail-reject")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-rail-priority")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-rail-auto-enqueue")).toBeInTheDocument();
  });

  it("Should wire the overview approval buttons to the page verbs", async () => {
    const user = userEvent.setup();
    const controller = buildController();
    renderDetail(controller);

    await user.click(screen.getByTestId("tasks-detail-now-approve"));

    expect(controller.page.handleApproveTask).toHaveBeenCalledTimes(1);
  });

  it("Should offer Start run on the local tier when the command state allows it", () => {
    const view = renderDetail(buildController());
    rerenderRunsTab(view);

    expect(screen.getByTestId("tasks-runs-start")).toBeInTheDocument();
  });

  it("Should keep the read states while dropping every lifecycle affordance on a remote tier", () => {
    unlatch();
    unlatch = latchGatewayTierForTest("private");
    const view = renderDetail(buildController());

    // Read states stay on every tier; only the mutation affordances go absent.
    expect(screen.getByTestId("tasks-detail-now-approval")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-detail-now-paused")).toBeInTheDocument();
    // The block band itself is a read state; its Clear-block button is not.
    expect(screen.getByTestId("tasks-detail-now-block-block_001")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-detail-now-stuck")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-detail-rail")).toBeInTheDocument();

    expect(screen.queryByTestId("tasks-detail-now-approve")).not.toBeInTheDocument();
    expect(screen.queryByTestId("tasks-detail-now-reject")).not.toBeInTheDocument();
    expect(screen.queryByTestId("tasks-detail-now-resume")).not.toBeInTheDocument();
    expect(screen.queryByTestId("tasks-detail-now-recover")).not.toBeInTheDocument();
    expect(screen.queryByTestId("tasks-detail-now-clear-block-block_001")).not.toBeInTheDocument();
    expect(screen.queryByTestId("tasks-rail-approve")).not.toBeInTheDocument();
    expect(screen.queryByTestId("tasks-rail-reject")).not.toBeInTheDocument();

    // The PATCH-backed rail editors go absent while their read rows keep the
    // current values (BR-1 — absent, never disabled).
    const rail = within(screen.getByTestId("tasks-detail-rail"));
    expect(screen.queryByTestId("tasks-rail-priority")).not.toBeInTheDocument();
    expect(screen.queryByTestId("tasks-rail-auto-enqueue")).not.toBeInTheDocument();
    expect(rail.getByText("Priority")).toBeInTheDocument();
    expect(rail.getByText("High")).toBeInTheDocument();
    expect(rail.getByText("Auto-enqueue")).toBeInTheDocument();
    expect(rail.getByText("Off")).toBeInTheDocument();

    rerenderRunsTab(view);

    expect(screen.queryByTestId("tasks-runs-start")).not.toBeInTheDocument();
  });
});
