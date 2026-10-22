import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { allProjectsTooltipOn, DESTINATION_COPY } from "@/systems/workspace";
import { CenteredSurface } from "@/storybook/story-layout";

import { GlobalScopeToggle } from "../global-scope-toggle";

const meta: Meta<typeof GlobalScopeToggle> = {
  title: "systems/os/components/GlobalScopeToggle",
  component: GlobalScopeToggle,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Menubar owner of breadth: pressed is All projects, unpressed is the focus (a project or User). Create dialogs read this mode; they never pick a destination themselves.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface className="p-6">
        <Story />
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** A project or User is focused: All projects is off and the control can turn it on. */
export const Off: Story = {
  args: {
    checked: false,
    locked: false,
    onCheckedChange: fn(),
    tooltip: DESTINATION_COPY.allProjects,
  },
};

/** All projects is on; the tooltip names the focus to return to. */
export const On: Story = {
  args: {
    checked: true,
    locked: false,
    onCheckedChange: fn(),
    tooltip: allProjectsTooltipOn("alpha"),
  },
};

/** All projects is on with User as the focus to return to (zero projects never lock it). */
export const OnBackToUser: Story = {
  args: {
    checked: true,
    locked: false,
    onCheckedChange: fn(),
    tooltip: allProjectsTooltipOn(DESTINATION_COPY.user),
  },
};

/** Destination resolution is still pending, so the globe waits. */
export const Locked: Story = {
  args: {
    checked: true,
    locked: true,
    onCheckedChange: fn(),
    tooltip: DESTINATION_COPY.allProjects,
  },
};
