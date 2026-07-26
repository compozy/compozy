import type { Meta, StoryObj } from "@storybook/react-vite";
import { AlertTriangle, Trash2 } from "lucide-react";

import { Button } from "../button";
import { ConfirmDialog } from "../custom/confirm-dialog";
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
