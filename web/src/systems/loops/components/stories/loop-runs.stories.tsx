import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { StorySurface } from "@/storybook/story-layout";

import { LoopRunsView } from "../runs/loop-runs-view";
import type { LoopOutcomeValue } from "../runs/loop-runs-outcome-filter";
import { loopRunFixtures } from "../../mocks/fixtures";

const meta: Meta<typeof LoopRunsView> = {
  title: "systems/loops/components/LoopRuns",
  component: LoopRunsView,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

function RunsHarness() {
  const [outcome, setOutcome] = useState<LoopOutcomeValue>("all");
  const pendingRequestCounts = new Map([[loopRunFixtures[0].id, 2]]);
  return (
    <StorySurface className="p-8">
      <div className="mx-auto max-w-[1320px]">
        <LoopRunsView
          onOutcomeChange={setOutcome}
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
