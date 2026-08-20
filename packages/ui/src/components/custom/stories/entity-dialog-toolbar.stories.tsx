import type { Meta, StoryObj } from "@storybook/react-vite";

import { EntityDialogToolbar, PillGroup } from "@compozy/ui";

const meta: Meta<typeof EntityDialogToolbar> = {
  title: "components/custom/EntityDialogToolbar",
  component: EntityDialogToolbar,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Layout row between a modal header and its body. Unpainted on its own. `EntityModeToolbar` composes this, adds the Simple/Advanced pills, and paints the recessed `--color-canvas-tint` chrome strip. Trailing is compact status. Workspace scope belongs in the footer hint, not this row.",
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

/** Stand-in for compact trailing status (source badge, usage count). */
const statusLabel = <span className="font-mono text-form-label text-muted">overlay draft</span>;

/**
 * Trailing only — compact status with no disclosure tier.
 */
export const TrailingOnly: Story = {
  args: {},
  render: () => <EntityDialogToolbar trailing={statusLabel} />,
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
      trailing={statusLabel}
    />
  ),
};

/** Empty layout row — the primitive stays unpainted without a mode control. */
export const Empty: Story = {
  args: {},
  render: () => <EntityDialogToolbar />,
};
