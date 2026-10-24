import type { Meta, StoryObj } from "@storybook/react-vite";

import { DashboardWindowSkeleton } from "../dashboard-window-skeleton";

const meta = {
  title: "systems/dashboard/components/DashboardWindowSkeleton",
  component: DashboardWindowSkeleton,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The Home window before its code or its overview has arrived: the page gutter, the date line and the zones a busy desk shows, in the frames the loaded dashboard fills. One busy status for the whole window; no queries or stores, so hidden windows load cheaply.",
      },
    },
  },
} satisfies Meta<typeof DashboardWindowSkeleton>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
