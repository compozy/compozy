import type { Meta, StoryObj } from "@storybook/react-vite";

import { PanelSurface } from "@/storybook/story-layout";

import { SettingsApplyRecordsPanel } from "../settings-apply-records-panel";

const meta = {
  title: "systems/settings/components/SettingsApplyRecordsPanel",
  component: SettingsApplyRecordsPanel,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The history of changes to the settings file. While the first read is pending its rows hold their place with the status, hashes and timing columns of a real record.",
      },
    },
  },
  decorators: [
    Story => (
      <PanelSurface className="w-full max-w-4xl p-6">
        <Story />
      </PanelSurface>
    ),
  ],
} satisfies Meta<typeof SettingsApplyRecordsPanel>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The first read is pending: three placeholder records in the real three-column row. */
export const Loading: Story = {
  args: {
    isLoading: true,
    isReloading: false,
    onRefresh: () => {},
    onReload: () => {},
    records: [],
  },
};

/** Nothing recorded yet. */
export const Empty: Story = {
  args: {
    isLoading: false,
    isReloading: false,
    onRefresh: () => {},
    onReload: () => {},
    records: [],
  },
};
