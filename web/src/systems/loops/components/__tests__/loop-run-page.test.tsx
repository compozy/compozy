import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createRef } from "react";
import { describe, expect, it, vi } from "vitest";

import { loopNodeLifecycleFixture } from "../../testing/loop-node-lifecycle-fixture";
import type { LoopRosterTableModel } from "../../lib/loop-run-roster-table";

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

const { makeBriefing, makeGeneration, makeRosterNode, makeTimelineEntry } =
  await import("../stories/loop-run-read-builders");
const { buildStoryBeats } = await import("../../lib/loop-run-story-beats");
const { buildRunDag } = await import("../../lib/loop-run-dag-view");
const { LoopRunDag } = await import("../run-page/inspect/loop-run-dag");
const { LoopNodeStateChip } = await import("../run-page/loop-node-state-chip");
const { LOOP_ROSTER_STATES, loopRosterStateChip } = await import("../../lib/loop-run-state-copy");
const { LoopRunStory } = await import("../run-page/loop-run-story");
const { LoopNodeRoster } = await import("../run-page/inspect/loop-node-roster");
const { LoopRunArtifactList } = await import("../run-page/loop-run-artifact-list");
const registerFixtures = await import("../stories/loop-run-register-fixtures");
const graphEngFixtures = await import("../stories/loop-run-graph-eng-fixtures");
const { registerPartialOutputsScenario } = registerFixtures;
const { buildScenarioProps } = await import("../stories/loop-run-scenario-props");
const { LoopRunNeedsYouCard } = await import("../run-page/loop-run-needs-you-card");
const { LOOP_NEEDS_YOU_ANCHOR_ID } = await import("../run-page/loop-run-briefing-constants");
const { LoopRunBriefing } = await import("../run-page/loop-run-briefing");
const { buildBriefingView } = await import("../../lib/loop-run-briefing-view");
const { projectLoopRequest } = await import("../../lib/loop-request-model");
const { projectLoopRunPageView } = await import("../../lib/loop-run-page-view");
const { emptyLoopRunLiveState } = await import("../../lib/loop-events");
const { answeredAskRequest, pendingEntityAskRequest, pendingReviewRequest } =
  await import("../../mocks/fixture-graph-eng-requests");
const { LoopRunControls } = await import("../run-page/loop-run-controls");
const { LoopNodeControlMenu } = await import("../run-page/loop-node-control-menu");
const { LoopNodeRowActions } = await import("../run-page/loop-node-row-actions");
const { LoopRunOverflowMenu } = await import("../run-page/loop-run-overflow-menu");
const { LoopRunControlDialog } = await import("../run-page/loop-run-control-dialog");
const { LoopNodeControlDialog } = await import("../run-page/loop-node-control-dialog");
const { LoopNodeAmendDialog } = await import("../run-page/loop-node-amend-dialog");
const { LoopQuarantineSheet } = await import("../run-page/loop-quarantine-sheet");
const { LOOP_NODE_VERB_PRESENTATION } = await import("../../lib/loop-node-controls");
const { quarantineChainRows } = await import("../../lib/loop-quarantine-entry");
const { loopNodeStateStrip, loopNodeVerbConfirmCopy, loopRunStateStrip } =
  await import("../../lib/loop-node-verb-copy");
const { checkLoopWaitPayload, loopWaitExpectRequiredKeys } =
  await import("../../lib/loop-node-wait-payload");
type LoopNodeLifecycle = import("../../lib/loop-node-lifecycle").LoopNodeLifecycle;
const { LoopRunUsageRail } = await import("../run-page/loop-run-usage-rail");
const { LoopRunStepsProgress } = await import("../run-page/loop-run-steps-progress");
const { LoopRunAboutRail } = await import("../run-page/loop-run-about-rail");
const { LoopRunRegisters } = await import("../run-page/loop-run-registers");
const { projectLoopRunRegisters } = await import("../../lib/loop-run-registers-view");
const { buildRunUsage } = await import("../../lib/loop-run-usage");
const { loopRunDetailByRunId } = await import("../../mocks/fixtures");
type LoopRunRecord = import("../../types").LoopRunRecord;

const detail = loopRunDetailByRunId.get("looprun_running")!;

function run(overrides: Partial<LoopRunRecord> = {}): LoopRunRecord {
  return { ...detail.run, ...overrides };
}

describe("LoopNodeStateChip", () => {
  it.each(LOOP_ROSTER_STATES)(
    "Should expose the visible %s state as its accessible name",
    state => {
      const chip = loopRosterStateChip(state);
      render(<LoopNodeStateChip chip={chip} />);

      const stateChip = screen.getByTestId(`loop-state-chip-${state}`);
      expect(stateChip).toHaveAccessibleName(chip.label);
      expect(stateChip).toHaveTextContent(chip.label);
    }
  );
});

describe("LoopRunNeedsYouCard", () => {
  it.each(["fresh load", "stale approval frame"])(
    "Should display the current durable human prompt on %s",
    source => {
      const currentRun = run({
        status: "needs-approval",
        generation: 2,
        active_gate_id: "release_review",
      });
      const live = emptyLoopRunLiveState();
      if (source === "stale approval frame") {
        live.needsApproval = {
          gateId: "release_review",
          generation: 1,
          title: "Old review",
          prompt: "Approve the previous release?",
          facts: [],
        };
      }
      const view = projectLoopRunPageView({
        run: currentRun,
        generations: [
          makeGeneration(1, {
            verdicts: [
              {
                gate_id: "release_review",
                item_index: 0,
                outcome: "awaiting_approval",
                blocking_issues: [],
                criteria: [
                  {
                    id: "reviewer",
                    type: "human",
                    outcome: "awaiting_approval",
                    passed: false,
                    prompt: "Approve the previous release?",
                  },
                ],
              },
            ],
          }),
          makeGeneration(2, {
            verdicts: [
              {
                gate_id: "release_review",
                item_index: 0,
                outcome: "awaiting_approval",
                blocking_issues: [],
                criteria: [
                  {
                    id: "reviewer",
                    type: "human",
                    outcome: "awaiting_approval",
                    passed: false,
                    prompt: "Approve onboarding/source-index.md for the Studio handoff?",
                  },
                ],
              },
            ],
          }),
        ],
        definition: undefined,
        live,
        nowMs: Date.parse("2026-10-05T12:00:00Z"),
      });
      render(
        <LoopRunNeedsYouCard
          run={currentRun}
          request={view.approvalRequest}
          fallbackFacts={view.approvalFallbackFacts}
          onDecision={vi.fn()}
        />
      );

      expect(
        screen.getByText("Approve onboarding/source-index.md for the Studio handoff?")
      ).toBeInTheDocument();
      expect(screen.queryByText("Approve the previous release?")).not.toBeInTheDocument();
      expect(screen.getByTestId("loop-run-needs-approval-origin")).toHaveTextContent(
        "release review · round 2"
      );
    }
  );

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

    expect(screen.getAllByTestId("loop-request-card")).toHaveLength(1);
    expect(screen.getByTestId("loop-request-progress")).toHaveTextContent("Question 2 of 2");

    fireEvent.click(screen.getByTestId("loop-request-details"));
    expect(screen.getByRole("alert")).toHaveTextContent("Context is temporarily unavailable");
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(onRequestFullContext).toHaveBeenCalledWith(3, pendingReviewRequest.node_id, 0);

    fireEvent.click(screen.getByTestId("loop-request-prev"));
    expect(screen.getByTestId("loop-request-progress")).toHaveTextContent("Question 1 of 2");
    fireEvent.click(screen.getByTestId("loop-request-details"));
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("Should show settled requests as recorded outcomes below the questions, never a form", () => {
    const nowMs = Date.parse("2026-08-17T10:00:00Z");
    render(
      <LoopRunNeedsYouCard
        fallbackFacts={[]}
        onDecision={vi.fn()}
        request={null}
        requests={[
          projectLoopRequest(pendingReviewRequest, { nowMs, runStatus: "running" }),
          projectLoopRequest(answeredAskRequest, { nowMs, runStatus: "running" }),
        ]}
        run={run({ status: "running" })}
        showApproval={false}
      />
    );

    expect(screen.getAllByTestId("loop-request-card")).toHaveLength(1);
    expect(screen.queryByTestId("loop-request-progress")).not.toBeInTheDocument();
    const settled = screen.getByTestId("loop-request-resolution");
    expect(settled).toHaveTextContent("operator pedro answered with respond.");
    expect(settled.querySelector("form")).toBeNull();
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

  it("Should render an entity-annotated answer with the shared picker", () => {
    render(
      <LoopRunNeedsYouCard
        fallbackFacts={[]}
        onDecision={vi.fn()}
        request={null}
        requests={[
          projectLoopRequest(pendingEntityAskRequest, {
            nowMs: Date.parse("2026-08-17T10:00:00Z"),
            runStatus: "running",
          }),
        ]}
        run={run({ status: "running" })}
        showApproval={false}
      />
    );

    const picker = screen.getByTestId("loop-request-field-assignment.reviewer");
    expect(picker.tagName).toBe("BUTTON");
    expect(screen.getByText("Reviewer")).toBeInTheDocument();
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
    // The streamed gate is what the card names, and it names it in words. This
    // assertion used to read `on_exceeded: halt` — it was pinning a wire enum
    // into the default register rather than checking the gate travelled.
    expect(screen.getByTestId("loop-run-needs-approval-origin")).toHaveTextContent("budget");
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

  // task_05 requirement 1 bans machine ids and raw enums from the default
  // register, and this line printed two of them — `needs_approval · <gate id>`,
  // and `on_exceeded: <enum>` on top of that when the gate was the budget.
  it("Should name the asking gate in words instead of the wire enum", () => {
    render(
      <LoopRunNeedsYouCard
        run={run({ status: "needs-approval", active_gate_id: "finalize_round", generation: 2 })}
        request={null}
        fallbackFacts={[]}
        onDecision={vi.fn()}
      />
    );

    const origin = screen.getByTestId("loop-run-needs-approval-origin");
    expect(origin).toHaveTextContent("finalize round · round 2");
    const card = screen.getByTestId("loop-run-needs-approval");
    expect(card).not.toHaveTextContent("needs_approval");
    expect(card).not.toHaveTextContent("finalize_round");
  });

  it("Should not restate the budget policy the usage rail already spells out", () => {
    render(
      <LoopRunNeedsYouCard
        run={run({
          status: "needs-approval",
          active_gate_id: "budget",
          budget_on_exceeded: "escalate",
          generation: 2,
        })}
        request={null}
        fallbackFacts={[]}
        onDecision={vi.fn()}
      />
    );

    const card = screen.getByTestId("loop-run-needs-approval");
    expect(card).not.toHaveTextContent("on_exceeded");
    expect(card).not.toHaveTextContent("escalate");
    expect(screen.getByTestId("loop-run-needs-approval-origin")).toHaveTextContent(
      "budget · round 2"
    );
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
    // A live run can still take the verb, so the row points at it.
    expect(screen.getByTestId("loop-run-needs-quarantine-detail-fix_batch-g2")).toHaveTextContent(
      "Retry it once the problem is fixed."
    );
    fireEvent.click(screen.getByTestId("loop-run-needs-open-quarantine-fix_batch-g2"));
    expect(onOpenQuarantine).toHaveBeenCalledWith("fix_batch");
  });

  it("Should stop instructing a requeue once the run has ended", () => {
    const onOpenQuarantine = vi.fn();
    render(
      <LoopRunNeedsYouCard
        run={run({ status: "failed" })}
        request={null}
        fallbackFacts={[]}
        showApproval={false}
        quarantinedNodes={[
          loopNodeLifecycleFixture({
            nodeId: "orchestrate",
            label: "orchestrate",
            state: "quarantined",
            parked: true,
            quarantined: true,
            quarantineEntry: {
              nodeId: "orchestrate",
              inputRef: "loop-run:r-1:node:orchestrate:input",
              target: "compozy__goal",
              episodes: [],
              requeues: [],
              truncated: false,
              attemptCount: 2,
              hint: "",
            },
          }),
        ]}
        onOpenQuarantine={onOpenQuarantine}
        onDecision={vi.fn()}
      />
    );
    // The daemon rejects requeue on a terminal run; the row keeps the reason
    // and the entry, and no longer asks for a verb nobody can take.
    const detail = screen.getByTestId("loop-run-needs-quarantine-detail-orchestrate-g2");
    expect(detail).toHaveTextContent("Set aside after 2 tries. This run has ended.");
    expect(detail).not.toHaveTextContent(/requeue/i);
    fireEvent.click(screen.getByTestId("loop-run-needs-open-quarantine-orchestrate-g2"));
    expect(onOpenQuarantine).toHaveBeenCalledWith("orchestrate");
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

describe("LoopNodeAmendDialog typed fields", () => {
  it("Should render an entity annotation with the shared picker and preserve its value", () => {
    render(
      <LoopNodeAmendDialog
        node={loopNodeLifecycleFixture({ nodeId: "review", outputStatus: "succeeded" })}
        onConfirm={vi.fn()}
        onOpenChange={vi.fn()}
        open
        originalOutput={{ reviewer: "reviewer" }}
        outputSchema={{
          type: "object",
          required: ["reviewer"],
          properties: {
            reviewer: { type: "string", "x-compozy-kind": "agent" },
          },
        }}
      />
    );

    const picker = screen.getByTestId("loop-amend-field-reviewer");
    expect(picker.tagName).toBe("BUTTON");
    expect(picker).toHaveTextContent("reviewer");
    expect(picker).toHaveTextContent("Not available");
  });
});

describe("LoopRunControls", () => {
  it("Should forward native div props and ref while preserving control layout classes", () => {
    const ref = createRef<HTMLDivElement>();
    render(
      <LoopRunControls
        ref={ref}
        aria-label="Run actions"
        className="custom-controls"
        status="running"
        onPause={vi.fn()}
        onResume={vi.fn()}
        onCancel={vi.fn()}
      />
    );

    const controls = screen.getByLabelText("Run actions");
    expect(ref.current).toBe(controls);
    expect(controls).toHaveClass("flex", "items-center", "gap-2", "custom-controls");
  });

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

  // WT-004 (run half): cancellation is the only destructive run control, and a
  // pause that has been requested but not yet landed offers no second pause.
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
    expect(screen.getByTestId("loop-run-cancel")).toHaveTextContent("Cancel run");
  });
});

// WT-004 (run confirmation): cancellation is destructive and irreversible for
// the run, so it may not commit without restating the state it acts on.
describe("LoopRunControlDialog", () => {
  it("Should restate the run's current identity and status before canceling", () => {
    const onConfirm = vi.fn();
    render(
      <LoopRunControlDialog
        generation={2}
        onConfirm={onConfirm}
        onOpenChange={vi.fn()}
        runId="r-7c4e19"
        status="running"
        verb="cancel"
      />
    );
    const dialog = screen.getByTestId("loop-run-control-dialog");
    expect(dialog).toHaveTextContent("Cancel run r-7c4e19?");
    // The strip is the guard against acting on a stale screen.
    expect(dialog).toHaveTextContent("r-7c4e19 is running · round 2");
    expect(dialog).not.toHaveTextContent("in flight");
    expect(dialog).not.toHaveTextContent("waiting on you");
    expect(dialog).toHaveTextContent("Active sessions are stopped automatically");
    expect(dialog).toHaveTextContent("cause operator_cancel");
    fireEvent.click(screen.getByRole("button", { name: "Cancel run" }));
    expect(onConfirm).toHaveBeenCalledWith("cancel");
  });

  it("Should describe cancel as an immediate stop with automatic session cleanup", () => {
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
    expect(dialog).toHaveTextContent("r-7c4e19 is watching · round 4");
    expect(dialog).toHaveTextContent("Active sessions are stopped automatically");
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
      "r-7c4e19 is running · 2 lanes in flight · 1 waiting on you · round 2 · 22m 14s"
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

// WT-004: the overflow keeps navigation only; cancellation is owned by the
// first-class destructive run control.
describe("LoopRunOverflowMenu", () => {
  it("Should keep navigation actions only", async () => {
    const user = userEvent.setup();
    render(<LoopRunOverflowMenu loopName="review-and-fix" />);
    const trigger = screen.getByTestId("loop-run-more");
    fireEvent.pointerDown(trigger, { button: 0, pointerType: "mouse" });
    await user.click(trigger);
    expect(await screen.findByTestId("loop-run-view-definition")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-run-inspect")).not.toBeInTheDocument();
    expect(screen.getAllByRole("menuitem")).toHaveLength(2);
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
      "Resume with data…"
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
    ).toBe("r-7c4e19 is running · round 2");
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

  it("Should keep the entry readable but withdraw the verbs once the run has ended", async () => {
    const quarantined = loopNodeLifecycleFixture({
      state: "quarantined",
      parked: true,
      quarantined: true,
      quarantineEntry: entry,
    });
    const props = { onOpenChange: vi.fn(), onVerb: vi.fn(), open: true, runId: "r-1" };
    const { rerender } = await act(async () =>
      render(<LoopQuarantineSheet {...props} node={quarantined} />)
    );
    // Live: the verbs are on offer and the foot says the run is still going.
    expect(screen.getByTestId("loop-quarantine-requeue")).toBeInTheDocument();
    expect(screen.getByTestId("loop-quarantine-foot")).toHaveTextContent("The run keeps working");

    await act(async () => rerender(<LoopQuarantineSheet {...props} node={quarantined} runEnded />));
    // Ended: the daemon rejects requeue and cancel, so neither is offered, while
    // the hint, the facts and the attempt chain stay exactly as retained.
    expect(screen.queryByTestId("loop-quarantine-requeue")).not.toBeInTheDocument();
    expect(screen.queryByTestId("loop-quarantine-cancel")).not.toBeInTheDocument();
    expect(screen.getByTestId("loop-quarantine-foot")).toHaveTextContent("This run has ended.");
    expect(screen.getByTestId("loop-quarantine-hint")).toHaveTextContent(entry.hint);
    expect(screen.getByTestId("loop-quarantine-chain")).toHaveTextContent("transport failed");
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
  it("Should reveal the loop, pinned version, inputs, and run id from About", async () => {
    render(
      <LoopRunAboutRail
        run={run()}
        versionLabel="v4 · pinned"
        inputRows={[{ key: "pr", label: "PR", value: "128", isAgent: false }]}
        startedBy="A webhook"
        workspaceLabel="Home"
      />
    );
    expect(screen.queryByTestId("loop-run-about-id")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "About this run" }));
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

  it("Should expose the last wake and open the daemon-selected best generation", async () => {
    const onOpenGeneration = vi.fn();
    render(
      <LoopRunAboutRail
        run={run({ best_generation: 1, best_score: 0.7 })}
        inputRows={[]}
        lastWakeAt="2026-08-19T18:44:00Z"
        onOpenGeneration={onOpenGeneration}
        startedBy="An API call"
        workspaceLabel="Home"
      />
    );

    // Operational status and navigation read with About still closed.
    expect(screen.queryByTestId("loop-run-about-id")).not.toBeInTheDocument();
    expect(screen.getByTestId("loop-run-about-last-woke")).toHaveTextContent("Last woke");
    const best = screen.getByRole("link", { name: "Best result · Round 1 · 0.70" });
    expect(best).toHaveAttribute("href", "#loop-generation-1");
    await userEvent.click(best);
    expect(onOpenGeneration).toHaveBeenCalledWith(1);

    await userEvent.click(screen.getByRole("button", { name: "About this run" }));
    expect(screen.getByTestId("loop-run-about-id")).toHaveTextContent(run().id);
  });
});

// The briefing strip points at the decision; it never carries it. That pointer
// has to actually arrive somewhere, for a keyboard user as much as a mouse one —
// an action that only looks like an action is worse than no action at all.
describe("LoopRunBriefing needs-you action", () => {
  function needsYouBriefing() {
    return buildBriefingView(
      makeBriefing({
        run_id: "looprun-1",
        status: "needs-approval",
        tone: "needs_you",
        headline: "The gate has been waiting for your decision",
        blockers: [
          {
            kind: "approval",
            gate_id: "aplicar-correcoes",
            waiting_since: "2026-08-19T18:41:00Z",
            unblocker: "compozy loop approve looprun-1 --gate aplicar-correcoes",
          },
        ],
        artifacts: [],
        progress: { round: 1, steps_done: 4, steps_total: 6 },
        usage: { tokens: 82_400 },
      })
    );
  }

  it("Should move focus to the needs-you region rather than merely scrolling", async () => {
    render(
      <>
        <LoopRunBriefing briefing={needsYouBriefing()} outcome={null} />
        <section data-testid="needs-you-region" id={LOOP_NEEDS_YOU_ANCHOR_ID} tabIndex={-1}>
          decision card
        </section>
      </>
    );

    const region = screen.getByTestId("needs-you-region");
    region.scrollIntoView = vi.fn();

    const action = screen.getByTestId("loop-run-briefing-action");
    expect(action).toHaveTextContent("Review the request");
    await userEvent.click(action);

    // Focus, not just scroll: leaving a keyboard caret in the strip would strand
    // the very person the pointer exists for.
    expect(region).toHaveFocus();
    expect(region.scrollIntoView).toHaveBeenCalled();
  });

  it("Should never render a decision button of its own", () => {
    render(<LoopRunBriefing briefing={needsYouBriefing()} outcome={null} />);
    // One primary per decision, in one viewport. The card owns Approve/Reject.
    expect(screen.queryByRole("button", { name: /approve/i })).toBeNull();
    expect(screen.queryByRole("button", { name: /reject/i })).toBeNull();
  });
});

// Failure navigation requires an actual failed target, not just the briefing's
// danger tone: budget exhaustion and stalling can leave every executed step healthy.
describe("LoopRunBriefing failure action", () => {
  it.each(["exhausted", "stalled"] as const)(
    "Should omit failed-step navigation for %s without a failure reference",
    status => {
      render(
        <LoopRunBriefing
          briefing={buildBriefingView(makeBriefing({ status, tone: "failed", blockers: [] }))}
          onOpenInspect={vi.fn()}
          outcome={null}
        />
      );

      expect(screen.queryByTestId("loop-run-briefing-action")).not.toBeInTheDocument();
    }
  );

  it.each([{ node_id: "collect" }, { gate_id: "review" }])(
    "Should retain Inspect navigation for the concrete failure reference %o",
    async reference => {
      const onOpenInspect = vi.fn();
      render(
        <LoopRunBriefing
          briefing={buildBriefingView(
            makeBriefing({
              status: "failed",
              tone: "failed",
              blockers: [
                {
                  kind: "failure",
                  ...reference,
                  waiting_since: "2026-10-05T10:57:50Z",
                  unblocker: "",
                },
              ],
            })
          )}
          onOpenInspect={onOpenInspect}
          outcome={null}
        />
      );

      await userEvent.click(screen.getByTestId("loop-run-briefing-action"));
      expect(onOpenInspect).toHaveBeenCalledOnce();
    }
  );
});

// The lib knows how to degrade a pruned session; what this owns is whether the
// register ever hands it the truth to degrade on. Wiring is exactly where this
// went wrong before: the projection took a pruned set the page never passed, so
// the sentence existed and was unreachable.
// The page shows several kinds of silence, and they are not interchangeable: a
// run that did nothing, a read still arriving, and a read that failed all render
// an empty list. Only the last one means the history is unknown, and saying
// "nothing happened" for it is the page asserting something it cannot know.
describe("run-page reads that failed", () => {
  it("Should say the story could not be read instead of claiming an eventless run", () => {
    const { rerender } = render(
      <LoopRunStory
        beats={[]}
        paging={{
          hasOlder: false,
          isLoading: false,
          isLoadingOlder: false,
          onLoadOlder: () => undefined,
        }}
      />
    );
    expect(screen.getByTestId("loop-run-story-empty")).toHaveTextContent(
      "Nothing has happened in this run yet."
    );

    rerender(
      <LoopRunStory
        beats={[]}
        paging={{
          hasOlder: false,
          isError: true,
          isLoading: false,
          isLoadingOlder: false,
          onLoadOlder: () => undefined,
        }}
      />
    );
    const failed = screen.getByTestId("loop-run-story-empty");
    expect(failed).toHaveAttribute("data-state", "error");
    expect(failed).not.toHaveTextContent("Nothing has happened in this run yet.");
    expect(failed).toHaveTextContent("could not be loaded");
  });

  it("Should mark stale story beats instead of passing them off as current", () => {
    // Built through the production projection rather than hand-assembled: a beat
    // that the real timeline could not produce would prove nothing about it.
    const beats = buildStoryBeats([
      makeTimelineEntry(12, "node_succeeded", "step review succeeded"),
    ]);
    const paging = {
      hasOlder: false,
      isLoading: false,
      isLoadingOlder: false,
      onLoadOlder: () => undefined,
    };
    const { rerender } = render(<LoopRunStory beats={beats} isReconnecting paging={paging} />);
    // Beats survive a dropped stream; what changes is whether they are current.
    expect(screen.getByTestId("loop-run-beat-12")).toBeInTheDocument();
    expect(screen.getByTestId("loop-run-story-reconnecting")).toBeInTheDocument();

    // A failed read is the more specific fact and outranks a reconnect.
    rerender(<LoopRunStory beats={beats} isReconnecting paging={{ ...paging, isError: true }} />);
    expect(screen.getByTestId("loop-run-story-degraded")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-run-story-reconnecting")).toBeNull();
  });

  // The Events lane is the escape hatch onto raw activity. Until the page wires
  // the `view=all` read it is borrowing Story's notable projection, and a
  // filtered subset presented as the whole event log is the exact lie this lane
  // exists to prevent.
  it("Should admit when the Events lane is showing only the notable projection", async () => {
    const timeline = [makeTimelineEntry(12, "node_succeeded", "step review succeeded")];
    const registers = projectLoopRunRegisters({
      briefing: null,
      nodes: [],
      rollups: [],
      timeline,
      graph: null,
    });
    const props = {
      generations: [],
      graph: null,
      isLive: true,
      isReconnecting: false,
      nodeLifecycles: [],
      nodes: [],
      nowMs: Date.parse("2026-08-19T18:50:00Z"),
      onOpenChange: () => undefined,
      onSelectionChange: () => undefined,
      open: true,
      registers,
      rollups: [],
      runStatus: "running",
      selection: null,
    };
    const { rerender } = render(<LoopRunRegisters {...props} />);
    await userEvent.click(screen.getByTestId("loop-lane-events"));
    expect(screen.getByTestId("loop-run-events-notable-only")).toBeInTheDocument();

    // With the raw read wired the lane stops qualifying itself, and its backward
    // paging becomes reachable.
    rerender(
      <LoopRunRegisters
        {...props}
        events={{
          beats: registers.beats,
          hasOlder: true,
          isLoading: false,
          isError: false,
          isLoadingOlder: false,
          onLoadOlder: () => undefined,
        }}
      />
    );
    expect(screen.queryByTestId("loop-run-events-notable-only")).toBeNull();
    expect(screen.getByTestId("loop-run-events-load-older")).toBeInTheDocument();
  });

  // The node panel looks a row up by exact node, item and round. A card with no
  // roster row behind it has no item to name, and substituting 0 would open
  // either the wrong worker or nothing at all — the sentinel this model exists
  // to remove. The card has to be honestly unavailable instead.
  it("Should make a DAG card with no roster row inert rather than selecting item 0", async () => {
    const graph = {
      nodes: [
        {
          id: "implementar",
          nodeClass: "action" as const,
          kind: "run-agent",
          isGate: false,
          eventsCount: 0,
          routes: [],
          hasAskExpect: false,
        },
        {
          id: "saida",
          nodeClass: "action" as const,
          kind: "run-agent",
          isGate: false,
          eventsCount: 0,
          routes: [],
          hasAskExpect: false,
        },
      ],
      edges: [{ from: "implementar", to: "saida" }],
    };
    const onSelect = vi.fn();
    // `implementar` ran at a non-zero item; `saida` was never reached.
    const dag = buildRunDag({
      graph,
      nodes: [makeRosterNode("implementar", "running", { generation: 1, item_index: 3 })],
      rollups: [],
      round: 1,
    });
    render(<LoopRunDag dag={dag} onSelect={onSelect} selection={null} />);

    const unreached = screen.getByTestId("loop-dag-node-saida");
    expect(unreached).toBeDisabled();
    expect(unreached).toHaveAttribute("data-selectable", "false");
    // Not an unpressed toggle — it has no pressed state to report at all.
    expect(unreached).not.toHaveAttribute("aria-pressed");
    await userEvent.click(unreached);
    expect(onSelect).not.toHaveBeenCalled();

    // A card with a real server-owned item stays selectable and passes it through
    // exactly — never normalised to 0.
    const reached = screen.getByTestId("loop-dag-node-implementar");
    expect(reached).toBeEnabled();
    expect(reached).toHaveAttribute("aria-pressed", "false");
    await userEvent.click(reached);
    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(onSelect).toHaveBeenCalledWith({
      nodeId: "implementar",
      itemIndex: 3,
      generation: 1,
    });
  });

  it("Should not call a roster that has not answered a run without steps", () => {
    const empty: LoopRosterTableModel = { rows: [], rounds: [], reachedNothing: true };
    const { rerender } = render(
      <LoopNodeRoster
        onRoundChange={() => undefined}
        onSelect={() => undefined}
        round={null}
        roster={empty}
        selectedKey={null}
      />
    );
    expect(screen.getByTestId("loop-node-roster-empty")).toBeInTheDocument();

    rerender(
      <LoopNodeRoster
        onRoundChange={() => undefined}
        onSelect={() => undefined}
        read={{ isLoading: false, isError: true }}
        round={null}
        roster={empty}
        selectedKey={null}
      />
    );
    expect(screen.queryByTestId("loop-node-roster-empty")).toBeNull();
    expect(screen.getByTestId("loop-node-roster-error")).toHaveTextContent("could not be read");
  });
});

// Truthful UI: the page must not offer a control the runtime cannot honour, and
// must not paint a deliberate cancellation as a success.
describe("run-page affordances that have to be real", () => {
  it("Should reveal a produced artifact ref on demand without promising an Open link", async () => {
    render(
      <LoopRunArtifactList
        outcome={{
          outcome: null,
          producedNothing: false,
          artifacts: [
            {
              key: "post.md:0",
              name: "post.md",
              output: "write_artifacts",
              availability: "available",
              note: null,
              ref: "sha256:2f81c4a9",
              toneForNote: null,
            },
          ],
        }}
      />
    );
    // No API operation resolves a content digest, so an "Open" affordance here
    // would promise a destination the product does not have.
    expect(screen.queryByRole("link")).toBeNull();
    expect(screen.queryByText("Open")).toBeNull();
    expect(screen.queryByTestId("loop-run-artifact-ref-post.md")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Details for post.md" }));
    expect(screen.getByTestId("loop-run-artifact-ref-post.md")).toHaveTextContent(
      "sha256:2f81c4a9"
    );
  });

  it("Should bound the default result list while keeping partial and pruned evidence discoverable", async () => {
    const artifacts = Array.from({ length: 20 }, (_, index) => ({
      key: `result-${index}`,
      name: `Result ${index + 1}`,
      output: null,
      availability: "available" as const,
      note: null,
      ref: `{"summary":"Result ${index + 1}"}`,
      toneForNote: null,
    }));
    render(
      <LoopRunArtifactList
        outcome={{
          outcome: null,
          producedNothing: false,
          artifacts: [
            ...artifacts,
            {
              key: "partial",
              name: "Partial result",
              output: null,
              availability: "partial",
              note: "Partial",
              ref: "partial contents",
              toneForNote: "warning",
            },
            {
              key: "pruned",
              name: "Old report",
              output: null,
              availability: "pruned",
              note: "Content no longer stored",
              ref: null,
              toneForNote: "neutral",
            },
          ],
        }}
      />
    );
    expect(screen.getAllByRole("listitem")).toHaveLength(3);
    expect(screen.getByText("1 partial")).toBeVisible();
    expect(screen.getByText("1 no longer stored")).toBeVisible();
    const more = screen.getByRole("button", { name: "19 more outputs" });
    expect(more).toHaveAttribute("aria-expanded", "false");
    await userEvent.click(more);
    expect(screen.getAllByRole("listitem")).toHaveLength(22);
    expect(screen.getByTestId("loop-run-artifact-Old report")).toHaveTextContent(
      "Content no longer stored"
    );
    await userEvent.click(screen.getByRole("button", { name: "Details for Result 20" }));
    expect(screen.getByTestId("loop-run-artifact-ref-Result 20")).toHaveTextContent(
      artifacts[19]!.ref
    );
    await userEvent.click(more);
    await waitFor(() =>
      expect(screen.queryByTestId("loop-run-artifact-Result 20")).not.toBeInTheDocument()
    );
  });

  // US-008.AC-3, staged as Visual Contract row VC-08. The state used to render
  // from a manufactured briefing, so the contract capture photographed a plain
  // "Done" run with no partial signal at all — and passed. This walks the
  // fixture through the same projection the story does, so a scenario that stops
  // staging the partial read fails here instead of inside a capture run.
  it("Should label a partial output and its coverage in the default register", () => {
    const props = buildScenarioProps(registerPartialOutputsScenario());
    render(<LoopRunArtifactList outcome={props.registers.outcome!} />);

    const note = screen.getByTestId("loop-run-artifact-note-partial");
    expect(note).toHaveTextContent("Partial");
    // Tone never travels alone, and warning is the lock's tone for partial.
    expect(note).toHaveAttribute("data-tone", "warning");
    // Retention took nothing, so the entry keeps what it can be opened against.
    expect(screen.getByTestId("loop-run-artifact-round-2-fixes.md")).toBeInTheDocument();

    // The coverage numbers live on the fan-out, spelled out as the graph-eng
    // lock requires — and in the default register, not behind Inspect.
    const fanOut = props.registers.progress?.steps.find(step => step.fanOut);
    expect(fanOut?.fanOut?.countLabel).toBe("partial 7 of 10");
  });

  it("Should not tone a canceled outcome as a success", () => {
    const briefing = buildBriefingView(makeBriefing({ status: "canceled", tone: "ok" }));
    render(
      <LoopRunBriefing
        briefing={briefing}
        outcome={{
          outcome: {
            status: "canceled",
            label: "Canceled",
            cause: null,
            at: "2026-08-19T18:44:00Z",
            actorLabel: "pedro",
          },
          artifacts: [],
          producedNothing: true,
        }}
      />
    );
    const pill = screen.getByTestId("loop-run-briefing-outcome");
    expect(pill).toHaveAttribute("data-outcome", "canceled");
    expect(pill).toHaveAttribute("data-tone", "neutral");
  });
});

// The fixture's identity and the condition under test have to be the same fact:
// two matching literals are a coincidence a refactor can quietly break.
const PRUNED_SESSION_ID = "ses-5d871c99";

describe("LoopRunRegisters Goal turn history", () => {
  const turns: import("../../types").GoalTurn[] = [1, 2, 3].map(turn => ({
    seq: turn,
    generation: 1,
    node_id: "goal",
    item_index: 0,
    turn,
    prompt_attempt: 0,
    session_id: "session-goal",
    binding_handle: "binding-goal",
    binding_epoch: 1,
    prompt_id: `prompt-${turn}`,
    result_status: "completed",
    reason_code: null,
    stop_reason: "end_turn",
    verdict_outcome: turn === 3 ? "approved" : "rejected",
    blocking_issues:
      turn === 3
        ? []
        : [{ id: "count_below_target", note: `Count ${turn}; ${3 - turn} increments remain.` }],
    criteria: [],
    warnings: [],
    evidence_ref: `sha256:turn-${turn}`,
    prompt_ref: null,
    tokens_used: null,
    actor_kind: "daemon",
    actor_id: "loop-action",
    started_at: "2026-09-11T19:00:00Z",
    ended_at: "2026-09-11T19:00:30Z",
  }));

  /** Renders the Goal history disclosure with deterministic turns and observable pagination. */
  function renderHistory(overrides = {}) {
    const onLoadMore = vi.fn();
    render(
      <LoopRunRegisters
        generations={[]}
        graph={null}
        isLive={false}
        isReconnecting={false}
        nodeLifecycles={[]}
        nodes={[]}
        nowMs={Date.parse("2026-09-11T19:01:00Z")}
        onOpenChange={() => undefined}
        onSelectionChange={() => undefined}
        open
        registers={projectLoopRunRegisters({
          briefing: null,
          nodes: [],
          rollups: [],
          timeline: [],
          graph: null,
        })}
        rollups={[]}
        selection={null}
        goalTurns={{
          turns,
          isLoading: false,
          isError: false,
          hasMore: true,
          isLoadingMore: false,
          onLoadMore,
          ...overrides,
        }}
      />
    );
    return onLoadMore;
  }

  it("Should retain ordered rejected and approved turns with blockers and evidence in Inspect", async () => {
    const onLoadMore = renderHistory();
    fireEvent.click(screen.getByRole("button", { name: /Goal turns/ }));
    const timeline = screen.getByRole("region", { name: "Goal turn timeline" });
    expect(
      [...timeline.querySelectorAll("[data-turn-seq]")].map(row =>
        row.getAttribute("data-turn-seq")
      )
    ).toEqual(["1", "2", "3"]);
    expect(within(timeline).getAllByText("Rejected")).toHaveLength(2);
    expect(within(timeline).getByText("Approved")).toBeVisible();
    expect(within(timeline).getByText("Count 1; 2 increments remain.")).toBeVisible();
    expect(within(timeline).getByText("Count 2; 1 increments remain.")).toBeVisible();
    expect(within(timeline).getByText("sha256:turn-1")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Load more turns" }));
    expect(onLoadMore).toHaveBeenCalledOnce();
  });

  it("Should report an unread Goal history instead of an empty completed history", () => {
    renderHistory({ turns: [], isError: true, hasMore: false });
    fireEvent.click(screen.getByRole("button", { name: /Goal turns/ }));
    expect(screen.getByRole("alert")).toHaveTextContent("Could not load Goal turns");
    expect(screen.queryByText(/No Goal turns yet/)).not.toBeInTheDocument();
  });
});

describe("LoopRunRegisters pruned session", () => {
  const rosterNode = makeRosterNode("revisor-estilo", "succeeded", {
    generation: 1,
    attempts: [
      {
        attempt: 1,
        state: "succeeded",
        disposition: "settled",
        started_at: "2026-08-19T18:41:07Z",
        ended_at: "2026-08-19T18:43:38Z",
      },
    ],
    session_id: PRUNED_SESSION_ID,
    cell_task_id: "loop.looprun-1.g1.node.revisor-estilo.0",
    started_at: "2026-08-19T18:41:07Z",
    ended_at: "2026-08-19T18:43:38Z",
  });

  function renderRegisters(prunedSessionIds?: ReadonlySet<string>) {
    const nodes = [rosterNode];
    return render(
      <LoopRunRegisters
        generations={[]}
        graph={null}
        isLive={false}
        isReconnecting={false}
        nodeLifecycles={[]}
        nodes={nodes}
        nowMs={Date.parse("2026-08-19T19:00:00Z")}
        onOpenChange={() => undefined}
        onSelectionChange={() => undefined}
        open
        prunedSessionIds={prunedSessionIds}
        registers={projectLoopRunRegisters({
          briefing: null,
          nodes,
          rollups: [],
          timeline: [],
          graph: null,
        })}
        rollups={[]}
        selection={{ nodeId: "revisor-estilo", itemIndex: 0, generation: 1 }}
      />
    );
  }

  it("Should open the recorded session while it is still there", () => {
    renderRegisters();

    expect(screen.getByTestId("loop-node-panel-link-session")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-node-panel-degraded-session")).toBeNull();
  });

  it("Should say the session is gone instead of offering a link that 404s", () => {
    renderRegisters(new Set([PRUNED_SESSION_ID]));

    expect(screen.queryByTestId("loop-node-panel-link-session")).toBeNull();
    expect(screen.getByTestId("loop-node-panel-degraded-session")).toHaveTextContent(
      "Session no longer available"
    );
    // The record link is a different store's fact and survives the degrade.
    expect(screen.getByTestId("loop-node-panel-link-record")).toBeInTheDocument();
  });
});

// What the lib models and what the reader actually sees are two different
// assertions. These own the second one: the words that reach the DOM.
describe("LoopRunRegisters roster and generation lanes", () => {
  const runningNode = makeRosterNode("implementar", "running", {
    attempts: [
      { attempt: 1, state: "running", disposition: "open", started_at: "2026-08-19T18:40:00Z" },
    ],
    started_at: "2026-08-19T18:40:00Z",
    ended_at: null,
    usage: { tokens: 14_800 },
  });

  const generation = makeGeneration(2, {
    origin: "gate_revise",
    verdicts: [
      {
        blocking_issues: [],
        criteria: [],
        gate_id: "quality",
        item_index: 0,
        outcome: "invalid_output",
      },
    ],
  });

  async function openLane(lane: "nodes" | "generations") {
    const nodes = [runningNode];
    render(
      <LoopRunRegisters
        bestGeneration={2}
        generations={[generation]}
        graph={null}
        isLive
        isReconnecting={false}
        nodeLifecycles={[]}
        nodes={nodes}
        nowMs={Date.parse("2026-08-19T18:50:00Z")}
        onOpenChange={() => undefined}
        onSelectionChange={() => undefined}
        open
        registers={projectLoopRunRegisters({
          briefing: null,
          nodes,
          rollups: [],
          timeline: [],
          graph: null,
        })}
        rollups={[]}
        runStatus="running"
        selection={null}
      />
    );
    await userEvent.click(screen.getByTestId(`loop-lane-${lane}`));
  }

  it("Should read a running step as in progress with its elapsed clock", async () => {
    await openLane("nodes");

    const row = screen.getByTestId("loop-roster-row-2:implementar:0");
    expect(row).toHaveTextContent("in progress");
    expect(row).not.toHaveTextContent("not started");
    expect(row).toHaveTextContent("10m");
  });

  it("Should show tokens beside a cost the header labels an estimate", async () => {
    await openLane("nodes");

    // `formatTokenCount` is the app-wide token formatter; the roster reuses it
    // rather than minting a second spelling of the same number.
    expect(screen.getByTestId("loop-roster-row-2:implementar:0")).toHaveTextContent(
      "14.8K · ~$0.07"
    );
    expect(screen.getByRole("columnheader", { name: /est\. cost/i })).toBeInTheDocument();
  });

  it("Should render daemon-authorized actions on the owning roster row", async () => {
    const nodes = [runningNode];
    render(
      <LoopRunRegisters
        generations={[]}
        graph={null}
        isLive
        isReconnecting={false}
        nodeLifecycles={[]}
        nodes={nodes}
        nowMs={Date.parse("2026-08-19T18:50:00Z")}
        onOpenChange={() => undefined}
        onSelectionChange={() => undefined}
        open
        registers={projectLoopRunRegisters({
          briefing: null,
          nodes,
          rollups: [],
          timeline: [],
          graph: null,
        })}
        renderNodeActions={node => (
          <button data-testid={`row-actions-${node.nodeId}`} type="button">
            Actions
          </button>
        )}
        rollups={[]}
        runStatus="running"
        selection={null}
      />
    );
    await userEvent.click(screen.getByTestId("loop-lane-nodes"));

    expect(screen.getByTestId("loop-roster-row-2:implementar:0")).toContainElement(
      screen.getByTestId("row-actions-implementar")
    );
  });

  it("Should give every round's row its own DOM identity", async () => {
    // The same step id exists once per round. A locator that names only the step
    // matches several rows at once, so a test asserting on "the retrying row"
    // silently asserts on whichever one it happened to find first.
    const nodes = [runningNode, { ...runningNode, generation: 3 }];
    render(
      <LoopRunRegisters
        generations={[]}
        graph={null}
        isLive
        isReconnecting={false}
        nodeLifecycles={[]}
        nodes={nodes}
        nowMs={Date.parse("2026-08-19T18:50:00Z")}
        onOpenChange={() => undefined}
        onSelectionChange={() => undefined}
        open
        registers={projectLoopRunRegisters({
          briefing: null,
          nodes,
          rollups: [],
          timeline: [],
          graph: null,
        })}
        rollups={[]}
        runStatus="running"
        selection={null}
      />
    );
    await userEvent.click(screen.getByTestId("loop-lane-nodes"));
    await userEvent.click(screen.getByRole("button", { name: "All rounds" }));

    expect(screen.getByTestId("loop-roster-row-2:implementar:0")).toBeInTheDocument();
    expect(screen.getByTestId("loop-roster-row-3:implementar:0")).toBeInTheDocument();
  });

  it("Should state the round's outcome in words and its own usage", async () => {
    await openLane("generations");

    const round = screen.getByTestId("loop-generation-2");
    expect(round).toHaveAttribute("id", "loop-generation-2");
    expect(within(round).getByText("Best", { exact: true })).toBeInTheDocument();
    expect(round).toHaveTextContent("the output did not match its schema");
    expect(round).not.toHaveTextContent("invalid_output");
    expect(screen.getByTestId("loop-generation-usage-2")).toHaveTextContent("14.8K · ~$0.07 est.");
    // A live run holding an unsettled step has not finished the round.
    expect(screen.getByTestId("loop-generation-progress-2")).toHaveTextContent("still running");
  });

  it("Should expose watch subscriptions and durable cursors inside Inspect", () => {
    const nodes = [runningNode];
    render(
      <LoopRunRegisters
        generations={[]}
        graph={null}
        isLive
        isReconnecting={false}
        nodeLifecycles={[]}
        nodes={nodes}
        nowMs={Date.parse("2026-08-19T18:50:00Z")}
        onOpenChange={() => undefined}
        onSelectionChange={() => undefined}
        open
        registers={projectLoopRunRegisters({
          briefing: null,
          nodes,
          rollups: [],
          timeline: [],
          graph: null,
        })}
        rollups={[]}
        runStatus="watching"
        selection={null}
        watchEvents={{
          cursors: { loop_run_events: 17 },
          last_wake_at: "2026-08-19T18:44:00Z",
          subscriptions: [
            { kind: "task.status_changed", filter: "event.payload.to_status == 'blocked'" },
          ],
        }}
      />
    );

    expect(screen.getByTestId("loop-run-inspect-watch")).toHaveTextContent("task.status_changed");
    expect(screen.getByTestId("loop-run-inspect-watch")).toHaveTextContent(
      "event.payload.to_status == 'blocked'"
    );
    expect(screen.getByTestId("loop-run-inspect-cursors")).toHaveTextContent("loop_run_events17");
  });
});

// The default read of Progress is the steps the served count is counting. A
// gate that passed and a branch the route declined carry state but say nothing
// the reader needs first, so they fold behind their own count — hidden, never
// dropped, and back in graph order on one click.
describe("LoopRunStepsProgress fold", () => {
  function routedProgress() {
    // VC-20's roster through the production projection: review ran, the gate
    // passed, write_artifacts was provably declined, collect_fixes is still ahead.
    return buildScenarioProps(registerFixtures.registerRoutedScenario()).registers.progress!;
  }

  it("Should hide quiet rows behind a counted summary and keep the way ahead visible", () => {
    render(<LoopRunStepsProgress progress={routedProgress()} />);

    expect(screen.getByTestId("loop-run-step-review")).toBeInTheDocument();
    // Reachable-but-unstarted is where the run is going; it never folds.
    expect(screen.getByTestId("loop-run-step-collect_fixes")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-run-step-has_issues")).toBeNull();
    expect(screen.queryByTestId("loop-run-step-write_artifacts")).toBeNull();

    // The fact the hidden rows carried stays on screen while they are hidden.
    expect(screen.getByTestId("loop-run-step-fold-summary")).toHaveTextContent(
      "1 succeeded · 1 not taken"
    );
    const toggle = screen.getByTestId("loop-run-step-fold-toggle");
    expect(toggle).toHaveTextContent("2 more steps");
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(toggle).toHaveAttribute("aria-controls", screen.getByTestId("loop-run-step-list").id);
  });

  it("Should bring the folded rows back in graph order, not as an appendix", async () => {
    render(<LoopRunStepsProgress progress={routedProgress()} />);

    await userEvent.click(screen.getByTestId("loop-run-step-fold-toggle"));

    const rows = within(screen.getByTestId("loop-run-step-list")).getAllByRole("listitem");
    expect(rows.map(row => row.getAttribute("data-node-id"))).toEqual([
      "review",
      "has_issues",
      "write_artifacts",
      "collect_fixes",
    ]);
    // Pending and not-taken stay distinguishable at a glance (SI-14): the
    // literal word travels with the chip once the row is back.
    expect(
      within(screen.getByTestId("loop-run-step-write_artifacts")).getByTestId(
        "loop-state-chip-not_taken"
      )
    ).toHaveTextContent("not taken");
    const toggle = screen.getByTestId("loop-run-step-fold-toggle");
    expect(toggle).toHaveTextContent("Show fewer steps");
    expect(toggle).toHaveAttribute("aria-expanded", "true");
  });
});

// Invariant (#705): a step that started loop runs reads where each child is —
// the inputs that set it apart, its status, the step it is on, why and for how
// long, and how far through it is — closed by default in Progress, open in the
// node panel, compact on the graph card, and a level down for a child's own
// children. Every reading comes from the child's own detail, briefing and roster.
// Owner: run-page components composed with useLoopChildRun and MSW I/O.
describe("Nested child runs", () => {
  /** Stubs the child-run routes over MSW and returns a provider with a query cache and the page context. */
  async function childReadHarness(
    options: {
      clockLive?: boolean;
      nowMs?: number;
      handlers?: import("msw").HttpHandler[];
      onRequest?: (url: URL) => void;
    } = {}
  ) {
    const { QueryClient, QueryClientProvider } = await import("@tanstack/react-query");
    const { createMswFetch } = await import("@/test/msw-fetch");
    const fixtures = await import("../stories/loop-run-nested-fixtures");
    const { STORY_NOW } = await import("../stories/loop-run-page-fixture-world");
    const { LoopRunChildReadContext } = await import("../../hooks/use-loop-run-child-read");
    const mswFetch = createMswFetch(() => [
      ...(options.handlers ?? []),
      ...fixtures.nestedChildRunHandlers,
    ]);
    vi.stubGlobal("fetch", (input: RequestInfo | URL, init?: RequestInit) => {
      const request = input instanceof Request ? input : new Request(input, init);
      options.onRequest?.(new URL(request.url, window.location.origin));
      return mswFetch(input, init);
    });
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const context = {
      workspaceId: fixtures.NESTED_STORY_WORKSPACE_ID,
      nowMs: options.nowMs ?? STORY_NOW,
      clockLive: options.clockLive ?? true,
    };
    /** Query cache plus the page context every child-run read takes. */
    function Harness({ children }: { children: React.ReactNode }) {
      return (
        <QueryClientProvider client={client}>
          <LoopRunChildReadContext value={context}>{children}</LoopRunChildReadContext>
        </QueryClientProvider>
      );
    }
    /** Drops the cache and the fetch stub so the next case starts clean. */
    const cleanup = () => {
      client.clear();
      vi.unstubAllGlobals();
    };
    return { Harness, cleanup, fixtures };
  }

  /** The child row for one run id inside a scope, failing loudly when it is absent. */
  function childRow(scope: HTMLElement, runId: string) {
    const row = within(scope)
      .getAllByTestId("loop-run-child-run")
      .find(element => element.getAttribute("data-child-run-id") === runId);
    if (!row) throw new Error(`no row for ${runId}`);
    return row;
  }

  it("Should read each child's inputs, status, step and time once Progress opens them", async () => {
    const { Harness, cleanup, fixtures } = await childReadHarness();
    const progress = buildScenarioProps(fixtures.nestedLoopsScenario()).registers.progress!;
    const rendered = render(
      <Harness>
        <LoopRunStepsProgress progress={progress} />
      </Harness>
    );
    try {
      const step = screen.getByTestId("loop-run-step-fix_batch");
      expect(
        within(within(step).getByTestId("loop-run-step-rollup-fix_batch")).getByTestId(
          "loop-state-chip-awaiting_child"
        )
      ).toBeVisible();
      const toggle = within(step).getByTestId("loop-run-child-runs-toggle");
      expect(toggle).toHaveTextContent("4 child runs");
      expect(toggle).toHaveAttribute("aria-expanded", "false");
      expect(within(step).queryAllByTestId("loop-run-child-run")).toHaveLength(0);

      await userEvent.click(toggle);
      expect(toggle).toHaveAttribute("aria-expanded", "true");

      await waitFor(() =>
        expect(
          within(childRow(step, "r-8f21a0")).getByTestId("loop-run-child-run-step")
        ).toHaveTextContent("At run tests")
      );
      const working = childRow(step, "r-8f21a0");
      // Siblings run the same loop; their inputs are what tell them apart.
      expect(within(working).getByTestId("loop-run-child-run-inputs")).toHaveTextContent(
        "batch: api · files: 4"
      );
      expect(within(working).getByTestId("loop-run-child-run-link")).toHaveTextContent(
        "fix-one-batch"
      );
      expect(within(working).getByTestId("loop-run-child-run-status")).toHaveTextContent("Running");
      expect(within(working).getByTestId("loop-run-child-run-on-step")).toHaveTextContent(
        "2m 00s on this step"
      );
      expect(within(working).getByTestId("loop-run-child-run-meta")).toHaveTextContent(
        "1 of 4 steps · 6m 00s"
      );

      await waitFor(() =>
        expect(
          within(childRow(step, "r-3b9c55")).getByTestId("loop-run-child-run-step")
        ).toHaveTextContent("At finalize round — waiting for your decision")
      );
      const stuck = childRow(step, "r-3b9c55");
      expect(within(stuck).getByTestId("loop-run-child-run-status")).toHaveTextContent(
        "Needs approval"
      );
      // The stuck child's time on its step keeps growing past its frozen run clock.
      expect(within(stuck).getByTestId("loop-run-child-run-on-step")).toHaveTextContent(
        "47m 00s on this step"
      );

      await waitFor(() =>
        expect(
          within(childRow(step, "r-d40e17")).getByTestId("loop-run-child-run-meta")
        ).toHaveTextContent("4 of 4 steps · 9m 00s")
      );
      const done = childRow(step, "r-d40e17");
      expect(within(done).getByTestId("loop-run-child-run-status")).toHaveTextContent("Done");
      expect(within(done).getByTestId("loop-run-child-run-step")).toHaveTextContent("");

      // A child waiting on a loop of its own opens it a level down.
      const parent = await waitFor(() => {
        const row = childRow(step, "r-5c71e2");
        within(row).getByTestId("loop-run-child-runs-toggle");
        return row;
      });
      const nestedToggle = within(parent).getByTestId("loop-run-child-runs-toggle");
      expect(nestedToggle).toHaveTextContent("1 child run");
      await userEvent.click(nestedToggle);
      await waitFor(() =>
        expect(
          within(childRow(parent, "r-9e04aa")).getByTestId("loop-run-child-run-step")
        ).toHaveTextContent("At judge")
      );
      const grandchild = childRow(parent, "r-9e04aa");
      expect(grandchild).toHaveAttribute("data-depth", "1");
      expect(within(grandchild).getByTestId("loop-run-child-run-link")).toHaveTextContent(
        "review-one-file"
      );
    } finally {
      rendered.unmount();
      cleanup();
    }
  });

  it("Should tell awaiting child from pending on the graph and read the child on its card", async () => {
    const { Harness, cleanup, fixtures } = await childReadHarness();
    const scenario = fixtures.nestedWaveScenario();
    const props = buildScenarioProps(scenario);
    const dag = buildRunDag({
      graph: props.graph,
      nodes: scenario.rosterNodes ?? [],
      rollups: [],
      round: 1,
    });
    const rendered = render(
      <Harness>
        <LoopRunDag dag={dag} onSelect={vi.fn()} selection={null} />
      </Harness>
    );
    try {
      /** The state glyph drawn on one graph card. */
      const glyph = (nodeId: string) =>
        screen.getByTestId(`loop-dag-node-${nodeId}`).querySelector('[data-slot="state-glyph"]');
      expect(glyph("wave")).toHaveAttribute("data-state", "delegated");
      expect(glyph("wave_ok")).toHaveAttribute("data-state", "queued");

      const wave = screen.getByTestId("loop-dag-node-wave");
      const line = await within(wave).findByTestId("loop-dag-child-line");
      await waitFor(() => expect(line).toHaveTextContent("fix batch9m 00s"));
      expect(line).toHaveAttribute(
        "title",
        "Child run run-one-wave is at fix batch — waiting on a child run, 9m 00s on this step"
      );
      // Cards with no child carry no child line.
      expect(
        within(screen.getByTestId("loop-dag-node-wave_ok")).queryByTestId("loop-dag-child-line")
      ).toBeNull();
    } finally {
      rendered.unmount();
      cleanup();
    }
  });

  it("Should open the child in the node panel without asking", async () => {
    const { Harness, cleanup, fixtures } = await childReadHarness();
    const { buildNodePanel } = await import("../../lib/loop-node-panel-view");
    const { LoopNodePanel } = await import("../run-page/inspect/loop-node-panel");
    const scenario = fixtures.nestedWaveScenario();
    const wave = (scenario.rosterNodes ?? []).find(node => node.node_id === "wave")!;
    const panel = buildNodePanel({ node: wave, graph: buildScenarioProps(scenario).graph });
    const rendered = render(
      <Harness>
        <LoopNodePanel panel={panel} />
      </Harness>
    );
    try {
      const panelEl = screen.getByTestId("loop-node-panel");
      expect(within(panelEl).getByTestId("loop-run-child-runs-toggle")).toHaveAttribute(
        "aria-expanded",
        "true"
      );
      await waitFor(() =>
        expect(
          within(childRow(panelEl, "r-wave02")).getByTestId("loop-run-child-run-step")
        ).toHaveTextContent("At fix batch — waiting on a child run")
      );
      expect(
        within(childRow(panelEl, "r-wave02")).getByTestId("loop-run-child-run-inputs")
      ).toHaveTextContent('batches: ["api","billing"] · wave: 2');
    } finally {
      rendered.unmount();
      cleanup();
    }
  });

  it("Should read only the child's current round, and say so when it is too wide", async () => {
    const { HttpResponse } = await import("msw");
    const { compozyApiMock } = await import("@/storybook/openapi-msw");
    const rosterReads: URL[] = [];
    const { Harness, cleanup, fixtures } = await childReadHarness({
      onRequest: url => {
        if (url.pathname.endsWith("/nodes")) rosterReads.push(url);
      },
      handlers: [
        // A round wider than the row reads: every page promises another.
        compozyApiMock.get(
          "/api/workspaces/{workspace_id}/loop-runs/{run_id}/nodes",
          ({ params, request }) => {
            if (params.run_id !== "r-8f21a0") return undefined;
            const cursor = Number(new URL(request.url).searchParams.get("cursor") ?? "0");
            return HttpResponse.json({
              run_id: "r-8f21a0",
              loop_name: "fix-one-batch",
              run_status: "running",
              nodes: [],
              fanout_rollups: [],
              next_cursor: String(cursor + 1),
            });
          }
        ),
      ],
    });
    const progress = buildScenarioProps(fixtures.nestedLoopsScenario()).registers.progress!;
    const rendered = render(
      <Harness>
        <LoopRunStepsProgress progress={progress} />
      </Harness>
    );
    try {
      const step = screen.getByTestId("loop-run-step-fix_batch");
      await userEvent.click(within(step).getByTestId("loop-run-child-runs-toggle"));
      await waitFor(() => {
        expect(
          within(childRow(step, "r-8f21a0")).getByTestId("loop-run-child-run-step")
        ).toHaveTextContent("This round has more steps than a row reads");
      });
      const wide = rosterReads.filter(url => url.pathname.includes("/r-8f21a0/"));
      // The round the briefing names, never the oldest-first whole roster,
      // and no more pages than the cap.
      expect(wide.every(url => url.searchParams.get("generation") === "1")).toBe(true);
      expect(wide).toHaveLength(5);
    } finally {
      rendered.unmount();
      cleanup();
    }
  });

  it("Should stop paging a child's round after a page fails, leaving the retry to the poll", async () => {
    const { HttpResponse } = await import("msw");
    const { compozyApiMock } = await import("@/storybook/openapi-msw");
    const rosterReads: URL[] = [];
    const { Harness, cleanup, fixtures } = await childReadHarness({
      onRequest: url => {
        if (url.pathname.endsWith("/r-8f21a0/nodes")) rosterReads.push(url);
      },
      handlers: [
        compozyApiMock.get(
          "/api/workspaces/{workspace_id}/loop-runs/{run_id}/nodes",
          ({ params, request }) => {
            if (params.run_id !== "r-8f21a0") return undefined;
            // Page one promises more; the next page is the one the daemon fails.
            return new URL(request.url).searchParams.has("cursor")
              ? HttpResponse.json({ error: "roster unavailable" }, { status: 500 })
              : HttpResponse.json({
                  run_id: "r-8f21a0",
                  loop_name: "fix-one-batch",
                  run_status: "running",
                  nodes: [],
                  fanout_rollups: [],
                  next_cursor: "1",
                });
          }
        ),
      ],
    });
    const progress = buildScenarioProps(fixtures.nestedLoopsScenario()).registers.progress!;
    const rendered = render(
      <Harness>
        <LoopRunStepsProgress progress={progress} />
      </Harness>
    );
    try {
      const step = screen.getByTestId("loop-run-step-fix_batch");
      await userEvent.click(within(step).getByTestId("loop-run-child-runs-toggle"));
      await waitFor(() =>
        expect(
          within(childRow(step, "r-8f21a0")).getByTestId("loop-run-child-run-step")
        ).toHaveTextContent("Couldn't read this child run")
      );
      await new Promise(resolve => setTimeout(resolve, 300));
      // One read for page one, one for the page that failed — and no more.
      expect(rosterReads).toHaveLength(2);
    } finally {
      rendered.unmount();
      cleanup();
    }
  });

  it("Should say a child could not be read when its briefing fails, keeping what did arrive", async () => {
    const { HttpResponse } = await import("msw");
    const { compozyApiMock } = await import("@/storybook/openapi-msw");
    const { Harness, cleanup, fixtures } = await childReadHarness({
      handlers: [
        compozyApiMock.get(
          "/api/workspaces/{workspace_id}/loop-runs/{run_id}/briefing",
          ({ params }) =>
            params.run_id === "r-8f21a0"
              ? HttpResponse.json({ error: "briefing unavailable" }, { status: 500 })
              : undefined
        ),
      ],
    });
    const progress = buildScenarioProps(fixtures.nestedLoopsScenario()).registers.progress!;
    const rendered = render(
      <Harness>
        <LoopRunStepsProgress progress={progress} />
      </Harness>
    );
    try {
      const step = screen.getByTestId("loop-run-step-fix_batch");
      await userEvent.click(within(step).getByTestId("loop-run-child-runs-toggle"));
      await waitFor(() =>
        expect(
          within(childRow(step, "r-8f21a0")).getByTestId("loop-run-child-run-step")
        ).toHaveTextContent("Couldn't read this child run")
      );
      // The detail did arrive, so the child is still named and its status shown.
      const row = childRow(step, "r-8f21a0");
      expect(within(row).getByTestId("loop-run-child-run-status")).toHaveTextContent("Running");
    } finally {
      rendered.unmount();
      cleanup();
    }
  });

  it("Should keep a running child's clock ticking after the page clock stops", async () => {
    // A detached child outlives its parent: the parent page's clock is frozen
    // (here at 0), and the child's elapsed time must not freeze with it.
    const { Harness, cleanup, fixtures } = await childReadHarness({ clockLive: false, nowMs: 0 });
    const progress = buildScenarioProps(fixtures.nestedLoopsScenario()).registers.progress!;
    const rendered = render(
      <Harness>
        <LoopRunStepsProgress progress={progress} />
      </Harness>
    );
    try {
      const step = screen.getByTestId("loop-run-step-fix_batch");
      await userEvent.click(within(step).getByTestId("loop-run-child-runs-toggle"));
      // The child started six minutes before the story clock and the page clock
      // is frozen at 0, so a reading of at least six minutes can only come from
      // the child's own live clock. How far past six depends on the suite's pace.
      await waitFor(() => {
        const meta = within(childRow(step, "r-8f21a0")).getByTestId("loop-run-child-run-meta");
        const reading = /1 of 4 steps · (\d+)m (\d+)s/.exec(meta.textContent ?? "");
        expect(reading).not.toBeNull();
        expect(Number(reading![1]) * 60 + Number(reading![2])).toBeGreaterThanOrEqual(360);
      });
    } finally {
      rendered.unmount();
      cleanup();
    }
  });
});

// Component-owned dialog and diff behavior formerly exercised through Storybook E2E.
// Keep these in the canonical run-page suite; real daemon journeys remain in web/e2e.
describe("Loop run dialogs and diff", () => {
  it("Should prefill a fork from its source and surface field and generation refusals", async () => {
    const { LoopForkDialog } = await import("../run-page/loop-fork-dialog");
    const { releaseTrainDetail, releaseTrainRun, RELEASE_TRAIN_LOOP_NAME } =
      await import("../../mocks");
    const onSubmit = vi.fn();
    const props = {
      open: true,
      loopName: RELEASE_TRAIN_LOOP_NAME,
      generations: [3, 2, 1],
      defaultGeneration: 2,
      inputSchema: releaseTrainDetail.definition.inputs,
      sourceInputs: releaseTrainRun.inputs ?? {},
      onOpenChange: vi.fn(),
      onSubmit,
    };
    const { rerender } = render(<LoopForkDialog {...props} />);
    expect(screen.getByTestId("loop-fork-dialog")).toBeVisible();
    expect(screen.getByTestId("loop-fork-generation")).toBeVisible();
    expect(screen.getByTestId("loop-fork-input-severity")).toBeVisible();
    expect(screen.getByTestId("loop-fork-submit")).toBeEnabled();
    fireEvent.click(screen.getByTestId("loop-fork-submit"));
    expect(onSubmit).toHaveBeenCalledWith({
      generation: 2,
      inputs: props.sourceInputs,
      reason: "",
    });

    rerender(
      <LoopForkDialog {...props} fieldErrors={{ services: "At least one service is required." }} />
    );
    expect(screen.getByText("At least one service is required.")).toBeVisible();
    rerender(
      <LoopForkDialog {...props} blockedReason="Generation 5 does not exist on this run." />
    );
    expect(screen.getByTestId("loop-fork-blocked")).toBeVisible();
    expect(screen.getByTestId("loop-fork-submit")).toBeDisabled();
  });

  it("Should keep the recorded amend output visible while showing daemon field errors", () => {
    const props = {
      open: true,
      node: loopNodeLifecycleFixture({
        nodeId: "render-notes",
        paused: true,
        state: "paused",
        outputStatus: "succeeded",
        itemIndex: 0,
      }),
      originalOutput: { risk: "high", summary: "Rollout for billing" },
      outputSchema: {
        type: "object",
        required: ["risk"],
        properties: {
          risk: { type: "string", enum: ["low", "medium", "high"] },
          summary: { type: "string" },
        },
      },
      onOpenChange: vi.fn(),
      onConfirm: vi.fn(),
    };
    const { rerender } = render(<LoopNodeAmendDialog {...props} />);
    expect(screen.getByTestId("loop-node-amend-dialog")).toBeVisible();
    expect(screen.getByTestId("loop-amend-original")).toBeVisible();
    expect(screen.getByTestId("loop-amend-original")).toHaveTextContent('risk: "high"');
    expect(screen.getByTestId("loop-amend-reason")).toBeVisible();
    rerender(
      <LoopNodeAmendDialog
        {...props}
        fieldErrors={{ risk: "risk must be one of low, medium, high." }}
      />
    );
    expect(screen.getByTestId("loop-amend-field-error-risk")).toHaveTextContent(
      "risk must be one of low, medium, high."
    );
    expect(screen.getByTestId("loop-amend-original")).toHaveTextContent('risk: "high"');
  });

  it("Should preview rerun and carried nodes while withholding amend and rerun from running cells", async () => {
    const { LoopNodeRerunDialog } = await import("../run-page/loop-node-rerun-dialog");
    const { unmount } = render(
      <LoopNodeRerunDialog
        open
        node={loopNodeLifecycleFixture({
          nodeId: "apply-migration",
          outputStatus: "succeeded",
          itemIndex: 0,
        })}
        rerunSet={{
          fromNode: "apply-migration",
          rerunNodes: ["apply-migration", "collect-rollout", "render-notes"],
          carriedNodes: ["services", "triage", "standard", "rollout"],
        }}
        onConfirm={vi.fn()}
        onOpenChange={vi.fn()}
      />
    );
    expect(screen.getByTestId("loop-node-rerun-dialog")).toBeVisible();
    expect(screen.getByTestId("loop-rerun-set")).toBeVisible();
    expect(screen.getByTestId("loop-rerun-node-apply-migration")).toBeVisible();
    expect(screen.getByTestId("loop-rerun-node-collect-rollout")).toBeVisible();
    expect(screen.getByTestId("loop-rerun-carried")).toHaveTextContent(
      "4 nodes carry forward unchanged."
    );
    unmount();
    render(
      <LoopNodeControlMenu
        node={loopNodeLifecycleFixture({ nodeId: "task_04", outputStatus: "running" })}
        runStatus="running"
        onVerb={vi.fn()}
      />
    );
    await userEvent.click(screen.getByTestId("loop-node-menu-trigger-task_04"));
    expect(screen.queryByTestId("loop-node-verb-amend")).not.toBeInTheDocument();
    expect(screen.queryByTestId("loop-node-verb-rerun")).not.toBeInTheDocument();
  });

  it("Should render grouped generation differences, run inputs, and an empty comparison", async () => {
    const { LoopRunDiffView } = await import("../run-diff/loop-run-diff-view");
    const { LoopRunDiffPickers } = await import("../run-diff/loop-run-diff-pickers");
    const { projectLoopDiff } = await import("../../lib/loop-run-diff-model");
    const { generationDiffFixture, runDiffFixture, emptyDiffFixture } = await import("../../mocks");
    const { rerender } = render(
      <LoopRunDiffView
        view={projectLoopDiff(generationDiffFixture)}
        pickers={
          <LoopRunDiffPickers
            mode="generation"
            generations={[3, 2, 1]}
            baseGeneration={3}
            againstGeneration={2}
            againstRunId=""
            runs={[]}
            onModeChange={vi.fn()}
            onBaseGenerationChange={vi.fn()}
            onAgainstGenerationChange={vi.fn()}
            onAgainstRunChange={vi.fn()}
          />
        }
      />
    );
    expect(screen.getByTestId("loop-run-diff-view")).toBeVisible();
    expect(screen.getByTestId("loop-diff-pickers")).toBeVisible();
    expect(screen.getAllByTestId(/^loop-diff-group-/)[0]).toBeVisible();
    expect(screen.getAllByTestId(/^loop-diff-row-/)[0]).toBeVisible();
    expect(screen.getAllByTestId(/^loop-diff-row-/)[0]).toHaveAttribute("data-change");
    rerender(<LoopRunDiffView view={projectLoopDiff(runDiffFixture)} />);
    expect(screen.getByTestId("loop-diff-inputs")).toBeVisible();
    rerender(<LoopRunDiffView view={projectLoopDiff(emptyDiffFixture)} />);
    expect(screen.getByTestId("loop-diff-empty")).toBeVisible();
  });
});

// E2E-020/021/024 presentation belongs to these real components, independent of a browser.
describe("Loop request and timeline presentation", () => {
  it("E2E-020 / E2E-021: Should render ask errors and navigate to the persisted review decisions", () => {
    const scenario = graphEngFixtures.pendingRequestsScenario();
    const props = buildScenarioProps(scenario);
    const cardProps = {
      run: props.run,
      request: null,
      fallbackFacts: [],
      showApproval: false,
      requests: props.requests,
      onDecision: vi.fn(),
    };
    const { rerender } = render(<LoopRunNeedsYouCard {...cardProps} />);
    expect(screen.getAllByTestId("loop-request-card")).toHaveLength(1);
    expect(screen.getByTestId("loop-request-progress")).toHaveTextContent("Question 1 of 2");
    expect(screen.getByTestId("loop-request-prompt")).toHaveTextContent(
      "Which regions ship first?"
    );
    fireEvent.click(screen.getByTestId("loop-request-details"));
    expect(screen.getByTestId("loop-request-context")).toBeVisible();
    expect(screen.getByTestId("loop-request-context-fetch")).toBeVisible();
    expect(screen.getByTestId("loop-request-submit")).toBeEnabled();
    rerender(
      <LoopRunNeedsYouCard
        {...cardProps}
        requestState={{
          engagedKey: "3:confirm-rollout:0",
          fieldErrors: { regions: "At least one region is required." },
        }}
      />
    );
    expect(screen.getByTestId("loop-request-field-error-regions")).toHaveTextContent(
      "At least one region is required."
    );
    expect(screen.getByTestId("loop-request-submit")).toBeEnabled();

    fireEvent.click(screen.getByTestId("loop-request-next"));
    expect(screen.getByTestId("loop-request-progress")).toHaveTextContent("Question 2 of 2");
    expect(screen.getByTestId("loop-review-proposed-args")).toBeVisible();
    for (const decision of ["approve", "edit", "reject", "respond"]) {
      expect(screen.getByTestId(`loop-request-decision-${decision}`)).toBeVisible();
    }
    expect(screen.queryByTestId("loop-request-decision-escalate")).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId("loop-request-decision-edit"));
    expect(screen.getByTestId("loop-review-proposed-args")).toBeVisible();
  });

  it("E2E-022: Should show recorded outcomes, daemon refusals and pending submission state", () => {
    const settled = buildScenarioProps(graphEngFixtures.resolvedRequestsScenario());
    const pending = buildScenarioProps(graphEngFixtures.pendingRequestsScenario());
    const common = { request: null, fallbackFacts: [], showApproval: false, onDecision: vi.fn() };
    const { rerender } = render(
      <LoopRunNeedsYouCard
        key="settled"
        {...common}
        run={settled.run}
        requests={settled.requests}
      />
    );
    expect(screen.getAllByTestId("loop-request-resolution")[0]).toBeVisible();
    expect(screen.queryByTestId("loop-request-submit")).not.toBeInTheDocument();
    rerender(
      <LoopRunNeedsYouCard
        key="refused"
        {...common}
        run={pending.run}
        requests={pending.requests}
        requestState={{
          engagedKey: "3:apply-migration:0",
          refusal: "Someone already answered this request.",
        }}
      />
    );
    expect(screen.getByTestId("loop-request-refusal")).toHaveTextContent("already answered");
    rerender(
      <LoopRunNeedsYouCard
        key="pending"
        {...common}
        run={pending.run}
        requests={pending.requests}
        requestState={{ engagedKey: "3:confirm-rollout:0", isAnswerPending: true }}
      />
    );
    expect(screen.getByTestId("loop-request-submit")).toBeDisabled();
  });

  it("E2E-024: Should render graph-completion timeline rows from the durable projection", () => {
    const props = buildScenarioProps(graphEngFixtures.pendingRequestsScenario());
    if (!props.storyPaging) throw new Error("Graph-completion fixture requires story paging");
    render(<LoopRunStory beats={props.registers.beats} paging={props.storyPaging} />);
    const story = screen.getByTestId("loop-run-story");
    expect(story).toBeVisible();
    expect(within(story).getAllByTestId(/^loop-run-beat-/)[0]).toBeVisible();
    for (const fragment of ["standard", "Which regions ship first?", "render-notes"]) {
      expect(story).toHaveTextContent(fragment);
    }
  });
});

// Invariant: reduced motion removes a genuinely live DAG pulse while every
// roster state retains both its readable name and glyph. Owner: run-page components.
describe("E2E-019: run graph motion and non-color state signals", () => {
  it("Should unmount a live graph pulse when reduced motion is enabled", async () => {
    const { MotionConfig } = await import("motion/react");
    const dag = buildRunDag({
      graph: {
        nodes: ["prepare", "execute"].map(id => ({
          id,
          nodeClass: "action" as const,
          kind: "run-agent",
          isGate: false,
          eventsCount: 0,
          routes: [],
          hasAskExpect: false,
        })),
        edges: [{ from: "prepare", to: "execute" }],
      },
      nodes: [
        makeRosterNode("prepare", "succeeded", { generation: 1 }),
        makeRosterNode("execute", "running", { generation: 1 }),
      ],
      rollups: [],
      round: 1,
    });
    const onSelect = vi.fn();
    const { rerender } = render(
      <MotionConfig reducedMotion="never">
        <LoopRunDag dag={dag} onSelect={onSelect} selection={null} />
      </MotionConfig>
    );
    expect(screen.getByTestId("loop-dag-edge-pulse")).toBeVisible();

    rerender(
      <MotionConfig reducedMotion="always">
        <LoopRunDag dag={dag} onSelect={onSelect} selection={null} />
      </MotionConfig>
    );
    expect(screen.queryByTestId("loop-dag-edge-pulse")).not.toBeInTheDocument();
    expect(screen.getByTestId("loop-dag-node-execute")).toHaveAttribute("data-state", "running");
  });

  it("Should pair every accessible roster state name with its visible glyph", () => {
    render(
      <>
        {LOOP_ROSTER_STATES.map(state => (
          <LoopNodeStateChip chip={loopRosterStateChip(state)} key={state} />
        ))}
      </>
    );
    for (const state of LOOP_ROSTER_STATES) {
      const chip = screen.getByTestId(`loop-state-chip-${state}`);
      expect(chip).toHaveAccessibleName(loopRosterStateChip(state).label);
      expect(chip).toHaveTextContent(loopRosterStateChip(state).label);
      expect(chip.querySelectorAll("svg")).toHaveLength(1);
    }
  });
});

// Invariant: request forms send the schema's exact value through the route's real
// mutation state; a successful response cannot replace the durable read projection.
// Owner: run-page components composed with useLoopRunRequestsState and MSW I/O.
describe("RunRequests and RunEnumRequest route composition", () => {
  it.each(["ask", "enum"] as const)(
    "E2E-022: Should submit the %s story request without optimistically resolving it",
    async kind => {
      const { QueryClient, QueryClientProvider } = await import("@tanstack/react-query");
      const { HttpResponse } = await import("msw");
      const { compozyApiMock } = await import("@/storybook/openapi-msw");
      const { createMswFetch } = await import("@/test/msw-fetch");
      const { storybookSystemHandlerGroups } = await import("@/storybook/msw");
      const { useLoopRun } = await import("../../hooks/use-loops");
      const { useLoopRunRequestsState } =
        await import("@/systems/os/apps/loops/use-loop-run-requests-state");
      const { GRAPH_ENG_RUN_ID, pendingEnumAskRequest, releaseTrainRunDetail } =
        await import("../../mocks");
      const { primaryWorkspaceFixture } = await import("@/systems/workspace/mocks");
      const { STORY_NOW } = await import("../stories/loop-run-page-fixture-world");
      const storyHandlers = Object.values(storybookSystemHandlerGroups).flat();
      const fixture =
        kind === "enum"
          ? { ...releaseTrainRunDetail, requests: [pendingEnumAskRequest] }
          : releaseTrainRunDetail;
      const requests: unknown[] = [];
      const responses: Array<{ status: number; body: unknown }> = [];
      const mswFetch = createMswFetch(() => [
        compozyApiMock.get("/api/workspaces/{workspace_id}/loop-runs/{run_id}", () =>
          HttpResponse.json(fixture)
        ),
        ...storyHandlers,
      ]);
      vi.stubGlobal("fetch", async (input: RequestInfo | URL, init?: RequestInit) => {
        const request = input instanceof Request ? input.clone() : new Request(input, init);
        const answering =
          request.method === "POST" && new URL(request.url).pathname.endsWith("/respond");
        if (answering) requests.push(await request.clone().json());
        const response = await mswFetch(input, init);
        if (answering)
          responses.push({ status: response.status, body: await response.clone().json() });
        return response;
      });
      const client = new QueryClient({
        defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
      });
      function RequestRoute() {
        const query = useLoopRun(primaryWorkspaceFixture.id, GRAPH_ENG_RUN_ID);
        const requestState = useLoopRunRequestsState(primaryWorkspaceFixture.id, GRAPH_ENG_RUN_ID);
        const detail = query.data;
        if (!detail) return null;
        return (
          <LoopRunNeedsYouCard
            run={detail.run}
            request={null}
            requests={(detail.requests ?? []).map(request =>
              projectLoopRequest(request, {
                nowMs: STORY_NOW,
                runStatus: detail.run.status,
              })
            )}
            requestState={requestState}
            fallbackFacts={[]}
            showApproval={false}
            onDecision={vi.fn()}
          />
        );
      }
      const rendered = render(
        <QueryClientProvider client={client}>
          <RequestRoute />
        </QueryClientProvider>
      );
      try {
        const card = await screen.findByTestId("loop-request-card");
        expect(card).toBeVisible();
        if (kind === "enum") {
          expect(within(card).getByTestId("loop-request-field-decision")).toBeVisible();
          fireEvent.click(within(card).getByRole("radio", { name: "approve" }));
        } else {
          fireEvent.change(within(card).getByTestId("loop-request-field-regions"), {
            target: { value: '["us-east"]' },
          });
          fireEvent.click(within(card).getByTestId("loop-request-option-canary-true"));
        }
        const submit = within(card).getByTestId("loop-request-submit");
        expect(submit).toBeEnabled();
        fireEvent.click(submit);
        await waitFor(() =>
          expect(responses).toEqual([
            { status: 200, body: expect.objectContaining({ state: "answered" }) },
          ])
        );
        expect(requests).toEqual([
          expect.objectContaining({
            payload:
              kind === "enum" ? { decision: "approve" } : { regions: ["us-east"], canary: true },
          }),
        ]);
        await waitFor(() => expect(submit).toBeEnabled());
        expect(within(card).queryByTestId("loop-request-resolution")).not.toBeInTheDocument();
      } finally {
        rendered.unmount();
        client.clear();
        vi.unstubAllGlobals();
      }
    }
  );
});
