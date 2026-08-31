import type { Meta, StoryObj } from "@storybook/react-vite";
import { userEvent, within } from "storybook/test";

import { Eyebrow } from "@compozy/ui";

import { LoopNodeAmendDialog } from "../run-page/loop-node-amend-dialog";
import { LoopNodeControlDialog } from "../run-page/loop-node-control-dialog";
import { LoopNodeControlMenu } from "../run-page/loop-node-control-menu";
import { LoopNodeRerunDialog } from "../run-page/loop-node-rerun-dialog";
import { LoopRunControlDialog } from "../run-page/loop-run-control-dialog";
import { LoopRunControls } from "../run-page/loop-run-controls";
import { LoopRunOverflowMenu } from "../run-page/loop-run-overflow-menu";
import type { LoopNodeVerb } from "../../lib/loop-node-controls";
import type { LoopNodeLifecycle } from "../../lib/loop-node-lifecycle";
import type { LoopRerunSet } from "../../lib/loop-rerun-set";

/**
 * Visual Contract capture target for node + run controls (VC-R3).
 *
 * Every menu here is the real `LoopNodeControlMenu` driven by a real
 * `LoopNodeLifecycle`, opened through a play function — so what the capture
 * shows is exactly what `loopNodeVerbs` derives from that node's payload, not a
 * mock-up of it.
 */

function node(overrides: Partial<LoopNodeLifecycle> & { nodeId: string }): LoopNodeLifecycle {
  return {
    label: overrides.nodeId.replace(/[_-]+/g, " "),
    state: null,
    parked: false,
    paused: false,
    pauseProvenance: null,
    quarantined: false,
    quarantinedAt: null,
    quarantineEntry: null,
    attentionFlag: "",
    attentionReason: "",
    attentionProducerNodeId: "",
    cancelState: "",
    cancelProvenance: null,
    lastEvidenceAt: null,
    deathResumeStreak: 0,
    revision: 1,
    waits: [],
    attempt: null,
    nextAttemptAt: null,
    failureClass: "",
    disposition: "",
    outputStatus: "running",
    generation: 2,
    sessionId: null,
    ...overrides,
    itemIndex: overrides.itemIndex ?? null,
  };
}

const RUNNING = node({ nodeId: "task_04", outputStatus: "running" });
const PAUSED = node({ nodeId: "task_03", paused: true, state: "paused" });
const QUARANTINED = node({
  nodeId: "task_03",
  quarantined: true,
  state: "quarantined",
  quarantinedAt: "2026-08-03T14:52:00Z",
});
const WAITING = node({
  nodeId: "task_04",
  state: "waiting",
  waits: [
    {
      nodeId: "task_04",
      generation: 2,
      itemIndex: 0,
      kind: "event",
      claimState: "waiting",
      escalationCursor: 0,
      admissionFailures: 0,
      ageSeconds: 18 * 60,
      createdAt: "2026-08-03T14:20:00Z",
      expect: { type: "object", required: ["env"] },
    },
  ],
});

const MENU_CASES: { caption: string; node: LoopNodeLifecycle; testId: string }[] = [
  {
    caption: "running — pause/cancel; no resume, no requeue",
    node: RUNNING,
    testId: "task_04",
  },
  {
    caption: "paused — the three resume modes; requeue still absent",
    node: PAUSED,
    testId: "task_03",
  },
  {
    caption: "quarantined — requeue is the recovery verb; resume is not offered",
    node: QUARANTINED,
    testId: "task_03",
  },
  {
    caption: "waiting — resume takes a payload validated against the wait",
    node: WAITING,
    testId: "task_04",
  },
];

const meta: Meta<typeof LoopNodeControlMenu> = {
  title: "systems/loops/components/LoopNodeControls",
  component: LoopNodeControlMenu,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

function MenuCase({ caption, node: subject }: { caption: string; node: LoopNodeLifecycle }) {
  return (
    <div className="flex flex-col gap-2">
      <p className="font-mono text-pill-group-badge text-faint">{caption}</p>
      <div className="flex items-center justify-between rounded-lg border border-line bg-canvas-soft px-4 py-3">
        <span className="text-ws-name font-medium text-fg-strong">{subject.nodeId}</span>
        <LoopNodeControlMenu node={subject} onVerb={() => {}} runStatus="running" />
      </div>
    </div>
  );
}

/** The four gated verb sets, each rendered from its own node payload. */
export const NodeMenus: Story = {
  args: {},
  render: () => (
    <div className="min-h-dvh bg-canvas p-6">
      <Eyebrow className="text-accent">Node row menu</Eyebrow>
      <p className="mt-1 mb-4 text-small-body text-muted">
        The verb set is whatever the node&apos;s payload allows — nothing more.
      </p>
      <div className="grid gap-5 min-[900px]:grid-cols-2">
        {MENU_CASES.map(entry => (
          <MenuCase caption={entry.caption} key={entry.caption} node={entry.node} />
        ))}
      </div>
    </div>
  ),
};

/** The same four sets with the quarantined menu open, so its items are visible. */
export const NodeMenuOpen: Story = {
  args: {},
  render: () => (
    <div className="min-h-dvh bg-canvas p-6">
      <Eyebrow className="text-accent">Node row menu — quarantined</Eyebrow>
      <p className="mt-1 mb-4 text-small-body text-muted">
        Requeue is the recovery verb; resume is never offered for a set-aside lane.
      </p>
      <div className="max-w-md">
        <MenuCase caption="quarantined" node={QUARANTINED} />
      </div>
    </div>
  ),
  play: async ({ canvasElement }: { canvasElement: HTMLElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByTestId("loop-node-menu-trigger-task_03"));
  },
};

function DialogCase({ verb, subject }: { verb: LoopNodeVerb; subject: LoopNodeLifecycle }) {
  return (
    <LoopNodeControlDialog
      onConfirm={() => {}}
      onOpenChange={() => {}}
      request={{ verb, node: subject }}
    />
  );
}

/** Pause asks what happens to the attempt already in flight. */
export const PauseConfirm: Story = {
  args: {},
  render: () => (
    <div className="min-h-dvh bg-canvas">
      <DialogCase subject={RUNNING} verb="pause" />
    </div>
  ),
};

/** Cancel closes the lane immediately and stops active work automatically. */
export const CancelConfirm: Story = {
  args: {},
  render: () => (
    <div className="min-h-dvh bg-canvas">
      <DialogCase subject={RUNNING} verb="cancel" />
    </div>
  ),
};

/** Requeue restates the quarantine state it is acting on. */
export const RequeueConfirm: Story = {
  args: {},
  render: () => (
    <div className="min-h-dvh bg-canvas">
      <DialogCase subject={QUARANTINED} verb="requeue" />
    </div>
  ),
};

/** A by-hand wait resume, with the payload the daemon will validate. */
export const WaitResumeConfirm: Story = {
  args: {},
  render: () => (
    <div className="min-h-dvh bg-canvas">
      <DialogCase subject={WAITING} verb="resume-wait" />
    </div>
  ),
};

/** The daemon's deterministic answer, rendered as information inside the dialog. */
export const DeterministicAnswer: Story = {
  args: {},
  render: () => (
    <div className="min-h-dvh bg-canvas">
      <LoopNodeControlDialog
        answer={{
          allowedTransitions: ["pause", "cancel"],
          detail: "task_03 isn't paused — it's running. From here you can pause or cancel it.",
          micro: "node_not_paused · state running",
          title: "Nothing to resume",
          tone: "info",
        }}
        onConfirm={() => {}}
        onOpenChange={() => {}}
        request={{ verb: "resume", node: PAUSED }}
      />
    </div>
  ),
};

/** The paused menu, open — the three resume modes with the `mode` each posts. */
export const ResumeModes: Story = {
  args: {},
  render: () => (
    <div className="min-h-dvh bg-canvas p-6">
      <Eyebrow className="text-accent">Node row menu — paused</Eyebrow>
      <p className="mt-1 mb-4 text-small-body text-muted">
        Three resume modes; requeue stays absent because a paused lane was never quarantined.
      </p>
      <div className="max-w-md">
        <MenuCase caption="paused" node={PAUSED} />
      </div>
    </div>
  ),
  play: async ({ canvasElement }: { canvasElement: HTMLElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByTestId("loop-node-menu-trigger-task_03"));
  },
};

/** The running menu, open — pause/cancel, with no resume and no requeue. */
export const RunningMenuOpen: Story = {
  args: {},
  render: () => (
    <div className="min-h-dvh bg-canvas p-6">
      <Eyebrow className="text-accent">Node row menu — running</Eyebrow>
      <p className="mt-1 mb-4 text-small-body text-muted">
        Nothing is parked, so no resume is offered; nothing was set aside, so no requeue.
      </p>
      <div className="max-w-md">
        <MenuCase caption="running" node={RUNNING} />
      </div>
    </div>
  ),
  play: async ({ canvasElement }: { canvasElement: HTMLElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByTestId("loop-node-menu-trigger-task_04"));
  },
};

/** The waiting menu, open — resume takes a payload validated against the wait. */
export const WaitingMenuOpen: Story = {
  args: {},
  render: () => (
    <div className="min-h-dvh bg-canvas p-6">
      <Eyebrow className="text-accent">Node row menu — waiting</Eyebrow>
      <p className="mt-1 mb-4 text-small-body text-muted">
        A by-hand resume stands in for the event this lane is waiting for.
      </p>
      <div className="max-w-md">
        <MenuCase caption="waiting" node={WAITING} />
      </div>
    </div>
  ),
  play: async ({ canvasElement }: { canvasElement: HTMLElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByTestId("loop-node-menu-trigger-task_04"));
  },
};

/** Run cancellation stays a first-class destructive button. */
export const RunCancel: Story = {
  args: {},
  render: () => (
    <div className="min-h-dvh bg-canvas p-6">
      <Eyebrow className="text-accent">Run controls</Eyebrow>
      <p className="mt-1 mb-4 text-small-body text-muted">
        Cancel closes the run immediately and stops active sessions automatically.
      </p>
      <div className="flex items-center gap-2 rounded-lg border border-line bg-canvas-soft px-4 py-3">
        <LoopRunControls
          onCancel={() => {}}
          onPause={() => {}}
          onResume={() => {}}
          status="running"
        />
        <LoopRunOverflowMenu loopName="review-and-fix" />
      </div>
    </div>
  ),
  play: async ({ canvasElement }: { canvasElement: HTMLElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByTestId("loop-run-more"));
  },
};

/** The production run cancel confirm, restating the run state it acts on. */
export const RunCancelConfirm: Story = {
  args: {},
  render: () => (
    <div className="min-h-dvh bg-canvas">
      <LoopRunControlDialog
        generation={2}
        onConfirm={() => {}}
        onOpenChange={() => {}}
        runId="r-7c4e19"
        status="running"
        verb="cancel"
      />
    </div>
  ),
};

const AMENDABLE = node({
  nodeId: "render-notes",
  paused: true,
  state: "paused",
  outputStatus: "succeeded",
  itemIndex: 0,
});
const RERUNNABLE = node({ nodeId: "apply-migration", outputStatus: "succeeded", itemIndex: 0 });

const NOTES_OUTPUT_SCHEMA = {
  type: "object",
  required: ["risk"],
  properties: {
    risk: { type: "string", enum: ["low", "medium", "high"] },
    summary: { type: "string" },
  },
};

const ROLLOUT_RERUN_SET: LoopRerunSet = {
  fromNode: "apply-migration",
  rerunNodes: ["apply-migration", "collect-rollout", "render-notes"],
  carriedNodes: ["services", "triage", "standard", "rollout"],
};

export const AmendDialog: Story = {
  args: {},
  render: () => (
    <div className="min-h-dvh bg-canvas">
      <LoopNodeAmendDialog
        node={AMENDABLE}
        onConfirm={() => {}}
        onOpenChange={() => {}}
        open
        originalOutput={{ risk: "high", summary: "Rollout for billing" }}
        outputSchema={NOTES_OUTPUT_SCHEMA}
      />
    </div>
  ),
};

export const AmendDialogValidationFailure: Story = {
  args: {},
  render: () => (
    <div className="min-h-dvh bg-canvas">
      <LoopNodeAmendDialog
        fieldErrors={{ risk: "risk must be one of low, medium, high." }}
        node={AMENDABLE}
        onConfirm={() => {}}
        onOpenChange={() => {}}
        open
        originalOutput={{ risk: "high", summary: "Rollout for billing" }}
        outputSchema={NOTES_OUTPUT_SCHEMA}
      />
    </div>
  ),
};

export const RerunDialog: Story = {
  args: {},
  render: () => (
    <div className="min-h-dvh bg-canvas">
      <LoopNodeRerunDialog
        node={RERUNNABLE}
        onConfirm={() => {}}
        onOpenChange={() => {}}
        open
        rerunSet={ROLLOUT_RERUN_SET}
      />
    </div>
  ),
};
