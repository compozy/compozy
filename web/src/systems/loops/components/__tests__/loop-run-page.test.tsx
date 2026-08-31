import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { loopNodeLifecycleFixture } from "../../testing/loop-node-lifecycle-fixture";
import type { LoopRosterTableModel } from "../../lib/loop-run-roster-table";
import type { LoopRunStoryScenario } from "../stories/loop-run-scenario-types";

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
const visualContractFixtures = await import("../stories/loop-run-vc-fixtures");
const lifecycleFixtures = await import("../stories/loop-run-lifecycle-fixtures");
const graphEngFixtures = await import("../stories/loop-run-graph-eng-fixtures");
const pageFixtures = await import("../stories/loop-run-page-fixtures");
const metricFixtures = await import("../stories/loop-run-metric-fixtures");
const { registerPartialOutputsScenario } = registerFixtures;
const { buildScenarioProps } = await import("../stories/loop-run-scenario-props");
const { LoopRunNeedsYouCard } = await import("../run-page/loop-run-needs-you-card");
const { LOOP_NEEDS_YOU_ANCHOR_ID } = await import("../run-page/loop-run-briefing-constants");
const { LoopRunBriefing } = await import("../run-page/loop-run-briefing");
const { buildBriefingView } = await import("../../lib/loop-run-briefing-view");
const { projectLoopRequest } = await import("../../lib/loop-request-model");
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
    expect(dialog).toHaveTextContent("r-7c4e19 is running · generation 2");
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
    expect(dialog).toHaveTextContent("r-7c4e19 is watching · generation 4");
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

    expect(screen.getByTestId("loop-run-about-last-woke")).toHaveTextContent("Last woke");
    const best = screen.getByRole("link", { name: "Best result · Gen 1 · 0.70" });
    expect(best).toHaveAttribute("href", "#loop-generation-1");
    await userEvent.click(best);
    expect(onOpenGeneration).toHaveBeenCalledWith(1);
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
    expect(failed).toHaveTextContent("could not be read");
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

// A staged scenario is evidence about the production reads, so its two reads
// have to agree the way the daemon's do. They did not: the briefing was built
// independently of the run record and re-synchronised on status and progress
// alone, which left the spend free to drift — every register capture showed
// 82.4k tokens over a run recording 68k, and every one of those captures passed
// its visual contract. Sweeping the module rather than a list means a scenario
// added later cannot quietly opt out.
describe("staged register scenarios", () => {
  // Every module that exports staged scenarios, so a story added to any of them
  // is covered without anyone remembering to add it here.
  const scenarios = [
    ...Object.entries(registerFixtures),
    ...Object.entries(visualContractFixtures),
    ...Object.entries(lifecycleFixtures),
    ...Object.entries(graphEngFixtures),
    ...Object.entries(pageFixtures),
    ...Object.entries(metricFixtures),
  ].filter(
    (candidate): candidate is [string, () => LoopRunStoryScenario] =>
      candidate[0].endsWith("Scenario") &&
      typeof candidate[1] === "function" &&
      candidate[1].length === 0
  );

  it("Should match at least one staged scenario", () => {
    // The only thing this guard owes: a filter that matched nothing would make
    // every case below pass without asserting anything. How many fixtures the
    // modules happen to export is not an invariant — freezing a count here would
    // fail on a scenario being added or retired, neither of which can break read
    // agreement.
    expect(scenarios).not.toHaveLength(0);
  });

  // A scenario that stages events has a history, and the story pane reads the
  // durable timeline rather than those events — so several contract rows
  // captured "Nothing has happened in this run yet." over a run several rounds
  // deep, and passed. The read is derived from the scenario's own events now,
  // which is what the daemon does with them.
  it.each(scenarios)("Should give %s a story when its events say it has one", (_name, build) => {
    const scenario = build();
    if (scenario.frames.length === 0) return;

    expect(buildScenarioProps(scenario).registers.beats.length).toBeGreaterThan(0);
  });

  it.each(scenarios)("Should keep %s's briefing agreeing with its run", (_name, build) => {
    const { run, briefing } = build();

    expect(briefing.run_id).toBe(run.id);
    expect(briefing.status).toBe(run.status);
    expect(briefing.progress).toEqual(run.progress);
    // Tokens are the run record's own number, never a second opinion about it.
    expect(briefing.usage.tokens).toBe(run.tokens_used);
    // A budget percentage over a run with no budget would be a division by zero
    // rendered as a fact.
    expect(briefing.usage.budget_used_pct === undefined).toBe(run.budget_tokens === 0);
  });
});

// Truthful UI: the page must not offer a control the runtime cannot honour, and
// must not paint a deliberate cancellation as a success.
describe("run-page affordances that have to be real", () => {
  it("Should print a produced artifact's ref as evidence, never as an Open link", () => {
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
    expect(screen.getByTestId("loop-run-artifact-ref-post.md")).toHaveTextContent(
      "sha256:2f81c4a9"
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
