import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { StorySurface } from "@/storybook/story-layout";
import {
  worktreeMissingFixture,
  worktreeReadyDirtyRunningFixture,
} from "@/systems/workspace/mocks";

import { SessionWorktreeBindingChip } from "../session-worktree-binding-chip";

const meta: Meta<typeof SessionWorktreeBindingChip> = {
  title: "systems/session/components/SessionWorktreeBindingChip",
  component: SessionWorktreeBindingChip,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Which worktree a session is bound to, in the session header. An unbound session renders nothing — absence is the signal. The exit control deliberately has no mount point here (OQ7): it lives in the worktree context alone.",
      },
    },
  },
  render: args => (
    <StorySurface className="flex items-center justify-center p-6">
      <SessionWorktreeBindingChip {...args} />
    </StorySurface>
  ),
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-16 — bound: the record label, and a way into that worktree's context. */
export const Bound: Story = {
  args: {
    worktree: worktreeReadyDirtyRunningFixture,
    worktreeId: worktreeReadyDirtyRunningFixture.id,
    onOpenContext: fn(),
  },
};

/** VC-17 — the binding no longer resolves, so the chip offers the resolution. */
export const MissingWithResolve: Story = {
  args: {
    worktree: worktreeMissingFixture,
    worktreeId: worktreeMissingFixture.id,
    onResolve: fn(),
  },
};
