import type { Meta, StoryObj } from "@storybook/react-vite";

import { gatewayAccessStore } from "@/systems/gateway";

import { WorkspaceProfilesHint } from "../workspace-profiles-hint";

const HINTS = [
  {
    name: "studio",
    path: ".compozy/profiles/studio",
    message: "Profile folder found",
    action: "create",
  },
  {
    name: "finance",
    path: ".compozy/profiles/finance",
    message: "Profile folder found",
    action: "create",
  },
];

const meta: Meta<typeof WorkspaceProfilesHint> = {
  title: "systems/profiles/components/WorkspaceProfilesHint",
  component: WorkspaceProfilesHint,
  parameters: { layout: "fullscreen" },
  decorators: [
    Story => {
      gatewayAccessStore.trigger.tierSignalled({ tier: "local" });
      return (
        <div className="relative h-screen bg-canvas">
          <Story />
        </div>
      );
    },
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Every absent repository profile name receives its own canonical create action. */
export const MultipleNames: Story = {
  args: { hints: HINTS, workspaceId: "workspace-compozy" },
};

/** Names already present in the profile catalog disappear while the remaining ask stays. */
export const PartiallyAdopted: Story = {
  args: {
    hints: [
      ...HINTS.slice(0, 1),
      {
        name: "marketing",
        path: ".compozy/profiles/marketing",
        message: "Profile folder found",
        action: "create",
      },
    ],
    workspaceId: "workspace-compozy",
  },
};
