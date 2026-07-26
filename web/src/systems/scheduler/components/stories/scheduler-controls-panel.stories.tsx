import type { Meta, StoryObj } from "@storybook/react-vite";

import { PanelSurface } from "@/storybook/story-layout";
import {
  schedulerAttentionStatusFixture,
  schedulerBacklogFixture,
  schedulerPausedStatusFixture,
  schedulerStatusFixture,
} from "../../mocks";
import { SchedulerControlsPanel } from "../scheduler-controls-panel";

const meta: Meta<typeof SchedulerControlsPanel> = {
  title: "systems/scheduler/components/SchedulerControlsPanel",
  component: SchedulerControlsPanel,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function Frame({ children }: { children: React.ReactNode }) {
  return <PanelSurface className="min-h-[420px] p-0">{children}</PanelSurface>;
}

export const Running: Story = {
  render: () => (
    <Frame>
      <SchedulerControlsPanel
        backlog={schedulerBacklogFixture}
        onDrain={() => undefined}
        onPause={() => undefined}
        onResume={() => undefined}
        status={schedulerStatusFixture}
      />
    </Frame>
  ),
};

export const Paused: Story = {
  render: () => (
    <Frame>
      <SchedulerControlsPanel
        backlog={schedulerBacklogFixture}
        onDrain={() => undefined}
        onPause={() => undefined}
        onResume={() => undefined}
        status={schedulerPausedStatusFixture}
      />
    </Frame>
  ),
};

export const AttentionPressure: Story = {
  render: () => (
    <Frame>
      <SchedulerControlsPanel
        backlog={schedulerBacklogFixture}
        onDrain={() => undefined}
        onPause={() => undefined}
        onResume={() => undefined}
        status={schedulerAttentionStatusFixture}
      />
    </Frame>
  ),
};
