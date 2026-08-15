import type { Meta, StoryObj } from "@storybook/react-vite";
import { AlertTriangle, Ban, Hourglass, Info, Pause, Trash2, Zap } from "lucide-react";

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

/** Danger well + eyebrow. Body copy stays muted; confirm uses tinted destructive. */
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
      eyebrow="Knowledge"
      icon={Trash2}
      iconTone="danger"
      onConfirm={() => undefined}
      title="Delete knowledge entry?"
      tone="danger"
    />
  ),
};

/** Warning paints the eyebrow; the well stays neutral; confirm maps to tinted destructive. */
export const Warning: Story = {
  args: {},
  render: () => (
    <ConfirmDialog
      cancelLabel="Keep draft"
      confirmIcon={AlertTriangle}
      confirmLabel="Discard"
      defaultOpen
      description="This draft has unsaved changes."
      eyebrow="Draft"
      icon={AlertTriangle}
      iconTone="neutral"
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
 * Accent well + 28px RadioCard wells. Machine trail lives in `footNote`,
 * not the body. (`children` is the Dialog trigger, not body content.)
 */
export const WithBody: Story = {
  args: {},
  render: () => (
    <ConfirmDialog
      body={
        <>
          <RadioCard
            description="Nothing is interrupted. The lane parks once the attempt in flight settles."
            icon={Hourglass}
            iconWellSize="lg"
            onSelect={() => undefined}
            selected
            title="Let the current attempt finish"
          />
          <RadioCard
            description="The in-flight attempt is asked to cancel, then the lane parks."
            icon={Ban}
            iconWellSize="lg"
            onSelect={() => undefined}
            selected={false}
            title="Ask the current attempt to stop"
          />
        </>
      }
      cancelLabel="Keep running"
      confirmLabel="Pause lane"
      defaultOpen
      description="The lane stops being scheduled. The rest of the run keeps working."
      eyebrow="Node"
      footNote={
        <>
          <Info />
          mode: drain
        </>
      }
      icon={Pause}
      iconTone="accent"
      note="task_03 is running · attempt 2"
      noteTone="info"
      onConfirm={() => undefined}
      title="Pause task_03?"
      tone="accent"
    />
  ),
};

/** Kill uses the solid danger fill; cancel stays on the tinted destructive. */
export const DestructiveSolidConfirm: Story = {
  args: {},
  render: () => (
    <ConfirmDialog
      cancelLabel="Keep running"
      confirmButtonProps={{ variant: "destructive-solid" }}
      confirmIcon={Zap}
      confirmLabel="Kill run"
      defaultOpen
      description="No on_* reactions fire. The run ends immediately."
      eyebrow="Run"
      footNote={
        <>
          <Info />
          node_killed · no on_* reactions
        </>
      }
      icon={Zap}
      iconTone="danger"
      onConfirm={() => undefined}
      title="Kill this run?"
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
