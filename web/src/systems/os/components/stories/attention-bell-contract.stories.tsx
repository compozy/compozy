import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { CenteredSurface } from "@/storybook/story-layout";

import type { OsAttentionSections, OsSessionAttentionRow } from "../../lib/attention-model";
import { AttentionBell } from "../attention-bell";

const NEEDS_YOU: OsSessionAttentionRow = {
  kind: "session",
  id: "session-needs-you",
  title: "Checkout launch review",
  agentName: "codex",
  workspaceId: "workspace-compozy",
  workspaceLabel: "compozy",
  badge: "waiting-for-input",
  reason: "Which release channel should ship?",
  changedAt: "2026-08-16T12:10:00Z",
  muted: false,
  stale: false,
};

const FINISHED: OsSessionAttentionRow = {
  ...NEEDS_YOU,
  id: "session-finished",
  title: "Runtime parity pass",
  workspaceId: "workspace-infra",
  workspaceLabel: "infra",
  badge: "done",
  reason: "finished while you were away",
};

type BellState = "needs-you" | "finished" | "quiet" | "disconnected" | "muted";

function AttentionBellContract({ state }: { state: BellState }) {
  const sections: OsAttentionSections = {
    needsYou:
      state === "needs-you" || state === "disconnected"
        ? [NEEDS_YOU]
        : state === "muted"
          ? [{ ...NEEDS_YOU, muted: true }]
          : [],
    finished: state === "finished" ? [FINISHED] : [],
  };
  return (
    <div className="w-100 rounded-lg border border-line bg-canvas-soft p-2 shadow-popover">
      <AttentionBell
        sections={sections}
        sessionsDisconnected={state === "disconnected"}
        tasksDisconnected={false}
        loading={false}
        onSelect={fn()}
      />
    </div>
  );
}

const meta: Meta<typeof AttentionBellContract> = {
  title: "systems/os/components/AttentionBellContract",
  component: AttentionBellContract,
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

export const NeedsYouOnly: Story = { args: { state: "needs-you" } };
export const FinishedOnly: Story = { args: { state: "finished" } };
export const Quiet: Story = { args: { state: "quiet" } };
export const Disconnected: Story = { args: { state: "disconnected" } };
export const MutedWorkspace: Story = { args: { state: "muted" } };
