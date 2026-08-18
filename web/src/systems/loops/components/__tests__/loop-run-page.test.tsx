import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { loopNodeLifecycleFixture } from "../../testing/loop-node-lifecycle-fixture";

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
const { LoopRunAttentionPanel, LoopRunWaitingPanel } =
  await import("../run-page/loop-run-parked-panels");
const { LoopRunNeedsYouCard } = await import("../run-page/loop-run-needs-you-card");
const { projectLoopRequest } = await import("../../lib/loop-request-model");
const { pendingReviewRequest } = await import("../../mocks/fixture-graph-eng-requests");
const { LoopRunOutcomeCard } = await import("../run-page/loop-run-outcome-card");
const { LoopRunControls } = await import("../run-page/loop-run-controls");
const { LoopNodeControlMenu } = await import("../run-page/loop-node-control-menu");
const { LoopNodeRowActions } = await import("../run-page/loop-node-row-actions");
const { LoopRunOverflowMenu } = await import("../run-page/loop-run-overflow-menu");
const { LoopRunControlDialog } = await import("../run-page/loop-run-control-dialog");
const { LoopNodeControlDialog } = await import("../run-page/loop-node-control-dialog");
const { LoopQuarantineSheet } = await import("../run-page/loop-quarantine-sheet");
const { LoopRunWaitsRail } = await import("../run-page/loop-run-waits-rail");
const { LOOP_NODE_VERB_PRESENTATION, loopNodeVerbs, loopNodeWaitResumeItemIndex } =
  await import("../../lib/loop-node-controls");
const { buildNodeNowLines } = await import("../../lib/loop-node-now-view");
const { quarantineChainRows } = await import("../../lib/loop-quarantine-entry");
const { loopNodeStateStrip, loopNodeVerbConfirmCopy, loopRunStateStrip } =
  await import("../../lib/loop-node-verb-copy");
const { checkLoopWaitPayload, loopWaitExpectRequiredKeys } =
  await import("../../lib/loop-node-wait-payload");
type LoopNodeLifecycle = import("../../lib/loop-node-lifecycle").LoopNodeLifecycle;
const { LoopRunUsageRail } = await import("../run-page/loop-run-usage-rail");
const { LoopRunAboutRail } = await import("../run-page/loop-run-about-rail");
const { LoopRunResolvedRuntimes } = await import("../run-page/loop-run-resolved-runtimes");
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
    sub: "Verdict revise.",
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
          parent_generation: 1,
          origin: "gate_revise",
          route_causes: [],
          verdicts: [],
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

  it("Should render score, best, and restored provenance from a generation row", () => {
    const originalScroll = Element.prototype.scrollIntoView;
    const scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView = scrollIntoView;
    window.location.hash = "#loop-generation-3";
    try {
      render(
        <LoopRunStoryTimeline
          rows={[
            storyRow({
              kind: "generation_started",
              generation: 3,
              score: 0.7,
              isBest: true,
              originLabel: "Restored from gen 1",
            }),
          ]}
          isLive={false}
          goalNodeIds={EMPTY_GOAL_IDS}
          goalTurns={[]}
        />
      );

      const row = screen.getByTestId("loop-story-row");
      expect(row).toHaveAttribute("id", "loop-generation-3");
      expect(row).toHaveTextContent("score 0.70");
      expect(row).toHaveTextContent("Best");
      expect(row).toHaveTextContent("Restored from gen 1");
      expect(scrollIntoView).toHaveBeenCalledWith({ block: "start" });
    } finally {
      Element.prototype.scrollIntoView = originalScroll;
      window.location.hash = "";
    }
  });

  it("Should stay quiet before any frame arrives", () => {
    render(<LoopRunStoryTimeline rows={[]} isLive goalNodeIds={EMPTY_GOAL_IDS} goalTurns={[]} />);
    expect(screen.getByText("Nothing yet")).toBeInTheDocument();
  });

  it("Should keep the newest generation flat and fold older generations", () => {
    render(
      <LoopRunStoryTimeline
        rows={[
          storyRow({
            key: "g2",
            generation: 2,
            title: "Newest beat",
            micro: "generation_started · gen 2",
          }),
          storyRow({
            key: "g1",
            generation: 1,
            title: "Older beat",
            micro: "generation_started · gen 1",
          }),
        ]}
        isLive={false}
        goalNodeIds={EMPTY_GOAL_IDS}
        goalTurns={[]}
      />
    );
    expect(screen.getByText("Newest beat")).toBeInTheDocument();
    expect(screen.getByTestId("loop-story-generation-1")).toHaveTextContent(
      "Generation 1 · 1 event"
    );
    expect(screen.queryByText("Older beat")).not.toBeInTheDocument();
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

  it("Should open the live agent session in one click when the cell has one bound", () => {
    render(
      <LoopRunNowCard
        run={run({ status: "running", generation: 2 })}
        now={now}
        isLive
        nodeSessions={new Map([["fix_batches", "sess-live-agent"]])}
        nodeLines={[
          {
            nodeId: "execute_task",
            state: "retrying",
            headline: "execute task is retrying",
            body: "Next attempt shortly.",
            chip: null,
            provenance: null,
            micro: "execute_task · gen 2",
          },
        ]}
        nodesById={
          new Map([["execute_task", loopNodeLifecycleFixture({ nodeId: "execute_task" })]])
        }
      />
    );
    expect(screen.getByTestId("loop-run-now-session-link")).toHaveAttribute(
      "data-params",
      JSON.stringify({ id: "sess-live-agent" })
    );
    // A lifecycle row without a bound session offers no dead session affordance.
    expect(screen.queryByTestId("loop-run-now-node-session-execute_task")).not.toBeInTheDocument();
  });
});

describe("LoopRunAttentionPanel", () => {
  it("Should collapse consumers behind one quarantined producer and route its entry", () => {
    const onOpenQuarantine = vi.fn();
    const consumers = ["collect", "review", "verify", "approve"].map(nodeId =>
      loopNodeLifecycleFixture({
        nodeId,
        label: nodeId,
        attentionFlag: "dependency_quarantined",
        attentionReason: `node ${nodeId} requires parked producer execute_task`,
        attentionProducerNodeId: "execute_task",
        generation: 3,
      })
    );
    render(<LoopRunAttentionPanel nodes={consumers} onOpenQuarantine={onOpenQuarantine} />);

    const row = screen.getByTestId("loop-run-attention-producer-execute_task");
    expect(row).toHaveTextContent("execute task is quarantined");
    expect(row).toHaveTextContent(
      "collect, review, verify and approve are parked behind it until it is requeued."
    );
    expect(screen.getByTestId("loop-run-attention")).toHaveTextContent("4 flagged");

    const buttons = screen.getAllByRole("button", { name: "Open quarantine entry" });
    expect(buttons).toHaveLength(1);
    fireEvent.click(buttons[0]);
    expect(onOpenQuarantine).toHaveBeenCalledWith("execute_task");
  });

  it("Should keep the daemon's own reason for non-dependency flags", () => {
    render(
      <LoopRunAttentionPanel
        nodes={[
          loopNodeLifecycleFixture({
            nodeId: "watcher",
            label: "watcher",
            attentionFlag: "expired_wait",
            attentionReason: "wait expired without a timeout route",
          }),
        ]}
      />
    );
    expect(screen.getByTestId("loop-run-attention-watcher")).toHaveTextContent(
      "watcher waited past its deadline"
    );
    expect(screen.queryByRole("button", { name: "Open quarantine entry" })).not.toBeInTheDocument();
  });

  it("Should keep dependency consumers without producer provenance as separate rows", () => {
    const consumers = ["collect", "review"].map(nodeId =>
      loopNodeLifecycleFixture({
        nodeId,
        label: nodeId,
        attentionFlag: "dependency_quarantined",
        attentionReason: `node ${nodeId} requires an unavailable producer`,
        attentionProducerNodeId: "",
      })
    );
    render(<LoopRunAttentionPanel nodes={consumers} />);

    expect(screen.getByTestId("loop-run-attention-collect")).toBeInTheDocument();
    expect(screen.getByTestId("loop-run-attention-review")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-run-attention-producer-unknown")).not.toBeInTheDocument();
    expect(screen.getByTestId("loop-run-attention-inventory-link")).toBeInTheDocument();
  });
});

describe("LoopRunWaitingPanel", () => {
  it("Should show the ladder strip only when cursor or next stamp exist", () => {
    render(
      <LoopRunWaitingPanel
        runId="r-7c4e19"
        nodes={[
          loopNodeLifecycleFixture({
            nodeId: "approve_fix",
            label: "approve fix",
            state: "waiting",
            parked: true,
            waits: [
              {
                nodeId: "approve_fix",
                generation: 2,
                itemIndex: 0,
                kind: "approval_escalation",
                claimState: "intervention_required",
                escalationCursor: 1,
                nextEscalationAt: "2026-08-03T15:02:00Z",
                admissionFailures: 0,
                ageSeconds: 120,
                createdAt: "2026-08-03T14:00:00Z",
                expect: { env: "staging" },
              },
            ],
          }),
        ]}
      />
    );
    expect(screen.getByTestId("loop-run-wait-ladder-approve_fix-0")).toHaveTextContent(
      "step 1 done"
    );
    expect(screen.getByTestId("loop-run-wait-approve_fix-0")).toHaveTextContent(
      "The ladder walks its steps"
    );
    expect(screen.getByTestId("loop-run-wait-approve_fix-0")).toHaveTextContent(
      '{"env":"staging"}'
    );
  });

  it("Should not describe a ladder that is not on screen", () => {
    render(
      <LoopRunWaitingPanel
        runId="r-7c4e19"
        nodes={[
          loopNodeLifecycleFixture({
            nodeId: "approve_fix",
            label: "approve fix",
            state: "waiting",
            parked: true,
            waits: [
              {
                nodeId: "approve_fix",
                generation: 2,
                itemIndex: 0,
                kind: "approval_escalation",
                claimState: "waiting",
                escalationCursor: 0,
                admissionFailures: 0,
                ageSeconds: 30,
                createdAt: "2026-08-03T14:00:00Z",
                expect: undefined,
              },
            ],
          }),
        ]}
      />
    );
    expect(screen.queryByTestId("loop-run-wait-ladder-approve_fix-0")).not.toBeInTheDocument();
    expect(screen.getByTestId("loop-run-wait-approve_fix-0")).toHaveTextContent(
      "Waiting for your decision"
    );
    expect(screen.getByTestId("loop-run-wait-approve_fix-0")).not.toHaveTextContent("ladder");
  });
});

describe("LoopRunNeedsYouCard", () => {
  it("Should keep same-node requests from different generations distinct and retry context", () => {
    const onRequestFullContext = vi.fn();
    const view = projectLoopRequest(pendingReviewRequest, {
      nowMs: Date.parse("2026-08-17T10:00:00Z"),
      runStatus: "running",
    });
    render(
      <LoopRunNeedsYouCard
        fallbackFacts={[]}
        onDecision={vi.fn()}
        request={null}
        requestState={{
          engagedKey: `3:${pendingReviewRequest.node_id}:0`,
          fullContextError: "Context is temporarily unavailable",
          onRequestFullContext,
        }}
        requests={[
          { ...view, request: { ...view.request, generation: 2 } },
          { ...view, request: { ...view.request, generation: 3 } },
        ]}
        run={run({ status: "running" })}
        showApproval={false}
      />
    );

    expect(screen.getAllByTestId("loop-request-card")).toHaveLength(2);
    expect(screen.getByRole("alert")).toHaveTextContent("Context is temporarily unavailable");
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(onRequestFullContext).toHaveBeenCalledWith(3, pendingReviewRequest.node_id, 0);
  });

  it("Should focus the requested form when opened from an attention deep link", () => {
    render(
      <LoopRunNeedsYouCard
        fallbackFacts={[]}
        onDecision={vi.fn()}
        request={null}
        requestFocus={{ nodeId: pendingReviewRequest.node_id, itemIndex: 0 }}
        requests={[
          projectLoopRequest(pendingReviewRequest, {
            nowMs: Date.parse("2026-08-17T10:00:00Z"),
            runStatus: "running",
          }),
        ]}
        run={run({ status: "running" })}
        showApproval={false}
      />
    );

    expect(screen.getByTestId("loop-request-decision-approve")).toHaveFocus();
  });

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

  it("Should render a quarantine row without the approval block", () => {
    const onOpenQuarantine = vi.fn();
    render(
      <LoopRunNeedsYouCard
        run={run({ status: "running" })}
        request={null}
        fallbackFacts={[]}
        showApproval={false}
        quarantinedNodes={[
          loopNodeLifecycleFixture({
            nodeId: "fix_batch",
            label: "fix batch",
            state: "quarantined",
            parked: true,
            quarantined: true,
          }),
        ]}
        onOpenQuarantine={onOpenQuarantine}
        onDecision={vi.fn()}
      />
    );
    expect(screen.getByTestId("loop-run-needs-quarantine-fix_batch-g2")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-run-needs-approval")).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId("loop-run-needs-open-quarantine-fix_batch-g2"));
    expect(onOpenQuarantine).toHaveBeenCalledWith("fix_batch");
  });

  it("Should keep distinct testids when fan-out quarantines two items of the same node", () => {
    const onOpenQuarantine = vi.fn();
    render(
      <LoopRunNeedsYouCard
        run={run({ status: "running" })}
        request={null}
        fallbackFacts={[]}
        showApproval={false}
        quarantinedNodes={[
          loopNodeLifecycleFixture({
            nodeId: "fix_batch",
            label: "fix batch",
            state: "quarantined",
            parked: true,
            quarantined: true,
            itemIndex: 0,
            generation: 2,
          }),
          loopNodeLifecycleFixture({
            nodeId: "fix_batch",
            label: "fix batch",
            state: "quarantined",
            parked: true,
            quarantined: true,
            itemIndex: 1,
            generation: 2,
          }),
        ]}
        onOpenQuarantine={onOpenQuarantine}
        onDecision={vi.fn()}
      />
    );
    expect(screen.getByTestId("loop-run-needs-quarantine-fix_batch-0-g2")).toBeInTheDocument();
    expect(screen.getByTestId("loop-run-needs-quarantine-fix_batch-1-g2")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("loop-run-needs-open-quarantine-fix_batch-0-g2"));
    fireEvent.click(screen.getByTestId("loop-run-needs-open-quarantine-fix_batch-1-g2"));
    expect(onOpenQuarantine).toHaveBeenNthCalledWith(1, "fix_batch");
    expect(onOpenQuarantine).toHaveBeenNthCalledWith(2, "fix_batch");
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
        cause=""
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

  it("Should point an exhausted run to its persisted best generation", () => {
    render(
      <LoopRunOutcomeCard
        run={run({
          status: "exhausted",
          generation: 3,
          iteration_cap: 3,
          best_generation: 1,
          best_score: 0.7,
        })}
        failure={null}
        repeatedIssueIds={[]}
        onStartNewRun={vi.fn()}
      />
    );

    expect(screen.getByTestId("loop-run-best-pointer").querySelector("a")).toHaveAttribute(
      "href",
      "#loop-generation-1"
    );
    expect(screen.getByTestId("loop-run-best-pointer")).toHaveTextContent(
      "Best result · Gen 1 · 0.70"
    );
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
  it("Should show Pause + Cancel while running and fire the callback", () => {
    const onPause = vi.fn();
    render(
      <LoopRunControls status="running" onPause={onPause} onResume={vi.fn()} onCancel={vi.fn()} />
    );
    expect(screen.getByTestId("loop-run-pause")).toBeInTheDocument();
    expect(screen.getByTestId("loop-run-cancel")).toHaveTextContent("Cancel run");
    expect(screen.queryByTestId("loop-run-resume")).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId("loop-run-pause"));
    expect(onPause).toHaveBeenCalledTimes(1);
  });

  it("Should show Resume while paused and render nothing for a terminal run", () => {
    const { rerender } = render(
      <LoopRunControls status="paused" onPause={vi.fn()} onResume={vi.fn()} onCancel={vi.fn()} />
    );
    expect(screen.getByTestId("loop-run-resume")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-run-pause")).not.toBeInTheDocument();
    rerender(
      <LoopRunControls status="done" onPause={vi.fn()} onResume={vi.fn()} onCancel={vi.fn()} />
    );
    expect(screen.queryByTestId("loop-run-controls")).not.toBeInTheDocument();
  });

  // WT-004 (run half): kill is reachable only from the overflow, and a pause
  // that has been requested but not yet landed offers no second pause.
  it("Should replace Pause with a disabled Pausing once a pause is requested", () => {
    render(
      <LoopRunControls
        status="running"
        pauseRequested
        onPause={vi.fn()}
        onResume={vi.fn()}
        onCancel={vi.fn()}
      />
    );
    expect(screen.queryByTestId("loop-run-pause")).not.toBeInTheDocument();
    expect(screen.getByTestId("loop-run-pausing")).toBeDisabled();
    // Kill is the destructive escape and never sits in this row.
    expect(screen.queryByTestId("loop-run-kill")).not.toBeInTheDocument();
  });
});

// WT-004 (run confirmation): cancel and kill are destructive and irreversible
// for the run, so neither may commit without restating the state it acts on.
describe("LoopRunControlDialog", () => {
  it("Should restate the run's current identity and status before killing", () => {
    const onConfirm = vi.fn();
    render(
      <LoopRunControlDialog
        generation={2}
        onConfirm={onConfirm}
        onOpenChange={vi.fn()}
        runId="r-7c4e19"
        status="running"
        verb="kill"
      />
    );
    const dialog = screen.getByTestId("loop-run-control-dialog");
    expect(dialog).toHaveTextContent("Kill run r-7c4e19?");
    // The strip is the guard against acting on a stale screen.
    expect(dialog).toHaveTextContent("r-7c4e19 is running · generation 2");
    expect(dialog).not.toHaveTextContent("in flight");
    expect(dialog).not.toHaveTextContent("waiting on you");
    expect(dialog).toHaveTextContent("interrupted mid-step");
    expect(dialog).toHaveTextContent("cause operator_kill");
    fireEvent.click(screen.getByRole("button", { name: "Kill run" }));
    expect(onConfirm).toHaveBeenCalledWith("kill");
  });

  it("Should describe cancel as a cooperative wind-down, not an immediate stop", () => {
    render(
      <LoopRunControlDialog
        generation={4}
        onConfirm={vi.fn()}
        onOpenChange={vi.fn()}
        runId="r-7c4e19"
        status="watching"
        verb="cancel"
      />
    );
    const dialog = screen.getByTestId("loop-run-control-dialog");
    expect(dialog).toHaveTextContent("Cancel run r-7c4e19?");
    expect(dialog).toHaveTextContent("r-7c4e19 is watching · generation 4");
    expect(dialog).toHaveTextContent("in-flight lanes drain");
    expect(dialog).toHaveTextContent("cause operator_cancel");
  });

  it("Should keep the daemon rejection visible in the open dialog", () => {
    render(
      <LoopRunControlDialog
        error="run already reached a terminal state"
        generation={4}
        onConfirm={vi.fn()}
        onOpenChange={vi.fn()}
        runId="r-7c4e19"
        status="watching"
        verb="cancel"
      />
    );
    expect(screen.getByTestId("loop-run-control-dialog")).toHaveTextContent(
      "run already reached a terminal state"
    );
  });

  it("Should append non-zero lane counts and elapsed to the run strip", () => {
    render(
      <LoopRunControlDialog
        elapsedLabel="22m 14s"
        generation={2}
        inFlightCount={2}
        onConfirm={vi.fn()}
        onOpenChange={vi.fn()}
        runId="r-7c4e19"
        status="running"
        verb="cancel"
        waitingOnYouCount={1}
      />
    );
    expect(screen.getByTestId("loop-run-control-dialog")).toHaveTextContent(
      "r-7c4e19 is running · 2 lanes in flight · 1 waiting on you · generation 2 · 22m 14s"
    );
  });

  it("Should render nothing until a verb is actually pending", () => {
    render(
      <LoopRunControlDialog
        generation={1}
        onConfirm={vi.fn()}
        onOpenChange={vi.fn()}
        runId="r-7c4e19"
        status="running"
        verb={null}
      />
    );
    expect(screen.queryByTestId("loop-run-control-dialog")).not.toBeInTheDocument();
  });
});

// WT-004 (run kill): kill reaches the operator only through the surface's single
// ⋯ overflow, and only while the run is live.
describe("LoopRunOverflowMenu kill", () => {
  it("Should offer Kill inside the overflow and report the click", async () => {
    const user = userEvent.setup();
    const onKill = vi.fn();
    render(<LoopRunOverflowMenu loopName="review-and-fix" onKill={onKill} />);
    const trigger = screen.getByTestId("loop-run-more");
    fireEvent.pointerDown(trigger, { button: 0, pointerType: "mouse" });
    await user.click(trigger);
    await user.click(await screen.findByTestId("loop-run-kill"));
    expect(onKill).toHaveBeenCalledTimes(1);
  });

  it("Should omit Kill entirely when the run can no longer be killed", async () => {
    const user = userEvent.setup();
    render(<LoopRunOverflowMenu loopName="review-and-fix" />);
    const trigger = screen.getByTestId("loop-run-more");
    fireEvent.pointerDown(trigger, { button: 0, pointerType: "mouse" });
    await user.click(trigger);
    expect(await screen.findByTestId("loop-run-view-definition")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-run-inspect")).not.toBeInTheDocument();
    expect(screen.queryByTestId("loop-run-kill")).not.toBeInTheDocument();
  });
});

// WT-004: node verbs are a pure function of daemon-declared state. Testing the
// gate directly (rather than only through a rendered menu) is what makes "no
// verb for a state the payload doesn't declare" checkable for every state.
describe("loopNodeVerbs", () => {
  const node = (overrides: Partial<LoopNodeLifecycle> = {}): LoopNodeLifecycle =>
    loopNodeLifecycleFixture(overrides);

  const wait = (claimState: string, itemIndex = 0) => ({
    nodeId: "task_03",
    generation: 2,
    itemIndex,
    kind: "event",
    claimState,
    escalationCursor: 0,
    admissionFailures: 0,
    ageSeconds: 120,
    createdAt: "2026-08-03T14:00:00Z",
    expect: undefined,
  });

  it("Should offer pause/cancel/kill on a running node and never resume or requeue", () => {
    expect(loopNodeVerbs(node(), "running")).toEqual(["pause", "cancel", "kill"]);
  });

  it("Should offer the three resume modes on a paused node and no requeue", () => {
    expect(loopNodeVerbs(node({ paused: true, state: "paused" }), "running")).toEqual([
      "resume",
      "resume-reset-attempts",
      "resume-immediate",
      "cancel",
      "kill",
    ]);
  });

  it("Should offer requeue only on a quarantined node, and never resume", () => {
    const verbs = loopNodeVerbs(node({ quarantined: true, state: "quarantined" }), "running");
    expect(verbs).toEqual(["open-quarantine", "requeue", "cancel", "kill"]);
    expect(verbs).not.toContain("resume");
  });

  it("Should offer a payload resume only while a wait is genuinely open", () => {
    expect(loopNodeVerbs(node({ state: "waiting", waits: [wait("waiting")] }), "running")).toEqual([
      "resume-wait",
      "cancel",
      "kill",
    ]);
    // A resumed cell no longer holds the node, so the wait verb disappears.
    expect(loopNodeVerbs(node({ waits: [wait("resumed")] }), "running")).toEqual([
      "pause",
      "cancel",
      "kill",
    ]);
  });

  it("Should target the open wait instead of an earlier historical cell", () => {
    const lifecycle = node({
      state: "waiting",
      waits: [wait("resumed", 2), wait("waiting", 7)],
    });

    expect(loopNodeWaitResumeItemIndex(lifecycle)).toBe(7);
  });

  it("Should narrow to kill while canceling and offer nothing once canceled", () => {
    expect(loopNodeVerbs(node({ cancelState: "draining", state: "canceling" }), "running")).toEqual(
      ["kill"]
    );
    expect(loopNodeVerbs(node({ cancelState: "canceled", state: "canceled" }), "running")).toEqual(
      []
    );
  });

  it("Should offer no verb at all once the run is terminal", () => {
    for (const status of ["done", "failed", "canceled", "exhausted"]) {
      expect(loopNodeVerbs(node({ paused: true, state: "paused" }), status)).toEqual([]);
    }
  });
});

describe("buildNodeNowLines", () => {
  it("Should present cancellation as a stable state without leaking the daemon token", () => {
    const [line] = buildNodeNowLines(
      [
        loopNodeLifecycleFixture({
          state: "canceling",
          cancelState: "draining",
          cancelProvenance: {
            actor_kind: "user",
            actor_id: "pedro",
            reason: "operator request",
            requested_at: "2026-08-03T14:00:00Z",
          },
        }),
      ],
      null,
      {}
    );
    if (!line) throw new Error("expected a current-state line for the canceling node");
    expect(line.state).toBe("canceling");
    expect(line.body).toContain("Cancellation is in progress — requested by you (pedro)");
    expect(line.body).not.toContain("draining");
    expect(line.micro).toContain("cancel_state draining");
  });

  it("Should omit the pause timestamp clause when the daemon timestamp is invalid", () => {
    const [line] = buildNodeNowLines(
      [
        loopNodeLifecycleFixture({
          state: "paused",
          parked: true,
          paused: true,
          pauseProvenance: {
            actor_kind: "system",
            actor_id: "autopause",
            reason: "retry storm",
            requested_at: "not-a-timestamp",
          },
        }),
      ],
      null,
      {}
    );
    if (!line) throw new Error("expected a current-state line for the paused node");
    expect(line.provenance).toBe("paused by system autopause · reason “retry storm”");
    expect(line.provenance).not.toContain("at ");
  });

  it("Should omit waiting and quarantined lanes from Happening now", () => {
    const lines = buildNodeNowLines(
      [
        loopNodeLifecycleFixture({ state: "waiting", parked: true }),
        loopNodeLifecycleFixture({
          nodeId: "task_04",
          state: "quarantined",
          parked: true,
          quarantined: true,
        }),
        loopNodeLifecycleFixture({
          nodeId: "task_05",
          state: "paused",
          parked: true,
          paused: true,
          itemIndex: 1,
        }),
      ],
      null,
      {}
    );
    expect(lines.map(line => line.state)).toEqual(["paused"]);
    expect(lines[0]?.micro).toContain("task_05[1] · gen 2");
  });
});

describe("LoopNodeControlDialog", () => {
  const node = loopNodeLifecycleFixture({
    state: "paused",
    parked: true,
    paused: true,
  });

  it("Should reset local form choices whenever a control request is reopened", async () => {
    const user = userEvent.setup();
    const props = {
      isPending: false,
      onConfirm: vi.fn(),
      onOpenChange: vi.fn(),
    };
    const { rerender } = render(
      <LoopNodeControlDialog {...props} request={{ verb: "pause", node }} />
    );

    await user.click(screen.getByTestId("loop-node-pause-mode-cancel"));
    expect(screen.getByTestId("loop-node-pause-mode-cancel")).toHaveAttribute(
      "aria-checked",
      "true"
    );

    rerender(<LoopNodeControlDialog {...props} request={null} />);
    rerender(<LoopNodeControlDialog {...props} request={{ verb: "pause", node }} />);

    expect(screen.getByTestId("loop-node-pause-mode-drain")).toHaveAttribute(
      "aria-checked",
      "true"
    );
    expect(screen.getByTestId("loop-node-pause-mode-cancel")).toHaveAttribute(
      "aria-checked",
      "false"
    );

    const waitingNode: LoopNodeLifecycle = {
      ...node,
      state: "waiting",
      paused: false,
      waits: [
        {
          nodeId: node.nodeId,
          generation: node.generation,
          itemIndex: 7,
          kind: "event",
          claimState: "waiting",
          escalationCursor: 0,
          admissionFailures: 0,
          ageSeconds: 10,
          createdAt: "2026-08-03T14:00:00Z",
          expect: undefined,
        },
      ],
    };
    rerender(<LoopNodeControlDialog {...props} request={null} />);
    rerender(
      <LoopNodeControlDialog {...props} request={{ verb: "resume-wait", node: waitingNode }} />
    );
    fireEvent.change(screen.getByTestId("loop-node-wait-payload"), {
      target: { value: '{"approved":true}' },
    });

    rerender(<LoopNodeControlDialog {...props} request={null} />);
    rerender(
      <LoopNodeControlDialog {...props} request={{ verb: "resume-wait", node: waitingNode }} />
    );

    expect(screen.getByTestId("loop-node-wait-payload")).toHaveValue("");
  });

  it("Should disable wait-resume confirm until the payload matches expect", () => {
    const waitingNode = loopNodeLifecycleFixture({
      state: "waiting",
      parked: true,
      waits: [
        {
          nodeId: "task_03",
          generation: 2,
          itemIndex: 0,
          kind: "event",
          claimState: "waiting",
          escalationCursor: 0,
          admissionFailures: 0,
          ageSeconds: 10,
          createdAt: "2026-08-03T14:00:00Z",
          expect: { type: "object", required: ["env"] },
        },
      ],
    });
    render(
      <LoopNodeControlDialog
        onConfirm={vi.fn()}
        onOpenChange={vi.fn()}
        request={{ verb: "resume-wait", node: waitingNode }}
      />
    );
    expect(screen.getByTestId("loop-node-wait-expect")).toHaveTextContent("env");
    expect(screen.getByRole("button", { name: "Resume lane" })).toBeDisabled();
    fireEvent.change(screen.getByTestId("loop-node-wait-payload"), {
      target: { value: '{"environment":"staging"}' },
    });
    expect(screen.getByTestId("loop-node-wait-invalid")).toHaveTextContent("Missing key env");
    expect(screen.getByRole("button", { name: "Resume lane" })).toBeDisabled();
    fireEvent.change(screen.getByTestId("loop-node-wait-payload"), {
      target: { value: '{"env":"staging"}' },
    });
    expect(screen.queryByTestId("loop-node-wait-invalid")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Resume lane" })).toBeEnabled();
  });

  it("Should render a deterministic answer as information, not a transport error", () => {
    render(
      <LoopNodeControlDialog
        answer={{
          allowedTransitions: ["pause"],
          detail: "task_03 isn't paused — it's running.",
          micro: "node_not_paused · state running",
          title: "Nothing to resume",
          tone: "info",
        }}
        onConfirm={vi.fn()}
        onOpenChange={vi.fn()}
        request={{ verb: "resume", node }}
      />
    );
    const answer = screen.getByTestId("loop-control-answer");
    expect(answer).toHaveAttribute("data-variant", "info");
    expect(answer).toHaveTextContent("Nothing to resume");
    expect(answer).toHaveTextContent("node_not_paused · state running");
  });
});

describe("LoopNodeControlMenu", () => {
  const quarantined = loopNodeLifecycleFixture({
    state: "quarantined",
    parked: true,
    quarantined: true,
    quarantinedAt: "2026-08-03T14:52:00Z",
    revision: 3,
    attempt: 4,
    failureClass: "payload_declared",
    disposition: "quarantined",
    outputStatus: "failed",
  });

  it("Should render only the quarantined verb set and report the chosen verb", async () => {
    const user = userEvent.setup();
    const onVerb = vi.fn();
    render(<LoopNodeControlMenu node={quarantined} onVerb={onVerb} runStatus="running" />);
    const trigger = screen.getByTestId("loop-node-menu-trigger-task_03");
    fireEvent.pointerDown(trigger, { button: 0, pointerType: "mouse" });
    await user.click(trigger);
    expect(LOOP_NODE_VERB_PRESENTATION.cancel.label).toBe("Cancel…");
    expect(await screen.findByTestId("loop-node-verb-requeue")).toBeInTheDocument();
    expect(screen.getByTestId("loop-node-verb-open-quarantine")).toBeInTheDocument();
    // Resume is never offered for quarantine — requeue is the recovery verb.
    expect(screen.queryByTestId("loop-node-verb-resume")).not.toBeInTheDocument();
    expect(screen.queryByTestId("loop-node-verb-pause")).not.toBeInTheDocument();
    await user.click(screen.getByTestId("loop-node-verb-requeue"));
    expect(onVerb).toHaveBeenCalledWith("requeue", quarantined);
  });

  it("Should render no trigger when the run is terminal", () => {
    render(<LoopNodeControlMenu node={quarantined} onVerb={vi.fn()} runStatus="canceled" />);
    expect(screen.queryByTestId("loop-node-menu-trigger-task_03")).not.toBeInTheDocument();
  });
});

describe("LoopNodeRowActions", () => {
  it("Should use the requeue icon for the promoted quarantine action", () => {
    render(
      <LoopNodeRowActions
        node={loopNodeLifecycleFixture({
          state: "quarantined",
          parked: true,
          quarantined: true,
        })}
        onVerb={vi.fn()}
        runStatus="running"
      />
    );
    const button = screen.getByTestId("loop-node-primary-requeue-task_03");
    expect(button.querySelector(".lucide-redo-2")).toBeInTheDocument();
    expect(button.querySelector(".lucide-play")).not.toBeInTheDocument();
  });

  it("Should promote resume-wait when the open wait needs a decision", () => {
    render(
      <LoopNodeRowActions
        node={loopNodeLifecycleFixture({
          state: "waiting",
          parked: true,
          waits: [
            {
              nodeId: "task_03",
              generation: 2,
              itemIndex: 0,
              kind: "approval_escalation",
              claimState: "intervention_required",
              escalationCursor: 1,
              admissionFailures: 0,
              ageSeconds: 120,
              createdAt: "2026-08-03T14:00:00Z",
              expect: undefined,
            },
          ],
        })}
        onVerb={vi.fn()}
        runStatus="running"
      />
    );
    expect(screen.getByTestId("loop-node-primary-resume-wait-task_03")).toHaveTextContent(
      "Resume with payload…"
    );
  });
});

describe("loopNodeVerbConfirmCopy", () => {
  it("Should not invent a quarantine episode when no entry was returned", () => {
    const copy = loopNodeVerbConfirmCopy(
      "requeue",
      loopNodeLifecycleFixture({ state: "quarantined", quarantined: true })
    );
    expect(copy?.body).not.toContain("episode");
  });

  it("Should reflect the selected pause mode in the micro trail", () => {
    const node = loopNodeLifecycleFixture();
    expect(loopNodeVerbConfirmCopy("pause", node, { pauseMode: "drain" })?.micro).toBe(
      "mode: drain"
    );
    expect(loopNodeVerbConfirmCopy("pause", node, { pauseMode: "cancel" })?.micro).toBe(
      "mode: cancel"
    );
  });
});

describe("loopNodeStateStrip", () => {
  it("Should append the attention clause and last evidence without a raw ISO", () => {
    const strip = loopNodeStateStrip(
      loopNodeLifecycleFixture({
        attentionFlag: "silence",
        attentionReason: "silent for 31m",
        lastEvidenceAt: "2026-08-03T14:21:00Z",
        outputStatus: "running",
      })
    );
    expect(strip).toContain("flagged: silent for 31m");
    expect(strip).toContain("last evidence");
    expect(strip).not.toContain("2026-08-03T14:21:00Z");
  });

  it("Should surface nextAttemptAt on a retrying strip", () => {
    const strip = loopNodeStateStrip(
      loopNodeLifecycleFixture({
        attempt: 2,
        nextAttemptAt: "2099-01-01T00:00:00Z",
        state: "retrying",
      })
    );
    expect(strip).toContain("is retrying");
    expect(strip).toContain("attempt 2");
    expect(strip).toContain("next");
    expect(strip).not.toContain("2099-01-01T00:00:00Z");
  });
});

describe("loopRunStateStrip", () => {
  it("Should omit zero lane counts", () => {
    expect(
      loopRunStateStrip({
        generation: 2,
        inFlightCount: 0,
        runId: "r-7c4e19",
        status: "running",
        waitingOnYouCount: 0,
      })
    ).toBe("r-7c4e19 is running · generation 2");
  });
});

describe("checkLoopWaitPayload", () => {
  it("Should name the first missing required key", () => {
    const check = checkLoopWaitPayload("{}", { type: "object", required: ["env"] });
    expect(check.ok).toBe(false);
    expect(check.error).toBe("Missing key env.");
  });

  it("Should treat a sample with a top-level type as a sample, not a schema", () => {
    const expectBody = { type: "deploy", env: "staging" };
    expect(loopWaitExpectRequiredKeys(expectBody)).toEqual(["type", "env"]);
    expect(checkLoopWaitPayload("{}", expectBody).ok).toBe(false);
    expect(checkLoopWaitPayload('{"type":"deploy","env":"staging"}', expectBody).ok).toBe(true);
  });

  it("Should honor a JSON Schema required list when type is object", () => {
    expect(loopWaitExpectRequiredKeys({ type: "object", required: ["env", "region"] })).toEqual([
      "env",
      "region",
    ]);
  });

  it("Should require no keys for a schema that only declares properties", () => {
    expect(
      loopWaitExpectRequiredKeys({
        type: "object",
        properties: { env: { type: "string" } },
      })
    ).toEqual([]);
  });
});

describe("LoopQuarantineSheet", () => {
  const entry = {
    nodeId: "task_03",
    inputRef: "loop-run:r-1:node:task_03:input",
    target: "compozy__fetch",
    episodes: [
      {
        generation: 2,
        quarantinedAt: "2026-08-03T14:52:00Z",
        attempts: [{ attempt: 1, cause: "transport failed", disposition: "quarantined" }],
      },
    ],
    requeues: [],
    truncated: false,
    attemptCount: 1,
    hint: "Repair the target, then requeue.",
    quarantinedAt: "2026-08-03T14:52:00Z",
  };

  it("Should offer requeue only while refreshed truth still reports quarantine", async () => {
    const user = userEvent.setup();
    const onVerb = vi.fn();
    const quarantined = loopNodeLifecycleFixture({
      state: "quarantined",
      parked: true,
      quarantined: true,
      quarantineEntry: entry,
    });
    const props = {
      onOpenChange: vi.fn(),
      onVerb,
      open: true,
      runId: "r-1",
    };
    const { rerender } = render(
      <LoopQuarantineSheet {...props} isRequeuePending node={quarantined} />
    );
    expect(screen.getByTestId("loop-quarantine-requeue")).toBeDisabled();
    expect(screen.getByTestId("loop-quarantine-cancel")).toHaveTextContent("Cancel…");

    rerender(<LoopQuarantineSheet {...props} node={quarantined} />);
    await user.click(screen.getByTestId("loop-quarantine-requeue"));
    expect(onVerb).toHaveBeenCalledWith("requeue", quarantined);
    await user.click(screen.getByTestId("loop-quarantine-cancel"));
    expect(onVerb).toHaveBeenCalledWith("cancel", quarantined);

    rerender(
      <LoopQuarantineSheet
        {...props}
        node={loopNodeLifecycleFixture({ quarantineEntry: entry, quarantined: false })}
      />
    );
    expect(screen.queryByTestId("loop-quarantine-requeue")).not.toBeInTheDocument();
    expect(screen.queryByTestId("loop-quarantine-cancel")).not.toBeInTheDocument();
  });

  it("Should pair a retained episode with the requeue from the same generation", () => {
    const rows = quarantineChainRows({
      ...entry,
      episodes: [
        { generation: 7, attempts: [{ attempt: 1 }] },
        { generation: 9, attempts: [{ attempt: 1 }] },
      ],
      requeues: [
        { actorKind: "user", actorId: "stale", generation: 5 },
        { actorKind: "user", actorId: "correct", generation: 9 },
      ],
      truncated: true,
      attemptCount: 2,
    });
    expect(rows[0].openedBy).toBeUndefined();
    expect(rows[1].openedBy?.actorId).toBe("correct");
  });

  it("Should carry the requeue reason on the episode boundary", async () => {
    render(
      <LoopQuarantineSheet
        node={loopNodeLifecycleFixture({
          quarantineEntry: {
            ...entry,
            episodes: [
              { generation: 7, attempts: [{ attempt: 1, cause: "first fail" }] },
              { generation: 9, attempts: [{ attempt: 1, cause: "second fail" }] },
            ],
            requeues: [
              {
                actorKind: "user",
                actorId: "operator",
                generation: 9,
                reason: "rotated the token",
              },
            ],
            attemptCount: 2,
          },
          quarantined: true,
          state: "quarantined",
        })}
        onOpenChange={vi.fn()}
        onVerb={vi.fn()}
        open
        runId="r-1"
      />
    );
    expect(await screen.findByTestId("loop-quarantine-episode-1")).toHaveTextContent(
      "rotated the token"
    );
    await waitFor(() => {
      expect(screen.getByTestId("loop-quarantine-facts")).toBeInTheDocument();
    });
  });
});

describe("LoopRunWaitsRail", () => {
  it("Should count every open wait truthfully and identify its sole node", () => {
    render(
      <LoopRunWaitsRail
        runId="r-7c4e19"
        nodes={[
          loopNodeLifecycleFixture({
            nodeId: "await_deploy",
            state: "waiting",
            parked: true,
            waits: [
              {
                nodeId: "await_deploy",
                generation: 2,
                itemIndex: 0,
                kind: "timer",
                claimState: "waiting",
                escalationCursor: 0,
                admissionFailures: 0,
                ageSeconds: 60,
                createdAt: "2026-08-03T14:00:00Z",
                expect: undefined,
              },
            ],
          }),
        ]}
      />
    );
    expect(screen.getByTestId("loop-run-waits-open-waits")).toHaveTextContent("1· await_deploy");
    expect(screen.queryByText("Waiting on you")).not.toBeInTheDocument();
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
      JSON.stringify({ name: "implement-tasks" })
    );
    expect(screen.getByTestId("loop-run-about-version")).toHaveTextContent("v4 · pinned");
    expect(screen.getByTestId("loop-run-input-pr")).toHaveTextContent("128");
    expect(screen.getByTestId("loop-run-about-started-by")).toHaveTextContent("A webhook");
    expect(screen.getByTestId("loop-run-about-workspace")).toHaveTextContent("Home");
    expect(screen.getByTestId("loop-run-about-id")).toHaveTextContent(run().id);
  });
});

describe("LoopRunResolvedRuntimes", () => {
  it("Should expose each durable runtime field with the daemon-reported provenance", () => {
    render(
      <LoopRunResolvedRuntimes
        generations={[
          {
            generation: 2,
            parent_generation: 1,
            origin: "gate_revise",
            route_causes: [],
            verdicts: [],
            outputs: [
              {
                node_id: "execute_task",
                item_index: 2,
                status: "running",
                task_run_id: "tr_204",
                resolved_runtime: {
                  provider: "openai",
                  model: "gpt-5.4",
                  reasoning: "high",
                  source: { provider: "run", model: "frontmatter", reasoning: "config" },
                },
              },
            ],
          },
        ]}
      />
    );

    const runtime = screen.getByTestId("loop-run-resolved-runtime");
    expect(runtime).toHaveTextContent("Generation 2 · execute_task · item 3");
    expect(runtime).toHaveTextContent("tr_204");
    expect(runtime).toHaveTextContent("openai");
    expect(runtime).toHaveTextContent("per-run rule");
    expect(runtime).toHaveTextContent("gpt-5.4");
    expect(runtime).toHaveTextContent("task frontmatter");
    expect(runtime).toHaveTextContent("high");
    expect(runtime).toHaveTextContent("config rule");
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
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

  it("Should add Started by and Round N of M as dot-separated segments", () => {
    render(
      <LoopRunSubhead
        run={run({ status: "running", generation: 2, iteration_cap: 5 })}
        subject={null}
        hasWatchSource={false}
        elapsedLabel="22m 14s"
        startedBy="The CLI"
      />
    );
    expect(screen.getByTestId("loop-run-started-by")).toHaveTextContent("Started by The CLI");
    expect(screen.getByTestId("loop-run-round")).toHaveTextContent("Round 2 of 5");
  });

  it("Should omit of M when the iteration cap is off", () => {
    render(
      <LoopRunSubhead
        run={run({ status: "running", generation: 2, iteration_cap: 0 })}
        subject={null}
        hasWatchSource={false}
        elapsedLabel="22m 14s"
        startedBy="hand"
      />
    );
    expect(screen.getByTestId("loop-run-started-by")).toHaveTextContent("Started by hand");
    expect(screen.getByTestId("loop-run-round")).toHaveTextContent("Round 2");
    expect(screen.getByTestId("loop-run-round")).not.toHaveTextContent("of");
  });
});
