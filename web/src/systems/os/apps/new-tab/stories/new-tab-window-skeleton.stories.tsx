import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";

import { NewTabWindowSkeleton } from "../new-tab-window-skeleton";

const meta = {
  title: "systems/os/components/NewTabWindowSkeleton",
  component: NewTabWindowSkeleton,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The New tab window before its code has loaded: the empty-tab line, the destination button and the palette chord line as blocks, in the centered column the loaded window uses. One busy status; no store.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="flex h-[360px] w-[640px] overflow-hidden rounded-lg border border-line-strong bg-canvas">
          <Story />
        </div>
      </CenteredSurface>
    ),
  ],
} satisfies Meta<typeof NewTabWindowSkeleton>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
