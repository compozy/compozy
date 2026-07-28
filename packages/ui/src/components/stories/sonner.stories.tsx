import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, within } from "storybook/test";

import { Button } from "../button";
import { Toaster } from "../sonner";
import { toast } from "../sonner-toast";

const meta: Meta<typeof Toaster> = {
  title: "components/ui/Sonner",
  component: Toaster,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Sonner toaster. Mount a Toaster locally in each story render so notifications stay inside the story iframe and do not leak between stories.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Success: Story = {
  args: {},
  render: () => (
    <div className="flex flex-col items-start gap-3">
      <Button onClick={() => toast.success("Session saved successfully.")}>Trigger toast</Button>
      <Toaster />
    </div>
  ),
};

export const Variants: Story = {
  args: {},
  render: () => (
    <div className="flex flex-col items-start gap-3">
      <div className="flex gap-2">
        <Button variant="outline" onClick={() => toast("Event recorded.")}>
          Default
        </Button>
        <Button variant="outline" onClick={() => toast.info("Dream consolidation scheduled.")}>
          Info
        </Button>
        <Button variant="outline" onClick={() => toast.warning("Approaching token budget.")}>
          Warning
        </Button>
        <Button
          variant="outline"
          onClick={() => toast.error("Daemon disconnected from the UDS socket.")}
        >
          Error
        </Button>
      </div>
      <Toaster />
    </div>
  ),
};

export const WithAction: Story = {
  args: {},
  render: () => (
    <div className="flex flex-col items-start gap-3">
      <Button
        onClick={() =>
          toast("Session archived.", {
            description: "It will be purged in 14 days.",
            action: { label: "Undo", onClick: () => toast.success("Session restored.") },
          })
        }
      >
        Archive session
      </Button>
      <Toaster />
    </div>
  ),
};

export const ErrorPlay: Story = {
  args: {},
  parameters: {
    docs: {
      description: {
        story:
          "Interaction test, clicking the trigger fires `toast.error` and the toast body renders with the danger message.",
      },
    },
  },
  render: () => (
    <div className="flex flex-col items-start gap-3">
      <Button
        data-testid="sonner-error-trigger"
        onClick={() => toast.error("Daemon disconnected from the UDS socket.")}
      >
        Trigger error toast
      </Button>
      <Toaster />
    </div>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const trigger = canvas.getByTestId("sonner-error-trigger");
    await userEvent.click(trigger);
    const body = await within(document.body).findByText("Daemon disconnected from the UDS socket.");
    expect(body).toBeInTheDocument();
  },
};
