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
          "Exact-name confirmation for irreversible deletion of an automation created here.",
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

/** The irreversible action opens with safe cancellation and exact-name confirmation. */
export const ScheduleConfirmation: Story = {
  args: {
    consequence: "Its schedule will stop asking summarizer. Past runs stay in the log.",
    isPending: false,
    name: "morning-digest",
    onConfirm: fn(),
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: "Delete automation" }));
    const page = within(canvasElement.ownerDocument.body);
    await expect(page.findByRole("dialog", { name: "Delete automation?" })).resolves.toBeDefined();
  },
};
