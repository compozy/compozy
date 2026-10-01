import type { Meta, StoryObj } from "@storybook/react-vite";

import { Surface } from "../surface";

const meta: Meta<typeof Surface> = {
  title: "components/ui/Surface",
  component: Surface,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "One of the two surface levels at `rounded-lg`: `card` (default) lifts content on `--canvas` with `shadow-card`; `sunken` recesses secondary lists on `--sunken` with no border or shadow. Compose it instead of re-copying the padding tuple; content layout stays with the consumer.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[320px]">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: () => (
    <Surface className="flex flex-col gap-2">
      <span className="text-form-label text-muted">Active runs</span>
      <span className="text-kpi-value font-display text-fg-strong tabular-nums">18</span>
    </Surface>
  ),
};

export const Compact: Story = {
  render: () => (
    <Surface size="compact" className="flex items-center justify-between gap-2">
      <span className="text-small-body text-fg">Queue depth</span>
      <span className="font-mono text-mono-id tabular-nums text-muted">142</span>
    </Surface>
  ),
};

/** Recessed inset for secondary lists — tool rows, command previews. */
export const Sunken: Story = {
  render: () => (
    <Surface variant="sunken" size="compact" className="flex flex-col gap-1">
      <span className="text-small-body text-muted">
        <span className="font-medium text-fg">Read</span> web/src/checkout/order-summary.tsx
      </span>
      <span className="text-small-body text-muted">
        <span className="font-medium text-fg">Run</span> bun run test checkout
      </span>
    </Surface>
  ),
};
