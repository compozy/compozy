import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    useNavigate: () => vi.fn(),
    Link: ({ to, params, children, ...props }: Record<string, unknown>) => (
      <a
        href={typeof to === "string" ? to : "#"}
        data-params={JSON.stringify(params)}
        {...(props as Record<string, unknown>)}
      >
        {children as React.ReactNode}
      </a>
    ),
  };
});

const { LoopRunProgressPanel } = await import("../run-page/loop-run-progress-panel");
const { LoopRunStoryTimeline } = await import("../run-page/loop-run-story-timeline");
const { LoopRunNowCard } = await import("../run-page/loop-run-now-card");
const { LoopRunNeedsYouCard } = await import("../run-page/loop-run-needs-you-card");
const { LoopRunOutcomeCard } = await import("../run-page/loop-run-outcome-card");
const { LoopRunControls } = await import("../run-page/loop-run-controls");
const { LoopRunUsageRail } = await import("../run-page/loop-run-usage-rail");
const { LoopRunAboutRail } = await import("../run-page/loop-run-about-rail");
const { LoopRunSubhead } = await import("../run-page/loop-run-subhead");
const { buildRunProgress } = await import("../../lib/loop-run-progress");
const { buildRunUsage } = await import("../../lib/loop-run-usage");
const { loopRunDetailByRunId } = await import("../../mocks/fixtures");
type LoopRunRecord = import("../../types").LoopRunRecord;
type LoopStoryRow = import("../../lib/loop-run-story").LoopStoryRow;
type LoopStoryNow = import("../../lib/loop-run-story").LoopStoryNow;

const detail = loopRunDetailByRunId.get("looprun_running")!;

function run(overrides: Partial<LoopRunRecord> = {}): LoopRunRecord {
  return { ...detail.run, ...overrides };
}

function storyRow(overrides: Partial<LoopStoryRow> = {}): LoopStoryRow {
  return {
    key: overrides.key ?? "f-1",
    kind: "gate_verdict",
    seq: 1,
    at: "2026-07-22T14:36:00Z",
    tone: "warning",
    icon: "check-warn",
    title: "Check: not clean yet",
    sub: "Verdict revise · confidence 0.91.",
    issues: [{ id: "issue_022", note: "no decision" }],
    micro: "gate_verdict · revise",
    ...overrides,
  };
}

const EMPTY_GOAL_IDS: ReadonlySet<string> = new Set();

describe("LoopRunProgressPanel", () => {
  it("Should render the goal voice, the group bar states, and the meta line", () => {
    const progress = buildRunProgress(
      run({ status: "running", generation: 2, reattempt_strategy: "failed_only" }),
      [
        {
          generation: 2,
          outputs: [
            { node_id: "fix", status: "succeeded", generation: 2, item_index: 1 },
            { node_id: "fix", status: "succeeded", generation: 2, item_index: 2 },
            { node_id: "fix", status: "running", generation: 2, item_index: 3 },
          ],
        },
      ],
      2
    );
    render(
      <LoopRunProgressPanel
        title="Resolve every review comment on PR 128"
        doneWhen="Done when a fresh review reports zero unresolved comments."
        progress={progress}
      />
    );
    expect(screen.getByTestId("loop-run-goal")).toHaveTextContent(
      "Resolve every review comment on PR 128"
    );
    expect(screen.getByTestId("loop-run-done-when")).toBeInTheDocument();
    const bar = screen.getByTestId("loop-run-progress-bar");
    expect(bar.querySelectorAll("[data-state='clean']")).toHaveLength(2);
    expect(bar.querySelectorAll("[data-state='active']")).toHaveLength(1);
    expect(screen.getByTestId("loop-run-progress-meta")).toHaveTextContent(
      "2 of 3 groups clean — the failed group is being redone"
    );
    expect(screen.getByTestId("loop-run-progress-right")).toHaveTextContent(
      "2 open points from the last check"
    );
  });

  it("Should hide the bar without fan-out and summarize the round alone", () => {
    const progress = buildRunProgress(run({ status: "watching", generation: 3 }), [], null);
    render(<LoopRunProgressPanel title="React to inbound review requests" progress={progress} />);
    expect(screen.queryByTestId("loop-run-progress-bar")).not.toBeInTheDocument();
    expect(screen.getByTestId("loop-run-progress-meta")).toHaveTextContent("Round 3");
  });
});

describe("LoopRunStoryTimeline", () => {
  it("Should render story rows with verbatim micro labels and mono issue ids", () => {
    render(
      <LoopRunStoryTimeline
        rows={[
          storyRow(),
          storyRow({
            key: "f-2",
            kind: "node_succeeded",
            tone: "success",
            icon: "node-done",
            title: "Fix batches — 2 of 3 clean",
            sub: undefined,
            issues: undefined,
            micro: "node_succeeded · fix_batches[1–2]",
          }),
        ]}
        isLive
        goalNodeIds={EMPTY_GOAL_IDS}
        goalTurns={[]}
      />
    );
    const rows = screen.getAllByTestId("loop-story-row");
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent("Check: not clean yet");
    expect(rows[0]).toHaveTextContent("gate_verdict · revise");
    expect(screen.getByTestId("loop-story-issues")).toHaveTextContent("issue_022 — no decision");
    expect(rows[1]).toHaveTextContent("node_succeeded · fix_batches[1–2]");
  });

  it("Should stay quiet before any frame arrives", () => {
    render(<LoopRunStoryTimeline rows={[]} isLive goalNodeIds={EMPTY_GOAL_IDS} goalTurns={[]} />);
    expect(screen.getByText("Nothing yet")).toBeInTheDocument();
  });
});

describe("LoopRunNowCard", () => {
  const now: LoopStoryNow = {
    nodeId: "fix_batches",
    label: "fix batches",
    itemIndex: 3,
    generation: 2,
    startedAt: "2026-07-22T14:36:00Z",
    taskLink: { taskId: "task_9", taskRunId: "tr_9" },
    isGoalNode: false,
  };

  it("Should show the running step with its task-run link and ticking trail", () => {
    render(
      <LoopRunNowCard
        run={run({ status: "running", generation: 2, reattempt_strategy: "failed_only" })}
        now={now}
        isLive
        stepElapsedLabel="4m 09s"
      />
    );
    expect(screen.getByTestId("loop-run-now-label")).toHaveTextContent("Working on fix batches");
    expect(screen.getByText("4m 09s")).toBeInTheDocument();
    expect(screen.getByTestId("loop-run-now-task-link")).toHaveAttribute(
      "data-params",
      JSON.stringify({ id: "task_9", runId: "tr_9" })
    );
  });

  it("Should render the watching card with cadence and last wake", () => {
    render(
      <LoopRunNowCard
        run={run({ status: "watching" })}
        now={null}
        isLive
        watchLastWakeAt="2026-07-22T14:40:00Z"
        watchCadence="checks every 30s"
      />
    );
    expect(screen.getByTestId("loop-run-now-label")).toHaveTextContent(
      "Watching for the next event"
    );
    expect(screen.getByText("checks every 30s")).toBeInTheDocument();
  });

  it("Should render the paused explainer without a live badge", () => {
    render(<LoopRunNowCard run={run({ status: "paused", generation: 2 })} now={null} isLive />);
    expect(screen.getByTestId("loop-run-now-label")).toHaveTextContent("Paused");
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });
});

describe("LoopRunNeedsYouCard", () => {
  it("Should route each closed decision with the streamed gate id", () => {
    const onDecision = vi.fn();
    render(
      <LoopRunNeedsYouCard
        run={run({ status: "needs-approval" })}
        request={{
          gateId: "budget",
          title: "Time limit reached — continue this run?",
          facts: [{ label: "Round", value: "3" }],
        }}
        fallbackFacts={[]}
        onDecision={onDecision}
      />
    );
    fireEvent.click(screen.getByTestId("loop-approval-approve"));
    fireEvent.click(screen.getByTestId("loop-approval-request-changes"));
    fireEvent.click(screen.getByTestId("loop-approval-reject"));
    expect(onDecision).toHaveBeenNthCalledWith(1, "approve", "budget");
    expect(onDecision).toHaveBeenNthCalledWith(2, "request_changes", "budget");
    expect(onDecision).toHaveBeenNthCalledWith(3, "reject", "budget");
    expect(screen.getByText(/on_exceeded: halt/)).toBeInTheDocument();
  });

  it("Should stand in with the usage snapshot when the payload has no facts", () => {
    render(
      <LoopRunNeedsYouCard
        run={run({ status: "needs-approval" })}
        request={null}
        fallbackFacts={[
          { label: "Time used", value: "45m 00s of 45m" },
          { label: "Round", value: "3" },
        ]}
        onDecision={vi.fn()}
      />
    );
    const facts = screen.getAllByTestId("loop-run-fact");
    expect(facts).toHaveLength(2);
    expect(facts[0]).toHaveTextContent("45m 00s of 45m");
  });
});

describe("LoopRunOutcomeCard", () => {
  it("Should tell what went wrong with recovery and restart, and never an invented Vault CTA", () => {
    render(
      <LoopRunOutcomeCard
        run={run({ status: "failed", created_at: "2026-01-01T00:00:00Z" })}
        failure={{
          kind: "coordinator_failure",
          code: "coordinator_failed",
          cause: "The Loop coordinator failed before it could settle the run.",
          recovery: "Inspect daemon logs for the correlated coordinator run, then start a new run.",
        }}
        fromStatus="running"
        terminalAt="2026-01-01T00:26:41Z"
        repeatedIssueIds={[]}
        onStartNewRun={vi.fn()}
      />
    );
    expect(screen.getByTestId("loop-run-outcome-title")).toHaveTextContent("Failed after 26m 41s");
    expect(
      screen.getByText("The Loop coordinator failed before it could settle the run.")
    ).toBeInTheDocument();
    expect(screen.getByTestId("loop-run-start-new")).toBeInTheDocument();
    // No Loops producer emits a Vault-repairable failure, so the CTA must be gone.
    expect(screen.queryByText("Open Vault")).not.toBeInTheDocument();
    expect(
      screen.getByText("status_changed · running → failed · cause coordinator_failed")
    ).toBeInTheDocument();
  });

  it("Should fire the same-inputs restart", () => {
    const onStartNewRun = vi.fn();
    render(
      <LoopRunOutcomeCard
        run={run({ status: "exhausted", generation: 50, iteration_cap: 50 })}
        failure={null}
        repeatedIssueIds={[]}
        onStartNewRun={onStartNewRun}
      />
    );
    expect(screen.getByTestId("loop-run-outcome-title")).toHaveTextContent("Round cap reached");
    fireEvent.click(screen.getByTestId("loop-run-start-new"));
    expect(onStartNewRun).toHaveBeenCalledTimes(1);
  });

  it("Should not invent a Time limit when the terminal frame lands before the wall budget", () => {
    render(
      <LoopRunOutcomeCard
        run={run({
          status: "exhausted",
          budget_wall_sec: 999_999,
          iteration_cap: 0,
          budget_tokens: 0,
          created_at: "2026-01-01T00:00:00Z",
        })}
        failure={null}
        terminalAt="2026-01-01T00:04:12Z"
        repeatedIssueIds={[]}
        onStartNewRun={vi.fn()}
      />
    );
    const title = screen.getByTestId("loop-run-outcome-title");
    expect(title).toHaveTextContent("Run exhausted after 4m 12s");
    expect(title).not.toHaveTextContent("Time limit");
  });

  it("Should report the Time limit from the terminal frame even when last_progress predates the cap", () => {
    // The status CAS never refreshes last_progress_at: the durable span (created →
    // last_progress) sits under the 30m cap, but the terminal status_changed `at`
    // is the truth and crosses it, so the run must read as a Time limit.
    render(
      <LoopRunOutcomeCard
        run={run({
          status: "exhausted",
          budget_wall_sec: 1_800,
          iteration_cap: 0,
          budget_tokens: 0,
          created_at: "2026-01-01T00:00:00Z",
          last_progress_at: "2026-01-01T00:20:00Z",
        })}
        failure={null}
        terminalAt="2026-01-01T00:31:00Z"
        repeatedIssueIds={[]}
        onStartNewRun={vi.fn()}
      />
    );
    expect(screen.getByTestId("loop-run-outcome-title")).toHaveTextContent(
      "Time limit reached after 31m 00s"
    );
  });

  it("Should render the stalled explainer with the repeated blocking ids", () => {
    render(
      <LoopRunOutcomeCard
        run={run({ status: "stalled" })}
        failure={null}
        noProgressWindow={2}
        repeatedIssueIds={["issue_022", "issue_024"]}
        onStartNewRun={vi.fn()}
      />
    );
    expect(screen.getByTestId("loop-run-outcome-title")).toHaveTextContent("Stalled — no progress");
    expect(screen.getByTestId("loop-run-stalled-issues")).toHaveTextContent("issue_022");
    expect(screen.queryByTestId("loop-run-start-new")).not.toBeInTheDocument();
  });

  it("Should keep the no-op outcome a quiet note", () => {
    render(
      <LoopRunOutcomeCard
        run={run({ status: "no-op" })}
        failure={null}
        repeatedIssueIds={[]}
        onStartNewRun={vi.fn()}
      />
    );
    expect(screen.getByTestId("loop-run-noop-note")).toHaveTextContent("Ran, nothing to do");
  });
});

describe("LoopRunControls", () => {
  it("Should show Pause + Stop while running and fire the callback", () => {
    const onPause = vi.fn();
    render(
      <LoopRunControls status="running" onPause={onPause} onResume={vi.fn()} onStop={vi.fn()} />
    );
    expect(screen.getByTestId("loop-run-pause")).toBeInTheDocument();
    expect(screen.getByTestId("loop-run-stop")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-run-resume")).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId("loop-run-pause"));
    expect(onPause).toHaveBeenCalledTimes(1);
  });

  it("Should show Resume while paused and render nothing for a terminal run", () => {
    const { rerender } = render(
      <LoopRunControls status="paused" onPause={vi.fn()} onResume={vi.fn()} onStop={vi.fn()} />
    );
    expect(screen.getByTestId("loop-run-resume")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-run-pause")).not.toBeInTheDocument();
    rerender(
      <LoopRunControls status="done" onPause={vi.fn()} onResume={vi.fn()} onStop={vi.fn()} />
    );
    expect(screen.queryByTestId("loop-run-controls")).not.toBeInTheDocument();
  });
});

describe("LoopRunUsageRail", () => {
  it("Should render the four rows with ceilings, ∞, and the policy note", () => {
    const rows = buildRunUsage(
      run({
        tokens_used: 268_000,
        budget_tokens: 1_500_000,
        budget_wall_sec: 2_700,
        generation: 2,
        iteration_cap: 0,
      }),
      1_334
    );
    render(
      <LoopRunUsageRail
        rows={rows}
        note="Cost is an estimate (tokens × rate), never a cap. If a limit is reached, this run stops as exhausted."
      />
    );
    expect(screen.getByTestId("loop-run-usage-time")).toHaveTextContent("22m 14s/ 45m");
    expect(screen.getByTestId("loop-run-usage-rounds")).toHaveTextContent("2/ ∞");
    expect(screen.getByTestId("loop-run-usage-cost")).toHaveTextContent("~$1.34estimate");
    expect(screen.getByTestId("loop-run-usage-note")).toHaveTextContent("never a cap");
  });
});

describe("LoopRunAboutRail", () => {
  it("Should link the loop, pin the version, and expose the run id", () => {
    render(
      <LoopRunAboutRail
        run={run()}
        versionLabel="v4 · pinned"
        inputRows={[{ key: "pr", label: "PR", value: "128", isAgent: false }]}
        startedBy="A webhook"
        workspaceLabel="Home"
      />
    );
    expect(screen.getByTestId("loop-run-about-loop")).toHaveAttribute(
      "data-params",
      JSON.stringify({ name: "software-delivery" })
    );
    expect(screen.getByTestId("loop-run-about-version")).toHaveTextContent("v4 · pinned");
    expect(screen.getByTestId("loop-run-input-pr")).toHaveTextContent("128");
    expect(screen.getByTestId("loop-run-about-started-by")).toHaveTextContent("A webhook");
    expect(screen.getByTestId("loop-run-about-workspace")).toHaveTextContent("Home");
    expect(screen.getByTestId("loop-run-about-id")).toHaveTextContent(run().id);
  });
});

describe("LoopRunSubhead", () => {
  it("Should speak Started while live and Ended with the frozen span on terminal runs", () => {
    const { rerender } = render(
      <LoopRunSubhead
        run={run({ status: "running" })}
        subject="PR 128"
        hasWatchSource
        elapsedLabel="22m 14s"
      />
    );
    expect(screen.getByTestId("loop-run-subhead")).toHaveTextContent("Started");
    expect(screen.getByTestId("loop-run-subject")).toHaveTextContent("Watching PR 128");
    rerender(
      <LoopRunSubhead
        run={run({ status: "failed" })}
        subject="PR 128"
        hasWatchSource
        elapsedLabel="26m 41s"
      />
    );
    expect(screen.getByTestId("loop-run-subhead")).toHaveTextContent("Ended");
    expect(screen.getByTestId("loop-run-elapsed")).toHaveTextContent("26m 41s");
  });
});
