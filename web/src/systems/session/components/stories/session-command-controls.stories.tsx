import type { Meta, StoryObj } from "@storybook/react-vite";

import type { AgentEventPayload, RuntimeActivityPayload } from "@/systems/session/types";
import { PanelSurface } from "@/storybook/story-layout";

import { RuntimeActivityNotice } from "../runtime-activity-notice";

const runtimeActivity: RuntimeActivityPayload = {
  current_tool: "read_file",
  elapsed_ms: 184_000,
  elapsed_seconds: 184,
  idle_seconds: 12,
  last_activity_detail: "Reading task orchestration plan",
  last_activity_kind: "tool_call",
};

const progressEvent: AgentEventPayload = {
  type: "runtime_progress",
  text: "Still working",
  runtime: runtimeActivity,
  timestamp: "2026-04-17T18:12:00Z",
};

const warningEvent: AgentEventPayload = {
  ...progressEvent,
  type: "runtime_warning",
  text: "Runtime is waiting on provider output",
  runtime: { ...runtimeActivity, current_tool: undefined, idle_seconds: 74 },
};

const errorEvent: AgentEventPayload = {
  type: "error",
  error:
    '{"code":-32603,"message":"Internal error","data":{"error":"peer disconnected before response"}}',
  failure: {
    kind: "process_exit",
    summary: "peer disconnected before response",
  },
  timestamp: "2026-05-14T15:32:02Z",
};

const markerEvent: AgentEventPayload = {
  type: "transcript_marker.created",
  text: "1 file mutation failed and was not recovered in this turn. Verify the affected file before trusting completion claims.",
  marker: {
    kind: "transcript_marker.file_mutation_unverified",
    occurred_at: "2026-05-20T13:56:00Z",
    summary:
      "1 file mutation failed and was not recovered in this turn. Verify the affected file before trusting completion claims.",
    evidence: { failure_count: 1, paths: ["checkout/retry.go"] },
  },
  timestamp: "2026-05-20T13:56:00Z",
};

const meta: Meta<typeof RuntimeActivityNotice> = {
  title: "systems/session/components/RuntimeActivityNotice",
  component: RuntimeActivityNotice,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Runtime activity notices used by session creation and active session headers. Provider, model, and reasoning command selectors are documented under systems/runtime/CommandSelects.",
      },
    },
  },
  decorators: [
    Story => (
      <PanelSurface className="min-h-[420px] p-6">
        <Story />
      </PanelSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * RuntimeActivityNotice exposes progress, warning, marker, and failure states
 * without opening a live session.
 */
export const RuntimeActivity: Story = {
  args: {},
  render: () => (
    <div className="grid gap-4">
      <RuntimeActivityNotice event={progressEvent} />
      <RuntimeActivityNotice event={warningEvent} />
      <RuntimeActivityNotice event={markerEvent} />
      <RuntimeActivityNotice event={errorEvent} />
    </div>
  ),
};
