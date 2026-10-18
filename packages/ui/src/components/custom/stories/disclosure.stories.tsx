import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, within } from "storybook/test";

import { Disclosure } from "../disclosure";

const meta: Meta<typeof Disclosure> = {
  title: "components/custom/Disclosure",
  component: Disclosure,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          'Closed-by-default progressive disclosure for the secondary layer: `inline` is a quiet text toggle ("More options", "Technical details"); `framed` is the bordered settings "Advanced" panel.',
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const Facts = () => (
  <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-small-body text-muted">
    <dt>Process</dt>
    <dd className="font-mono">4312</dd>
    <dt>Socket</dt>
    <dd className="font-mono">~/.compozy/compozy.sock</dd>
  </dl>
);

export const Inline: Story = {
  args: { label: "Technical details", children: <Facts /> },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const toggle = canvas.getByRole("button", { name: "Technical details" });
    await expect(toggle).toHaveAttribute("aria-expanded", "false");
    await userEvent.click(toggle);
    await expect(canvas.getByText("4312")).toBeVisible();
  },
};

export const InlineMedium: Story = {
  args: { label: "History", size: "md", defaultOpen: true, children: <Facts /> },
};

export const Framed: Story = {
  args: {
    label: "Advanced",
    variant: "framed",
    size: "md",
    contentProps: { className: "p-4" },
    children: <Facts />,
  },
};
