import type { Meta, StoryObj } from "@storybook/react-vite";
import { Bot } from "lucide-react";

import { OwnerAvatar } from "../owner-avatar";

const meta: Meta<typeof OwnerAvatar> = {
  title: "components/custom/OwnerAvatar",
  component: OwnerAvatar,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          'Owner avatar primitive — resolves background/foreground via `colorsFor(ownerKind, ownerId)` against the tokenised palette (`--avatar-agent-N-*`, `--avatar-human-N-*`, `--avatar-system-*`). Renders a 2-char monogram by default; accepts a glyph slot for system owners. Emits `aria-label="{Role} {Name}"` so screen readers announce the role',
      },
    },
  },
  decorators: [
    Story => (
      <div className="bg-background p-6">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Agent: Story = {
  args: { ownerKind: "agent", ownerId: "planner-prime", name: "Planner Prime" },
};

export const Human: Story = {
  args: { ownerKind: "human", ownerId: "pedro", name: "Pedro Nauck" },
};

export const System: Story = {
  args: { ownerKind: "system", ownerId: "daemon", name: "Daemon" },
};

export const Glyph: Story = {
  args: {
    ownerKind: "system",
    ownerId: "scheduler",
    name: "Scheduler",
    glyph: <Bot width={14} height={14} strokeWidth={1.75} />,
  },
};

export const SizeLadder: Story = {
  render: () => (
    <div className="flex items-center gap-4">
      <OwnerAvatar ownerKind="agent" ownerId="planner" name="Planner Prime" size="sm" />
      <OwnerAvatar ownerKind="agent" ownerId="planner" name="Planner Prime" />
      <OwnerAvatar ownerKind="agent" ownerId="planner" name="Planner Prime" size="lg" />
    </div>
  ),
};

export const OwnerKindMatrix: Story = {
  render: () => (
    <div className="grid grid-cols-3 gap-6 [&_label]:text-muted [&_label]:text-[11px] [&_label]:font-medium [&_label]:uppercase [&_label]:tracking-mono">
      <div className="flex flex-col items-center gap-2">
        <label>Agent · sm/default/lg</label>
        <div className="flex items-center gap-3">
          <OwnerAvatar ownerKind="agent" ownerId="planner" name="Planner" size="sm" />
          <OwnerAvatar ownerKind="agent" ownerId="planner" name="Planner" />
          <OwnerAvatar ownerKind="agent" ownerId="planner" name="Planner" size="lg" />
        </div>
      </div>
      <div className="flex flex-col items-center gap-2">
        <label>Human · sm/default/lg</label>
        <div className="flex items-center gap-3">
          <OwnerAvatar ownerKind="human" ownerId="pedro" name="Pedro" size="sm" />
          <OwnerAvatar ownerKind="human" ownerId="pedro" name="Pedro" />
          <OwnerAvatar ownerKind="human" ownerId="pedro" name="Pedro" size="lg" />
        </div>
      </div>
      <div className="flex flex-col items-center gap-2">
        <label>System · sm/default/lg</label>
        <div className="flex items-center gap-3">
          <OwnerAvatar ownerKind="system" ownerId="daemon" name="Daemon" size="sm" />
          <OwnerAvatar ownerKind="system" ownerId="daemon" name="Daemon" />
          <OwnerAvatar ownerKind="system" ownerId="daemon" name="Daemon" size="lg" />
        </div>
      </div>
    </div>
  ),
};
