import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { DeleteButton } from "../delete-button";

const meta: Meta<typeof DeleteButton> = {
  title: "components/custom/DeleteButton",
  component: DeleteButton,
  args: { label: "Close Review", onConfirm: fn(), onCancel: fn(), onPhaseChange: fn() },
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "In-place confirm for a destructive action: the bin opens into a cross that cancels and a check that confirms; Escape cancels. Use it where a confirm dialog would cover the thing being removed.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Rest: Story = {};

export const Open: Story = { args: { phase: "open" } };

/** Uncontrolled, mounted asking for confirmation: a host that opens straight into the confirm. */
export const DefaultOpen: Story = { args: { defaultPhase: "open" } };

/** Confirm waits for a precondition, such as a destination for the removed item's contents. */
export const ConfirmUnavailable: Story = { args: { phase: "open", canConfirm: false } };

export const Confirmed: Story = { args: { phase: "confirmed" } };

export const ConfirmFlow: Story = {
  tags: ["play-fn"],
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: "Close Review" }));
    await expect(canvas.getByRole("button", { name: "Cancel" })).toHaveFocus();
    await userEvent.click(canvas.getByRole("button", { name: "Confirm: Close Review" }));
    await expect(args.onConfirm).toHaveBeenCalledTimes(1);
  },
};

export const EscapeCancels: Story = {
  tags: ["play-fn"],
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: "Close Review" }));
    await userEvent.keyboard("{Escape}");
    await expect(args.onCancel).toHaveBeenCalledTimes(1);
    await expect(canvas.getByRole("button", { name: "Close Review" })).toHaveFocus();
  },
};
