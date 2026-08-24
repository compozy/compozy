import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { settingsNotificationPresetCollectionFixture } from "@/systems/settings/mocks";

import { NotificationPresetsPanel } from "../notification-presets-panel";

const meta: Meta<typeof NotificationPresetsPanel> = {
  title: "systems/notifications/components/NotificationPresetsPanel",
  component: NotificationPresetsPanel,
  parameters: { layout: "fullscreen" },
  decorators: [
    Story => (
      <div className="min-h-screen bg-canvas px-8 py-12">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Preset enablement is projected for the active profile and mutates only that profile. */
export const ActiveProfile: Story = {
  args: {
    presets: settingsNotificationPresetCollectionFixture.presets.map(preset => ({
      ...preset,
      profile: "marketing",
    })),
    isLoading: false,
    error: null,
    pendingName: null,
    canMutate: true,
    profile: {
      id: "01J9MARKETING00000000000000",
      name: "marketing",
      color: "#c26ad6",
      icon: "megaphone",
      emoji: null,
      archived: false,
    },
    onCreate: fn(),
    onToggle: fn(),
    onDelete: fn(),
  },
};
