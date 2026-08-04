import type { Meta, StoryObj } from "@storybook/react-vite";
import { AlertTriangle, Trash2 } from "lucide-react";

import { Button } from "../button";
import { ConfirmDialog } from "../custom/confirm-dialog";
import { RadioCard } from "../custom/radio-card";
import { DialogTrigger } from "../dialog";

const meta: Meta<typeof ConfirmDialog> = {
  title: "components/custom/ConfirmDialog",
  component: ConfirmDialog,
  parameters: {
    layout: "centered",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Danger: Story = {
  args: {},
  render: () => (
    <ConfirmDialog
      cancelLabel="Cancel"
      confirmIcon={Trash2}
      confirmLabel="Delete"
      defaultOpen
      description={
        <>
          This removes <span className="font-mono">operator-style.md</span> from global knowledge.
        </>
      }
      onConfirm={() => undefined}
      title="Delete knowledge entry?"
      tone="danger"
    />
  ),
};

export const Warning: Story = {
  args: {},
  render: () => (
    <ConfirmDialog
      cancelLabel="Keep draft"
      confirmIcon={AlertTriangle}
      confirmLabel="Discard"
      defaultOpen
      description="This draft has unsaved changes."
      onConfirm={() => undefined}
      title="Discard draft?"
      tone="warning"
    />
  ),
};

export const WithNote: Story = {
  args: {},
  render: () => (
    <ConfirmDialog
      cancelLabel="Cancel"
      confirmIcon={Trash2}
      confirmLabel="Remove override"
      defaultOpen
      description="This removes the workspace override."
      note="The builtin provider remains available after the override is removed."
      onConfirm={() => undefined}
      title="Remove provider override?"
      tone="danger"
    />
  ),
};

export const TypingRequired: Story = {
  args: {},
  render: () => (
    <ConfirmDialog
      cancelLabel="Cancel"
      confirmIcon={Trash2}
      confirmLabel="Delete"
      confirmTyping="operator-style.md"
      defaultOpen
      description="Confirm the filename before removing this entry."
      onConfirm={() => undefined}
      title="Delete knowledge entry?"
      tone="danger"
    />
  ),
};

/**
 * `body` is the in-dialog content slot — a mode choice, an input, a machine
 * trail. It sits between the note strip and the footer. (`children` is the
 * Dialog trigger, not body content.)
 */
export const WithBody: Story = {
  args: {},
  render: () => (
    <ConfirmDialog
      body={
        <>
          <RadioCard
            description="Nothing is interrupted. The lane parks once the attempt in flight settles."
            onSelect={() => undefined}
            selected
            title="Let the current attempt finish"
          />
          <RadioCard
            description="The in-flight attempt is asked to cancel, then the lane parks."
            onSelect={() => undefined}
            selected={false}
            title="Ask the current attempt to stop"
          />
          <p className="font-mono text-pill-group-badge text-faint">node_paused · drain</p>
        </>
      }
      cancelLabel="Keep running"
      confirmLabel="Pause lane"
      defaultOpen
      description="The lane stops being scheduled. The rest of the run keeps working."
      note="task_03 is running · attempt 2"
      noteTone="info"
      onConfirm={() => undefined}
      title="Pause task_03?"
      tone="accent"
    />
  ),
};

export const Triggered: Story = {
  args: {},
  render: () => (
    <ConfirmDialog
      cancelLabel="Cancel"
      confirmLabel="Delete"
      description="Open from a trigger to verify focus handoff."
      onConfirm={() => undefined}
      title="Delete entry?"
    >
      <DialogTrigger render={<Button variant="outline">Open confirm</Button>} />
    </ConfirmDialog>
  ),
};
