import type { Meta, StoryObj } from "@storybook/react-vite";

import { PanelSurface } from "@/storybook/story-layout";

import { SettingsPageState } from "../settings-page-state";

const meta = {
  title: "systems/settings/components/SettingsPageState",
  component: SettingsPageState,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Whole-page loading and failure for a settings section. Loading is one status region over the section's own skeleton: the real scaffold, groups and rows, with placeholders for what the daemon returns, so the page arrives without anything moving.",
      },
    },
  },
  decorators: [
    Story => (
      <PanelSurface className="flex min-h-[720px] flex-col">
        <Story />
      </PanelSurface>
    ),
  ],
} satisfies Meta<typeof SettingsPageState>;

export default meta;
type Story = StoryObj<typeof meta>;

/** A form section: the consequence line, grouped rows, a choice group and the Advanced fold. */
export const LoadingForm: Story = {
  args: { slug: "general", state: "loading" },
};

/** A listing section at the wide column: toolbar and provider cards. */
export const LoadingListing: Story = {
  args: { slug: "providers", state: "loading" },
};

/** A table section: the MCP toolbar over its real table header and rows. */
export const LoadingTable: Story = {
  args: { slug: "mcp", state: "loading" },
};

/** The layout canvas at its own width. */
export const LoadingCanvas: Story = {
  args: { slug: "layouts", state: "loading" },
};

/** The failure keeps its alert, the cause behind the Details fold, and a retry. */
export const Failed: Story = {
  args: {
    slug: "general",
    state: "error",
    error: new Error("daemon unreachable"),
    onRetry: () => {},
  },
};
