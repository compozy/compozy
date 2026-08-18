import type { Meta, StoryObj } from "@storybook/react-vite";

import { StorySurface } from "@/storybook/story-layout";

import { LoopRunsView } from "../runs/loop-runs-view";
import type { LoopOutcomeValue } from "../../lib/loop-runs-view";
import { loopRunFixtures } from "../../mocks/fixtures";

const meta: Meta<typeof LoopRunsView> = {
  title: "systems/loops/components/LoopRuns",
  component: LoopRunsView,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

function RunsHarness({ outcome = "all" }: { outcome?: LoopOutcomeValue }) {
  const pendingRequestCounts = new Map([[loopRunFixtures[0].id, 2]]);
  return (
    <StorySurface className="p-8">
      <div className="mx-auto max-w-[1320px]">
        <LoopRunsView
          outcome={outcome}
          pendingRequestCounts={pendingRequestCounts}
          runs={loopRunFixtures}
        />
      </div>
    </StorySurface>
  );
}

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
