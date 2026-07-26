import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";

import { storybookMswParameters } from "@/storybook/msw";
import { aghApiMock } from "@/storybook/openapi-msw";
import { CenteredSurface } from "@/storybook/story-layout";
import type { AgentEventPayload } from "@/systems/session";

import { ClarificationDataPart } from "../clarification-data-part";

const pendingClarification: AgentEventPayload = {
  type: "clarify",
  session_id: "sess-payout-review",
  turn_id: "clarify:req-release-path",
  raw: {
    status: "pending",
    request: {
      request_id: "req-release-path",
      session_id: "sess-payout-review",
      question: "Which release path should the payout migration use?",
      choices: ["Canary", "Immediate"],
      asked_at: "2026-07-19T20:00:00Z",
      deadline: "2026-07-19T20:05:00Z",
    },
    at: "2026-07-19T20:00:00Z",
  },
};

const meta: Meta<typeof ClarificationDataPart> = {
  title: "systems/session/components/ClarificationDataPart",
  component: ClarificationDataPart,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Timeline renderer for durable clarification events, including recoverable live-read failures.",
      },
    },
  },
  args: {
    data: pendingClarification,
    sessionId: "sess-payout-review",
    workspaceId: "ws-risk",
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

/** Failed live authority read preserves the durable question and offers an explicit retry. */
export const LiveReadFailed: Story = {
  args: {},
  parameters: storybookMswParameters({
    session: [
      aghApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}/clarifications", () =>
        HttpResponse.json({ error: "clarification broker unavailable" }, { status: 503 })
      ),
    ],
  }),
};
