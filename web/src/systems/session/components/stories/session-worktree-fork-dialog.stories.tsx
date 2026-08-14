import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { fn, userEvent, within } from "storybook/test";

import { StorySurface } from "@/storybook/story-layout";
import type { WorktreeMaterialization } from "@/systems/workspace";
import {
  worktreeBehindFixture,
  worktreeMergedFixture,
  worktreeReadyDirtyRunningFixture,
} from "@/systems/workspace/mocks";

import { SessionWorktreeForkDialog } from "../session-worktree-fork-dialog";

const WORKTREES = [worktreeReadyDirtyRunningFixture, worktreeBehindFixture, worktreeMergedFixture];
const SESSION_TITLE = "Fix payment retries";

/** A materialization with inert controls: stories state a phase, they never drive one. */
function materialization(
  overrides: Partial<WorktreeMaterialization> = {}
): WorktreeMaterialization {
  return {
    status: "idle",
    start: fn(),
    cancel: fn(),
    recover: fn(),
    retry: fn(),
    reset: fn(),
    ...overrides,
  };
}

function Harness({
  blockedReason,
  onNewWorktree,
  initialTarget = worktreeBehindFixture.id,
}: {
  blockedReason?: string;
  onNewWorktree?: () => void;
  initialTarget?: string;
}) {
  const [target, setTarget] = useState(initialTarget);
  const [live, setLive] = useState<WorktreeMaterialization | undefined>();
  return (
    <StorySurface>
      <SessionWorktreeForkDialog
        blockedReason={blockedReason}
        currentWorktreeId={worktreeReadyDirtyRunningFixture.id}
        materialization={live}
        onConfirm={fn()}
        onNewWorktree={
          onNewWorktree
            ? () => {
                onNewWorktree();
                setLive(materialization({ status: "creating" }));
              }
            : undefined
        }
        onOpenChange={fn()}
        onTargetChange={setTarget}
        open
        sessionTitle={SESSION_TITLE}
        targetWorktreeId={target}
        worktrees={WORKTREES}
      />
    </StorySurface>
  );
}

const meta: Meta<typeof SessionWorktreeForkDialog> = {
  title: "systems/session/components/SessionWorktreeForkDialog",
  component: SessionWorktreeForkDialog,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "A live session's binding cannot change, so moving work to a worktree means forking. The three facts are the whole dialog: a second session is created, this one keeps running, and nothing moves on disk.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-13 — the three facts, above a target the operator still has to choose. */
export const ThreeFactConfirm: Story = {
  args: {
    open: true,
    sessionTitle: SESSION_TITLE,
    worktrees: WORKTREES,
    targetWorktreeId: worktreeBehindFixture.id,
    onOpenChange: fn(),
    onTargetChange: fn(),
    onConfirm: fn(),
  },
  render: () => <Harness />,
};

/**
 * VC-14 — selecting New worktree starts creation immediately.
 *
 * Authorized delta: `onNewWorktree` does not change `targetWorktreeId`, so the
 * trigger stays empty. Confirm stays `Fork session` and disabled — there is no
 * `Create & fork` label and no `kind: "new"` on the trigger. The strip uses the
 * fork's `phase ?? "creating"` fallback until a pending row exists.
 */
export const NewWorktreeTarget: Story = {
  args: ThreeFactConfirm.args,
  render: () => <Harness initialTarget="" onNewWorktree={fn()} />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(await body.findByTestId("session-fork-target"));
    await userEvent.click(await body.findByRole("option", { name: /new worktree/i }));
  },
};

/** VC-15 — mid-turn: the daemon's refusal is the note, and confirm cannot run. */
export const BlockedMidTurn: Story = {
  args: ThreeFactConfirm.args,
  render: () => <Harness blockedReason="Wait for the current turn to finish." />,
};
