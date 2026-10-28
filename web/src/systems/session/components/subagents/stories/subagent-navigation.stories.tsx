import type { ReactNode } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, waitFor, within } from "storybook/test";
import { fn } from "storybook/test";

import { Toaster } from "@compozy/ui";

import { SessionInspectorSubagentsSection } from "../session-inspector-subagents-section";
import { cardStates, migrationReviews, roster } from "./subagent-story-fixtures";

/** Lean stand-in for the 320px inspector rail. */
function Rail({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-2">
      <span className="font-mono text-mono-id text-faint">{label}</span>
      <aside className="w-80 rounded-lg border border-line bg-canvas-soft p-4">{children}</aside>
    </div>
  );
}

const neverStop = () => new Promise<never>(() => undefined);
const failingStop = () =>
  new Promise<never>((_, reject) => setTimeout(() => reject(new Error("stop failed")), 600));

/** Nav VC-01/03: the inspector Subagents section. */
function RosterStates() {
  return (
    <div className="grid w-fit grid-cols-3 items-start gap-x-6 gap-y-8 p-8">
      <Toaster />
      <Rail label="live + failed + previous · 11 subagents">
        <SessionInspectorSubagentsSection subagents={roster} onOpen={fn()} onStop={neverStop} />
      </Rail>
      <Rail label="live only">
        <SessionInspectorSubagentsSection
          subagents={[
            { ...cardStates.runningProgress, created_at: cardStates.queued.created_at },
            cardStates.queued,
          ]}
          onOpen={fn()}
          onStop={neverStop}
        />
      </Rail>
      <Rail label="live + previous · open">
        <SessionInspectorSubagentsSection
          subagents={[
            cardStates.failed,
            cardStates.runningProgress,
            cardStates.completed,
            cardStates.canceled,
          ]}
          onOpen={fn()}
          onStop={neverStop}
          defaultPreviousOpen
        />
      </Rail>
      <Rail label="paging · 6 rows, then Show 12 more">
        <SessionInspectorSubagentsSection
          subagents={migrationReviews(20)}
          onOpen={fn()}
          defaultPreviousOpen
        />
      </Rail>
      <Rail label="stop pending · press Stop">
        <SessionInspectorSubagentsSection
          subagents={[cardStates.runningProgress, cardStates.running]}
          onOpen={fn()}
          onStop={neverStop}
        />
      </Rail>
      <Rail label="stop failed · press Stop, toast">
        <SessionInspectorSubagentsSection
          subagents={[cardStates.runningProgress]}
          onOpen={fn()}
          onStop={failingStop}
        />
      </Rail>
    </div>
  );
}

const meta: Meta = {
  title: "systems/session/components/subagents/Navigation",
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const InspectorRoster: Story = { render: () => <RosterStates /> };

/** Stop pending: the running ring replaces the square while the cancel is in flight. */
export const InspectorStopPending: Story = {
  tags: ["play-fn"],
  render: () => (
    <div className="p-8">
      <Rail label="stop pending">
        <SessionInspectorSubagentsSection
          subagents={[cardStates.runningProgress, cardStates.running]}
          onOpen={fn()}
          onStop={neverStop}
        />
      </Rail>
    </div>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const [stop] = await canvas.findAllByRole("button", { name: "Stop subagent", hidden: true });
    await userEvent.click(stop!);
    await waitFor(() =>
      expect(canvas.getByRole("button", { name: "Stopping subagent" })).toBeDisabled()
    );
  },
};
