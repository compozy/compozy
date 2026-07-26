import type { Meta, StoryObj } from "@storybook/react-vite";

import { StorySurface, StoryTopbarHost } from "@/storybook/story-layout";

import { LoopDetailView } from "../detail/loop-detail";
import type { LoopBindingRow } from "../../lib/loop-bindings";
import { readLoopGraph } from "../../lib/loop-graph";
import {
  loopCatalogFixtures,
  loopDetailByName,
  loopEffectiveConfigFixture,
  loopRunFixtures,
} from "../../mocks/fixtures";

const meta: Meta<typeof LoopDetailView> = {
  title: "systems/loops/components/LoopDetail",
  component: LoopDetailView,
  decorators: [
    Story => (
      <StoryTopbarHost title="software-delivery">
        <Story />
      </StoryTopbarHost>
    ),
  ],
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component: "Loop detail states with run, edit, configure, and delete Topbar actions.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const loop = loopDetailByName.get("software-delivery")!;
const catalogEntry = loopCatalogFixtures.find(entry => entry.name === "software-delivery")!;
const recentRuns = loopRunFixtures.filter(run => run.loop_name === "software-delivery").slice(0, 5);
const config = {
  iteration_cap: 3,
  budget_on_exceeded: "escalate" as const,
  no_progress_window: 2,
  fan_out_width: 4,
  gate_max_revisions: 2,
  reattempt_strategy: "full_body" as const,
  human_gate_enabled: true,
};

const BINDINGS: LoopBindingRow[] = [
  {
    id: "job_nightly",
    name: "nightly",
    kind: "schedule",
    enabled: false,
    meta: "Cron 0 3 * * * · next in 6h",
  },
];

const noop = () => {};

/** Workspace Loop with an existing schedule binding. */
export const WithBinding: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <LoopDetailView
        loop={loop}
        effectiveConfig={{ ...loopEffectiveConfigFixture, ...config }}
        graph={readLoopGraph(loop.definition)}
        recentRuns={recentRuns}
        bindings={BINDINGS}
        bindingsLoading={false}
        successRate={catalogEntry.success_rate_30d}
        aggregate={catalogEntry.aggregate_30d}
        onRun={noop}
        onConfigure={noop}
        onOpenEditor={noop}
        onDelete={async () => undefined}
        onDeleteReset={noop}
        deletePending={false}
        deleteError={null}
        onAddTrigger={noop}
        onAddSchedule={noop}
      />
    </StorySurface>
  ),
};

/** Workspace Loop before any trigger or schedule has been attached. */
export const NoBindings: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <LoopDetailView
        loop={loop}
        effectiveConfig={{ ...loopEffectiveConfigFixture, ...config }}
        graph={readLoopGraph(loop.definition)}
        recentRuns={recentRuns}
        bindings={[]}
        bindingsLoading={false}
        successRate={catalogEntry.success_rate_30d}
        aggregate={catalogEntry.aggregate_30d}
        onRun={noop}
        onConfigure={noop}
        onOpenEditor={noop}
        onDelete={async () => undefined}
        onDeleteReset={noop}
        deletePending={false}
        deleteError={null}
        onAddTrigger={noop}
        onAddSchedule={noop}
      />
    </StorySurface>
  ),
};
