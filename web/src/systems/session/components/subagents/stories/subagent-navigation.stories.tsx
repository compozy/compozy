import type { ReactNode } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, waitFor, within } from "storybook/test";
import { fn } from "storybook/test";

import { StateGlyph, Toaster, type StateGlyphState } from "@compozy/ui";

import { SessionInspectorSubagentsSection } from "../session-inspector-subagents-section";
import { SubagentChip } from "../subagent-chip";
import type { SubagentCounts } from "../subagent-format";
import { cardStates, migrationReviews, roster } from "./subagent-story-fixtures";

/** Lean stand-in for a 280px sidebar row; the chip sits in its trailing slot. */
function RowHost({
  label,
  title,
  glyph,
  children,
}: {
  label: string;
  title: string;
  glyph: StateGlyphState;
  children?: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-2">
      <span className="font-mono text-mono-id text-faint">{label}</span>
      <div className="flex w-70 items-start gap-1 rounded-lg border border-line bg-sidebar p-2">
        <div className="grid min-w-0 flex-1 grid-cols-[12px_minmax(0,1fr)] items-start gap-2 px-2 py-1.5">
          <StateGlyph size="sm" state={glyph} className="mt-1" />
          <span className="flex min-w-0 flex-col">
            <span className="truncate text-body text-fg">{title}</span>
            <span className="truncate text-eyebrow font-medium text-muted">dev-loop</span>
          </span>
        </div>
        <span className="flex items-center pt-1">{children}</span>
      </div>
    </div>
  );
}

const counts = (live: number, total: number, failed = 0, attention = 0): SubagentCounts => ({
  live,
  total,
  failed,
  attention,
});

/** Nav VC-02: the parent-row chip states. */
function ChipStates() {
  return (
    <div className="grid w-fit grid-cols-2 gap-x-8 gap-y-6 p-8">
      <RowHost label="live" title="Ship checkout v2" glyph="running">
        <SubagentChip counts={counts(2, 10)} parentTurnRunning preview={roster} onOpen={fn()} />
      </RowHost>
      <RowHost
        label="attention · a subagent waits for you"
        title="Ship checkout v2"
        glyph="running"
      >
        <SubagentChip
          counts={counts(3, 10, 0, 1)}
          parentTurnRunning
          preview={roster}
          onOpen={fn()}
        />
      </RowHost>
      <RowHost label="failed, none live · total only" title="Upgrade payment SDK" glyph="stopped">
        <SubagentChip counts={counts(0, 6, 2)} parentTurnRunning={false} onOpen={fn()} />
      </RowHost>
      <RowHost label="delegated · parent idle" title="Refactor cart totals" glyph="delegated">
        <SubagentChip counts={counts(2, 4)} parentTurnRunning={false} onOpen={fn()} />
      </RowHost>
      <RowHost label="hidden · all settled cleanly" title="Draft Q4 roadmap" glyph="done">
        <SubagentChip counts={counts(0, 3)} parentTurnRunning={false} />
      </RowHost>
    </div>
  );
}

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
      <Rail label="live + failed + previous · 10 subagents">
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

export const Chip: Story = { render: () => <ChipStates /> };
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
