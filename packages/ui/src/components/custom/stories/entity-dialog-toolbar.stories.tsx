import type { Meta, StoryObj } from "@storybook/react-vite";
import { Box, Globe } from "lucide-react";

import { EntityDialogToolbar, PillGroup } from "@agh/ui";

const meta: Meta<typeof EntityDialogToolbar> = {
  title: "components/custom/EntityDialogToolbar",
  component: EntityDialogToolbar,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "The band between a modal header and its body (`.modebar`, modal-system.css:194). Rules only its bottom edge — a top border would stack against the ruled header's own and print a 2px seam. `EntityModeToolbar` composes this and adds the Simple/Advanced pills; surfaces with scope but no disclosure tier use this directly, so scope sits in the same place on every modal that has it.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[720px] bg-canvas-soft">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

const scopePills = (
  <PillGroup
    aria-label="Scope"
    items={[
      {
        value: "global",
        label: (
          <span className="flex items-center gap-1.5">
            <Globe className="size-3" />
            Global
          </span>
        ),
      },
      {
        value: "workspace",
        label: (
          <span className="flex items-center gap-1.5">
            <Box className="size-3" />
            Workspace
          </span>
        ),
      },
    ]}
    onChange={() => undefined}
    size="md"
    value="workspace"
  />
);

/**
 * Scope only — the automation editors, which have no disclosure tier. With
 * nothing on the leading edge the scope starts at the gutter.
 */
export const TrailingOnly: Story = {
  args: {},
  render: () => <EntityDialogToolbar trailing={scopePills} trailingLabel="Scope" />,
};

/** With a leading control the trailing one is pushed to the far edge. */
export const LeadingAndTrailing: Story = {
  args: {},
  render: () => (
    <EntityDialogToolbar
      leading={
        <PillGroup
          aria-label="Editor mode"
          items={[
            { value: "simple", label: "Simple" },
            { value: "advanced", label: "Advanced" },
          ]}
          onChange={() => undefined}
          size="sm"
          value="simple"
        />
      }
      trailing={scopePills}
      trailingLabel="Scope"
    />
  ),
};

/** Empty bar — verifies the band keeps its height with no controls. */
export const Empty: Story = {
  args: {},
  render: () => <EntityDialogToolbar />,
};
