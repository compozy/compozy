import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";

import type { ClarifyEventRequest, ClarifyEventView } from "../../types";
import { ClarificationReceipt } from "../clarification-receipt";

const request: ClarifyEventRequest = {
  request_id: "req-1",
  workspace_id: "ws_alpha",
  session_id: "sess-001",
  agent_name: "mock",
  question: "Which environment should the migration target first?",
  choices: ["Staging", "Production"],
  asked_at: "2026-07-16T00:00:00Z",
  deadline: "2026-07-16T00:05:00Z",
};

function view(
  status: ClarifyEventView["status"],
  answer: ClarifyEventView["answer"]
): ClarifyEventView {
  return { status, requestId: request.request_id, request, answer, at: "2026-07-16T00:00:10Z" };
}

const meta: Meta<typeof ClarificationReceipt> = {
  title: "systems/session/components/ClarificationReceipt",
  component: ClarificationReceipt,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Truthful, non-interactive receipt for a terminal clarification. Distinct per outcome and never invents an answer for a timeout.",
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

/** Resolved — shows the chosen answer. */
export const Resolved: Story = {
  args: { view: view("resolved", { choice: 1, text: "", fallback: false }) },
};

/** Timed out — the fallback sentinel, with no invented answer. */
export const TimedOut: Story = {
  args: { view: view("timed_out", { choice: null, text: "", fallback: true }) },
};

/** Canceled — neutral tone, no answer. */
export const Canceled: Story = {
  args: { view: view("canceled", null) },
};
