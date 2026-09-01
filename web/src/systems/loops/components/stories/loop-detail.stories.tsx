import type { Meta, StoryObj } from "@storybook/react-vite";

import { LoopDetailView } from "../detail/loop-detail";
import type { LoopBindingRow } from "../../lib/loop-bindings";
import { readLoopGraph } from "../../lib/loop-graph";
import { releaseTrainDetail } from "../../mocks";
import {
  loopCatalogFixtures,
  loopDetailByName,
  loopEffectiveConfigFixture,
  loopRunFixtures,
} from "../../mocks/fixtures";
import { LoopsStoryShell } from "./loops-story-shell";

const meta: Meta<typeof LoopDetailView> = {
  title: "systems/loops/components/LoopDetail",
  component: LoopDetailView,
  decorators: [
    Story => (
      <LoopsStoryShell crumb="implement-tasks">
        <Story />
      </LoopsStoryShell>
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

const loop = loopDetailByName.get("implement-tasks")!;
const catalogEntry = loopCatalogFixtures.find(entry => entry.name === "implement-tasks")!;
// Keeps the operator-canceled terminal in view: `canceled` is its own ending, not a failure.
const deliveryRuns = loopRunFixtures.filter(run => run.loop_name === "implement-tasks");
const canceledRun = deliveryRuns.find(run => run.status === "canceled")!;
const recentRuns = [
  ...deliveryRuns.filter(run => run.status !== "canceled").slice(0, 4),
  canceledRun,
];
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
    <>
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
    </>
  ),
};

/** Workspace Loop before any trigger or schedule has been attached. */
export const NoBindings: Story = {
  args: {},
  render: () => (
    <>
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
    </>
  ),
};

export const GraphCompletionGlyphs: Story = {
  args: {},
  render: () => (
    <LoopDetailView
      aggregate={catalogEntry.aggregate_30d}
      bindings={[]}
      bindingsLoading={false}
      deleteError={null}
      deletePending={false}
      effectiveConfig={{ ...loopEffectiveConfigFixture, ...config }}
      graph={readLoopGraph(releaseTrainDetail.definition)}
      loop={releaseTrainDetail}
      onAddSchedule={noop}
      onAddTrigger={noop}
      onConfigure={noop}
      onDelete={async () => undefined}
      onDeleteReset={noop}
      onOpenEditor={noop}
      onRun={noop}
      recentRuns={[]}
      successRate={catalogEntry.success_rate_30d}
    />
  ),
};
