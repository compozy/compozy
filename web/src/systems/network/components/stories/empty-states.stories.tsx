import type { Meta, StoryObj } from "@storybook/react-vite";

import { PanelSurface } from "@/storybook/story-layout";
import {
  DaemonDown,
  DirectEmpty,
  DirectsEmpty,
  NetworkEmpty,
  ThreadEmpty,
  ThreadsEmpty,
} from "@/systems/network";

const meta: Meta = {
  title: "systems/network/components/EmptyStates",
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component: "Empty / disabled / error states with verbatim copy from `_design.md` §7.",
      },
    },
  },
};

export default meta;
type Story = StoryObj;

export const NetworkOff: Story = {
  render: () => (
    <PanelSurface className="min-h-[320px]">
      <NetworkEmpty disabledByAdmin onOpenSettings={() => undefined} />
    </PanelSurface>
  ),
};

export const NetworkReady: Story = {
  render: () => (
    <PanelSurface className="min-h-[320px]">
      <NetworkEmpty onOpenSettings={() => undefined} />
    </PanelSurface>
  ),
};

export const NoThreads: Story = {
  render: () => (
    <PanelSurface className="min-h-[320px]">
      <ThreadsEmpty />
    </PanelSurface>
  ),
};

export const NoDirects: Story = {
  render: () => (
    <PanelSurface className="min-h-[320px]">
      <DirectsEmpty />
    </PanelSurface>
  ),
};

export const ThreadEmptyState: Story = {
  name: "Thread empty",
  render: () => (
    <PanelSurface className="min-h-[200px]">
      <ThreadEmpty />
    </PanelSurface>
  ),
};

export const DirectEmptyState: Story = {
  name: "Direct empty",
  render: () => (
    <PanelSurface className="min-h-[200px]">
      <DirectEmpty />
    </PanelSurface>
  ),
};

export const NetworkUnreachable: Story = {
  render: () => (
    <PanelSurface className="min-h-[320px]">
      <DaemonDown />
    </PanelSurface>
  ),
};
