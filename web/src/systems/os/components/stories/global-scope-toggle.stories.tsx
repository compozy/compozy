import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { GLOBAL_SCOPE_COPY } from "@/systems/workspace";
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
          "Menubar owner of Global vs workspace destination. Create dialogs read this mode; they never pick a destination themselves.",
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

/** Workspace-scoped: Global is off and the control can turn it on. */
export const Off: Story = {
  args: {
    checked: false,
    locked: false,
    onCheckedChange: fn(),
    tooltip: GLOBAL_SCOPE_COPY.tooltipOff,
  },
};

/** Global is on; the tooltip names the workspace to return to. */
export const On: Story = {
  args: {
    checked: true,
    locked: false,
    onCheckedChange: fn(),
    tooltip: "Back to alpha",
  },
};

/** No project workspace exists, so the globe toggle cannot leave Global. */
export const Locked: Story = {
  args: {
    checked: true,
    locked: true,
    onCheckedChange: fn(),
    tooltip: GLOBAL_SCOPE_COPY.tooltipLocked,
  },
};
