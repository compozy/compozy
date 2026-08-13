import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { fn } from "storybook/test";

import { StorySurface } from "@/storybook/story-layout";
import { worktreeBehindFixture, worktreeReadyDirtyRunningFixture } from "@/systems/workspace/mocks";

import type { LoopEnvironmentSpec } from "../../types";
import { LoopWorktreeSection } from "../configure/loop-worktree-section";

const WORKTREES = [worktreeReadyDirtyRunningFixture, worktreeBehindFixture];

function SectionHarness({ initial }: { initial: LoopEnvironmentSpec | null }) {
  const [value, setValue] = useState(initial);
  return (
    <StorySurface className="w-[520px]">
      <LoopWorktreeSection gitBacked onChange={setValue} value={value} worktrees={WORKTREES} />
    </StorySurface>
  );
}

const meta: Meta<typeof LoopWorktreeSection> = {
  title: "systems/loops/components/LoopWorktreeEnvironment",
  component: LoopWorktreeSection,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The loop-level environment default. Inherit is the rendering of an absent key — choosing it clears the stored default rather than writing a placeholder mode — and a node override always wins over whatever is set here. The per-node control is captured on the LoopEditor inspector (VC-31..VC-35).",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-28 — unset: no loop default is declared, so nodes choose for themselves. */
export const LoopDefaultUnset: Story = {
  args: { value: null, worktrees: WORKTREES, gitBacked: true, onChange: fn() },
  render: () => <SectionHarness initial={null} />,
};

/** VC-29 — the loop default names one worktree every node inherits. */
export const LoopDefaultNamedWorktree: Story = {
  args: {
    ...LoopDefaultUnset.args,
    value: { mode: "worktree", worktree_ref: worktreeBehindFixture.id },
  },
  render: () => (
    <SectionHarness initial={{ mode: "worktree", worktree_ref: worktreeBehindFixture.id }} />
  ),
};

/** VC-30 — per-run: a fresh worktree for each run of the loop. */
export const LoopDefaultPerRun: Story = {
  args: { ...LoopDefaultUnset.args, value: { mode: "per_run" } },
  render: () => <SectionHarness initial={{ mode: "per_run" }} />,
};
