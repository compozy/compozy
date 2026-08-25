import type { Meta, StoryObj } from "@storybook/react-vite";

import { PanelSurface } from "@/storybook/story-layout";

import { AgentCallTree } from "../agent-call-tree";
import { buildCallTree } from "../../lib/agent-comms-tree";
import {
  activityTreeCallsFixture,
  buildLargeTreeFixture,
  callFixtureRootSessionId,
  nineStateCallsFixture,
} from "../../mocks";
import type { CallPayload } from "../../types";

const meta: Meta<typeof AgentCallTree> = {
  title: "systems/agent-comms/components/AgentCallTree",
  component: AgentCallTree,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Delegation trees: one group per governed root, rows indented by the daemon's own depth. Counts come from filtered `total` probes, so a header can say 3 calls while a page shows fewer.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function frame(
  calls: readonly CallPayload[],
  args: Partial<Parameters<typeof AgentCallTree>[0]> = {}
) {
  return (
    <PanelSurface>
      <AgentCallTree
        tree={buildCallTree(calls)}
        onSelectCall={() => undefined}
        onStopSubtree={() => undefined}
        {...args}
      />
    </PanelSurface>
  );
}

/** Two live trees at depths 1–3, with real per-tree counts on the headers. */
export const Default: Story = {
  render: () =>
    frame(activityTreeCallsFixture, {
      countsByRoot: new Map([[callFixtureRootSessionId, { total: 3, running: 2, needsYou: 0 }]]),
    }),
};

/** Every one of the nine states in one tree. */
export const StateSpectrum: Story = {
  render: () => frame(nineStateCallsFixture),
};

/** Counts still in flight: the header shows no summary rather than a zero. */
export const CountsUnknown: Story = {
  render: () => frame(activityTreeCallsFixture),
};

/** The scale case — 150 sibling calls under one root. */
export const LargeTree: Story = {
  render: () => frame(buildLargeTreeFixture(150)),
};

/** No drain permission: the control is absent, never disabled. */
export const WithoutStopSubtree: Story = {
  render: () => (
    <PanelSurface>
      <AgentCallTree
        tree={buildCallTree(activityTreeCallsFixture)}
        onSelectCall={() => undefined}
      />
    </PanelSurface>
  ),
};
