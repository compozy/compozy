import type { ReactNode } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import {
  storyAgentNames,
  storySessionIds,
  storyWorkspaceIds,
  storyWorkspacePaths,
} from "@/storybook/fintech-scenario";
import { sessionFixtures } from "@/systems/session/mocks";
import type { SessionLifecycleActionHandlers, SessionPayload } from "@/systems/session";
import { PanelSurface } from "@/storybook/story-layout";

import { AgentSessionsList } from "../agent-sessions-list";

const fraudSessions: SessionPayload[] = sessionFixtures.filter(
  session => session.agent_name === storyAgentNames.fraud
);

const fallbackFraudSession: SessionPayload = {
  id: storySessionIds.fraud,
  name: "Payout hold triage",
  agent_name: storyAgentNames.fraud,
  runtime: {
    status: "ready",
    transition: "initial_bind",
    effective: { provider: "claude" },
    selection_revision: 0,
  },
  workspace_id: storyWorkspaceIds.risk,
  workspace_path: storyWorkspacePaths.risk,
  state: "active",
  badge: "idle",
  attachable: true,
  archived_at: null,
  available_commands: [],
  created_at: "2026-04-17T16:00:00Z",
  updated_at: "2026-04-17T18:10:00Z",
};

const failureBaseSession = fraudSessions[0] ?? fallbackFraudSession;

const fraudSessionsWithFailure: SessionPayload[] = [
  ...fraudSessions,
  {
    ...failureBaseSession,
    id: "sess_fraud_failed",
    name: "Settlement export retry",
    state: "stopped",
    badge: "stopped",
    attachable: false,
    stop_reason: "agent_crashed",
    failure: {
      kind: "agent_crashed",
      summary: "partner settlement export terminated unexpectedly",
    },
    activity: {
      elapsed_ms: 142_000,
      elapsed_seconds: 142,
      idle_seconds: 0,
      iteration_current: 4,
      iteration_max: 6,
      last_activity_at: "2026-04-17T18:42:00Z",
      last_activity_kind: "tool",
      last_progress_at: "2026-04-17T18:42:00Z",
    },
    updated_at: "2026-04-17T18:42:00Z",
  },
];

const readyFraudSessions: SessionPayload[] = fraudSessions.length
  ? fraudSessions.map((session, index) =>
      index === 0 ? { ...session, name: "Checkout Retry Fencing" } : session
    )
  : [{ ...fallbackFraudSession, name: "Checkout Retry Fencing" }];

const archivedFraudSessions: SessionPayload[] = [
  {
    ...failureBaseSession,
    id: "sess_fraud_archived",
    name: "Historical settlement audit",
    state: "stopped",
    badge: "stopped",
    archived_at: "2026-08-04T12:00:00Z",
  },
];

const sessionActions: SessionLifecycleActionHandlers = {
  pendingAction: null,
  pendingSessionId: null,
  onArchive: fn(),
  onDelete: fn(),
  onStop: fn(),
  onUnarchive: fn(),
};

const meta: Meta<typeof AgentSessionsList> = {
  title: "systems/agent/components/AgentSessionsList",
  component: AgentSessionsList,
  parameters: {
    layout: "fullscreen",
    router: { kind: "stub" as const },
  },
  args: {
    agentName: storyAgentNames.fraud,
    sessions: fraudSessions,
    archivedSessions: archivedFraudSessions,
    archivedTotal: archivedFraudSessions.length,
    sessionActions,
    status: "ready",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

interface FrameProps {
  children: ReactNode;
}

function Frame({ children }: FrameProps) {
  return (
    <PanelSurface>
      <div className="flex w-full flex-col gap-4 px-6 py-5">{children}</div>
    </PanelSurface>
  );
}

/**
 * Default -- sessions sorted by last activity with state-gated row actions and a closed archive.
 */
export const Default: Story = {
  args: { sessions: readyFraudSessions },
  render: args => (
    <Frame>
      <AgentSessionsList {...args} />
    </Frame>
  ),
};

/**
 * One session has a failure payload -- surfaces the FAILED chip via the danger tone.
 */
export const WithFailure: Story = {
  args: { sessions: fraudSessionsWithFailure },
  render: args => (
    <Frame>
      <AgentSessionsList {...args} />
    </Frame>
  ),
};

/**
 * Empty state -- no sessions for the agent yet.
 */
export const Empty: Story = {
  args: { sessions: [] },
  render: args => (
    <Frame>
      <AgentSessionsList {...args} />
    </Frame>
  ),
};

/**
 * Loading skeleton while sessions are being fetched.
 */
export const Loading: Story = {
  args: { status: "loading", sessions: [] },
  render: args => (
    <Frame>
      <AgentSessionsList {...args} />
    </Frame>
  ),
};

/**
 * Error fallback when the sessions query rejects.
 */
export const Error: Story = {
  args: { status: "error", sessions: [] },
  render: args => (
    <Frame>
      <AgentSessionsList {...args} />
    </Frame>
  ),
};
