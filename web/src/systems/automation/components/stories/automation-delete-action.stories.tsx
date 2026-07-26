import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { CenteredSurface } from "@/storybook/story-layout";

import { AutomationDeleteAction } from "../automation-delete-action";

const meta: Meta<typeof AutomationDeleteAction> = {
  title: "systems/automation/components/AutomationDeleteAction",
  component: AutomationDeleteAction,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Exact-name confirmation for irreversible deletion of dynamic Jobs and Triggers.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <Story />
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** The irreversible Job action opens with safe cancellation and exact-name confirmation. */
export const JobConfirmation: Story = {
  args: {
    isPending: false,
    kind: "jobs",
    name: "software-delivery-qa",
    onConfirm: fn(),
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: "Delete job" }));
    const page = within(canvasElement.ownerDocument.body);
    await expect(page.findByRole("dialog", { name: "Delete job?" })).resolves.toBeDefined();
  },
};
