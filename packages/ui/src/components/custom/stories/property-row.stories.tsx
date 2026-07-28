import type { Meta, StoryObj } from "@storybook/react-vite";
import { ChevronsUpDown } from "lucide-react";

import { MonoId } from "../mono-id";
import { PropertyRow } from "../property-row";

const meta: Meta<typeof PropertyRow> = {
  title: "components/custom/PropertyRow",
  component: PropertyRow,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Detail-rail key/value row: quiet label left, value right. Supports a mono value voice for ids and an inline editor slot.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-72 rounded-lg border border-line bg-canvas-soft px-4 py-3">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Static value rows, including the mono id voice.
 */
export const Default: Story = {
  args: {},
  render: () => (
    <>
      <PropertyRow label="Workspace">launch-hq</PropertyRow>
      <PropertyRow label="Attempts">2 of 3</PropertyRow>
      <PropertyRow label="Task id" mono>
        <MonoId value="task_001" />
      </PropertyRow>
    </>
  ),
};

/**
 * Long runtime values stay on one line and retain the complete value as a
 * native title affordance.
 */
export const LongValue: Story = {
  args: {},
  render: () => (
    <PropertyRow label="Workspace" mono>
      workspace_01J7GJ67XQ6S0C8R1DVKPX9F5N
    </PropertyRow>
  ),
};

/**
 * Inline editor slot replacing the static value.
 */
export const WithEditor: Story = {
  args: {},
  render: () => (
    <PropertyRow
      editor={
        <button
          className="inline-flex items-center gap-1.5 rounded-sm px-1.5 py-0.5 text-small-body font-medium text-fg hover:bg-row-hover"
          type="button"
        >
          High
          <ChevronsUpDown aria-hidden="true" className="size-3 text-faint" />
        </button>
      }
      label="Priority"
    />
  ),
};
