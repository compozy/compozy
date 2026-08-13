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

/** Stand-in for the domain-owned scope statement chip that fills this slot in production. */
const scopeStatement = (
  <span className="inline-flex h-7 items-center rounded-md border border-line bg-canvas-tint px-2.5 text-form-hint text-muted">
    Creates in <span className="ml-1 font-medium text-fg-strong">launch-hq</span>
  </span>
);

/**
 * Scope only — the automation editors, which have no disclosure tier. With
 * nothing on the leading edge the statement starts at the gutter.
 */
export const TrailingOnly: Story = {
  args: {},
  render: () => <EntityDialogToolbar trailing={scopeStatement} />,
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
      trailing={scopeStatement}
    />
  ),
};

/** Empty bar — verifies the band keeps its height with no controls. */
export const Empty: Story = {
  args: {},
  render: () => <EntityDialogToolbar />,
};
