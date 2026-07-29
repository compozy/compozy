import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import type { AgentEventPayload } from "@/systems/session";

import { ClarificationDataPart } from "../clarification-data-part";

function clarifyEvent(overrides: Record<string, unknown>): AgentEventPayload {
  return {
    type: "clarify",
    session_id: "sess-payout-review",
    turn_id: "clarify:req-release-path",
    raw: {
      request: {
        request_id: "req-release-path",
        session_id: "sess-payout-review",
        question: "Which release path should the payout migration use?",
        choices: ["Canary", "Immediate"],
        asked_at: "2026-07-19T20:00:00Z",
        deadline: "2026-07-19T20:05:00Z",
      },
      at: "2026-07-19T20:05:00Z",
      ...overrides,
    },
  };
}

const meta: Meta<typeof ClarificationDataPart> = {
  title: "systems/session/components/ClarificationDataPart",
  component: ClarificationDataPart,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Transcript record for a clarify lifecycle event. Pending questions live on the composer decision dock — the transcript renders nothing until the lifecycle turns terminal, then keeps a one-line receipt from durable evidence.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="w-full max-w-xl">
          <Story />
        </div>
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

export const AnsweredReceipt: Story = {
  args: {
    data: clarifyEvent({ status: "resolved", answer: { choice: 0 } }),
  },
};

export const TimedOutReceipt: Story = {
  args: {
    data: clarifyEvent({ status: "timed_out", answer: null }),
  },
};

export const CanceledReceipt: Story = {
  args: {
    data: clarifyEvent({ status: "canceled", answer: null }),
  },
};

// A pending lifecycle renders nothing here — the composer dock owns the ask.
export const PendingRendersNothing: Story = {
  args: {
    data: clarifyEvent({ status: "pending", answer: null }),
  },
};
