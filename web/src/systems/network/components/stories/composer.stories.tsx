import type { Meta, StoryObj } from "@storybook/react-vite";

import { PanelSurface } from "@/storybook/story-layout";
import { Composer } from "@/systems/network/components/composer";

const meta: Meta<typeof Composer> = {
  title: "systems/network/components/Composer",
  component: Composer,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Channel-level / detail composer per `_design.md` §5.7. Shared base; the channel and detail variants compose this primitive with different placeholders and submit handlers.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: () => (
    <PanelSurface className="min-h-[180px] p-0">
      <Composer
        onSubmit={({ reset }) => reset()}
        placeholder="Reply..."
        sendLabel="Send to #ops"
        testIdSuffix="story-default"
      />
    </PanelSurface>
  ),
};

export const Submitting: Story = {
  render: () => (
    <PanelSurface className="min-h-[180px] p-0">
      <Composer
        isSending
        onSubmit={() => undefined}
        placeholder="Reply..."
        sendLabel="Send to #ops"
        testIdSuffix="story-submitting"
      />
    </PanelSurface>
  ),
};

export const Disabled: Story = {
  name: "Disabled (network off)",
  render: () => (
    <PanelSurface className="min-h-[180px] p-0">
      <Composer
        disabled
        disabledReason="Network is off."
        onSubmit={() => undefined}
        placeholder="Reply..."
        sendLabel="Send to #ops"
        testIdSuffix="story-disabled"
      />
    </PanelSurface>
  ),
};

export const Focus: Story = {
  name: "Focus state (input-fill → canvas + line-strong ring)",
  parameters: {
    pseudo: { focusVisible: true },
  },
  render: () => (
    <PanelSurface className="min-h-[180px] p-0">
      <Composer
        onSubmit={({ reset }) => reset()}
        placeholder="Reply..."
        sendLabel="Send to #ops"
        testIdSuffix="story-focus"
      />
    </PanelSurface>
  ),
};
