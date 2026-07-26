import type { Meta, StoryObj } from "@storybook/react-vite";

import { PanelSurface } from "@/storybook/story-layout";
import { bridgeProvidersFixture } from "@/systems/bridges/mocks";

import { BridgeEmptyState } from "../bridge-empty-state";

const meta: Meta<typeof BridgeEmptyState> = {
  title: "systems/bridges/components/BridgeEmptyState",
  component: BridgeEmptyState,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const WithProviders: Story = {
  render: () => (
    <PanelSurface>
      <BridgeEmptyState onCreate={() => undefined} providers={bridgeProvidersFixture} />
    </PanelSurface>
  ),
};

export const WithoutProviders: Story = {
  render: () => (
    <PanelSurface>
      <BridgeEmptyState onCreate={() => undefined} providers={[]} />
    </PanelSurface>
  ),
};

export const AllProvidersUnavailable: Story = {
  render: () => (
    <PanelSurface>
      <BridgeEmptyState
        onCreate={() => undefined}
        providers={[
          {
            ...bridgeProvidersFixture[0],
            enabled: false,
            health: "unhealthy",
            health_message: "Runtime health checks are failing for this provider.",
          },
        ]}
      />
    </PanelSurface>
  ),
};
