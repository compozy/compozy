import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { CenteredSurface } from "@/storybook/story-layout";
import type { SessionLifecycleActionHandlers, SessionPayload } from "@/systems/session";
import { SessionListRow } from "@/systems/session/components/session-list/session-list-row";
import { SessionStatusLine } from "@/systems/session/components/session-status-line";

const ACTIONS: SessionLifecycleActionHandlers = {
  pendingAction: null,
  pendingSessionId: null,
  onArchive: fn(),
  onDelete: fn(),
  onRename: fn(),
  onStop: fn(),
  onUnarchive: fn(),
};

function session(
  id: string,
  badge: SessionPayload["badge"],
  pendingInteractions: SessionPayload["pending_interactions"] = []
): SessionPayload {
  return {
    id,
    name: "Checkout launch review",
    agent_name: "codex",
    runtime: {
      status: "ready",
      transition: "initial_bind",
      effective: { provider: "codex" },
      selection_revision: 0,
    },
    workspace_id: "workspace-compozy",
    workspace_path: "/workspace/compozy",
    state: "active",
    badge,
    attachable: true,
    archived_at: null,
    available_commands: [],
    pending_interactions: pendingInteractions,
    created_at: "2026-08-16T12:00:00Z",
    updated_at: "2026-08-16T12:10:00Z",
  };
}

const PRECEDENCE = session("session-precedence", "waiting-for-auth", [
  {
    interaction_id: "permission-1",
    kind: "permission",
    provider_request_id: "provider-permission-1",
    turn_id: "turn-1",
    title: "Run the release command",
    decisions: ["allow-once", "reject-once"],
    status: "pending",
    created_at: "2026-08-16T12:10:00Z",
  },
  {
    interaction_id: "clarify-1",
    kind: "clarify",
    provider_request_id: "provider-clarify-1",
    turn_id: "turn-1",
    title: "Which release channel?",
    choices: ["Stable", "Preview"],
    status: "pending",
    created_at: "2026-08-16T12:10:01Z",
  },
]);

type SignalState = "precedence" | "handoff" | "waiting" | "done" | "running";

function SessionSignalContract({ state }: { state: SignalState }) {
  if (state === "precedence") {
    return (
      <div className="w-120 rounded-lg border border-line bg-canvas-soft p-2">
        <SessionListRow
          session={PRECEDENCE}
          onSelect={fn()}
          sessionActions={ACTIONS}
          testIdPrefix="contract-signal"
        />
      </div>
    );
  }
  if (state === "handoff") {
    return (
      <div className="grid w-120 gap-2 rounded-lg border border-line bg-canvas-soft p-2">
        <SessionListRow
          session={session("session-done", "done")}
          onSelect={fn()}
          sessionActions={ACTIONS}
          testIdPrefix="contract-signal"
        />
        <SessionListRow
          session={session("session-running", "running")}
          onSelect={fn()}
          sessionActions={ACTIONS}
          testIdPrefix="contract-signal"
        />
      </div>
    );
  }
  const badge = state === "waiting" ? "waiting-for-input" : state;
  return (
    <div className="w-120 rounded-lg border border-line bg-canvas-soft px-4 py-3">
      <SessionStatusLine session={session(`session-${state}`, badge)} />
    </div>
  );
}

const meta: Meta<typeof SessionSignalContract> = {
  title: "systems/os/components/SessionSignalContract",
  component: SessionSignalContract,
  parameters: { layout: "fullscreen" },
  decorators: [
    Story => (
      <CenteredSurface>
        <Story />
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Precedence: Story = { args: { state: "precedence" } };
export const DoneToRunningHandoff: Story = { args: { state: "handoff" } };
export const StatusWaitingForInput: Story = { args: { state: "waiting" } };
export const StatusDone: Story = { args: { state: "done" } };
export const StatusRunning: Story = { args: { state: "running" } };
