import type { Meta, StoryObj } from "@storybook/react-vite";

import { StorySurface } from "@/storybook/story-layout";

import { LoopRunsView } from "../runs/loop-runs-view";
import type { LoopOutcomeValue } from "../../lib/loop-runs-view";
import type { LoopRun } from "../../types";
import { loopRunFixtures } from "../../mocks/fixtures";

const meta: Meta<typeof LoopRunsView> = {
  title: "systems/loops/components/LoopRuns",
  component: LoopRunsView,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * When the degraded read landed, near the board's staged 40s.
 *
 * Stamped once when the module loads rather than on every render: a pinned ISO
 * would age into "3w ago" and reading the clock during render is impure. Each
 * capture navigates to a fresh page, so the sentence lands within seconds of the
 * staged window.
 */
const STALE_READ_AT = new Date(Date.now() - 40_000).toISOString();

function RunsHarness({
  outcome = "all",
  runs = loopRunFixtures,
  isReconnecting = false,
}: {
  outcome?: LoopOutcomeValue;
  runs?: readonly LoopRun[];
  isReconnecting?: boolean;
}) {
  return (
    <StorySurface className="p-8">
      <div className="mx-auto max-w-[1320px]">
        <LoopRunsView
          isReconnecting={isReconnecting}
          lastReadAt={isReconnecting ? STALE_READ_AT : undefined}
          // The host wires both branches (`loop-runs-location.tsx`): browse the
          // catalog when nothing has ever run, clear the filter otherwise. A
          // story that omitted it captured an empty state with no action at all.
          onEmptyAction={() => undefined}
          onRetry={() => undefined}
          outcome={outcome}
          runs={runs}
        />
      </div>
    </StorySurface>
  );
}

/**
 * Thirty runs of the same shapes. The roster has to stay readable at the scale
 * an actual workspace reaches, and grouping is what keeps it readable.
 */
const manyRuns: readonly LoopRun[] = Array.from({ length: 30 }, (_, index) => {
  const seed = loopRunFixtures[index % loopRunFixtures.length];
  return { ...seed, id: `${seed.id}-${index + 1}` };
});

export const Default: Story = {
  render: () => <RunsHarness />,
};

/** The toolbar outcome chip narrows the partition to one status. */
export const OutcomeFiltered: Story = {
  render: () => <RunsHarness outcome="done" />,
};

/** No fixture run is `watching`, so the outcome filter reaches the truthful empty state. */
export const OutcomeFilteredEmpty: Story = {
  render: () => <RunsHarness outcome="watching" />,
};

/** VC-34 · a workspace that has never run a loop explains how to start one. */
export const EmptyWorkspace: Story = {
  render: () => <RunsHarness runs={[]} />,
};

/** VC-35 · thirty runs: grouping is what keeps the roster readable at scale. */
export const DozensActive: Story = {
  render: () => <RunsHarness runs={manyRuns} />,
};

/** VC-36 · a dropped stream is not an empty workspace and never borrows its words. */
export const TransportDegraded: Story = {
  render: () => <RunsHarness isReconnecting />,
};
