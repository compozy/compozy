import type { Meta, StoryObj } from "@storybook/react-vite";

import type { SessionComposerFeedback } from "@/components/assistant-ui/hooks/session-busy-input-store";
import { SessionComposerFeedbackNote } from "@/components/assistant-ui/session-composer-feedback-note";
import type { SessionSendOutcome } from "@/systems/session";

const outcome = (overrides: Partial<SessionSendOutcome>): SessionSendOutcome => ({
  disposition: "steering",
  entryId: "inp_4d9",
  idempotencyKey: "idk_1f77",
  messageId: "msg_01k4",
  queuePosition: null,
  replayed: false,
  steerDelivery: "injected",
  turnId: "t_9f2",
  ...overrides,
});

const disposition = (value: SessionSendOutcome): SessionComposerFeedback => ({
  action: value.disposition === "queued" ? "queue" : "steer",
  draftText: "",
  kind: "disposition",
  outcome: value,
});

/**
 * The one-line answer inside the composer card after a busy send
 * (`sessions-stability-composer.html` §04–05): the glyph names the verb, the
 * bold phrase the outcome, the mono suffix the daemon's own delivery word or
 * entry id. Refusals lead with "Not sent" under the only tone on the row.
 */
const meta: Meta<typeof SessionComposerFeedbackNote> = {
  title: "systems/session/components/assistant-ui/SessionComposerFeedbackNote",
  component: SessionComposerFeedbackNote,
  parameters: { layout: "centered" },
  decorators: [
    Story => (
      <div className="flex w-[560px] flex-col gap-3 rounded-lg border border-line bg-elevated px-3.5 py-3">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-01 — the live turn took the guidance. */
export const SteerInjected: Story = {
  args: { feedback: disposition(outcome({ steerDelivery: "injected" })) },
};

/** VC-02 — accepted behind a blocking tool. */
export const SteerPendingInjection: Story = {
  args: { feedback: disposition(outcome({ steerDelivery: "pending_injection" })) },
};

/** VC-03 — the agent cannot take guidance mid-turn; the turn was interrupted and replaced. */
export const SteerInterruptFallback: Story = {
  args: { feedback: disposition(outcome({ steerDelivery: "interrupt_fallback" })) },
};

/** VC-04 — queued with its position and durable entry id. */
export const Queued: Story = {
  args: {
    feedback: disposition(
      outcome({ disposition: "queued", entryId: "inp_4d8", queuePosition: 2, steerDelivery: null })
    ),
  },
};

export const Interrupting: Story = {
  args: {
    feedback: {
      action: "interrupt",
      draftText: "",
      kind: "disposition",
      outcome: outcome({ disposition: "interrupting", steerDelivery: null }),
    },
  },
};

/** The turn ended as the send landed: it became the next message. */
export const TurnEnded: Story = {
  args: { feedback: disposition(outcome({ disposition: "direct", steerDelivery: null })) },
};

/** VC-05 — a stale-fence refusal: reason, code, and the draft back in the field. */
export const RefusedTurnChanged: Story = {
  args: {
    feedback: {
      action: "steer",
      draftText: "Only touch the lifecycle tests, skip the store package",
      kind: "refusal",
      refusal: {
        attachmentCount: 0,
        code: "active_turn_mismatch",
        currentTurnId: "t_9f3",
        message: null,
      },
    },
  },
};

export const RefusedFilesOnSteer: Story = {
  args: {
    feedback: {
      action: "steer",
      draftText: "review these",
      kind: "refusal",
      refusal: {
        attachmentCount: 2,
        code: "steer_attachments_unsupported",
        currentTurnId: null,
        message: null,
      },
    },
  },
};

export const RefusedNotDelivered: Story = {
  args: {
    feedback: {
      action: "queue",
      draftText: "ship it",
      kind: "refusal",
      refusal: { attachmentCount: 0, code: "not_delivered", currentTurnId: null, message: null },
    },
  },
};
