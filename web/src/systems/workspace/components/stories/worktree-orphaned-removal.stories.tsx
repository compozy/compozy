import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";
import { fn, userEvent, within } from "storybook/test";

import { compozyApiMock } from "@/storybook/openapi-msw";
import { StorySurface } from "@/storybook/story-layout";
import { buildWorktreeFixture, discoveredStaleWorktreeFixture } from "@/systems/workspace/mocks";

import type { WorktreeRefusal } from "../../lib/worktree-refusal";
import { WorktreeBulkRemoveDialog } from "../worktree-bulk-remove-dialog";
import { WorktreeRemoveDialog } from "../worktree-remove-dialog";

/** #953 — checkouts whose .git link is gone after an interrupted removal. */
const meta: Meta<typeof WorktreeRemoveDialog> = {
  title: "systems/workspace/components/WorktreeOrphanedRemoval",
  component: WorktreeRemoveDialog,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "A half-removed checkout is refused with its leftover inventory; force deletes the folder and clears Git's entry.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const RISK = {
  changedFiles: 0,
  insertions: 0,
  deletions: 0,
  unpushedCommits: 1,
  existsOnRemote: false,
  leftoverFiles: 1284,
};

const orphanedWorktree = buildWorktreeFixture({
  name: "perf-baseline",
  branch: "",
  path: "/Users/ada/.compozy/worktrees/compozy/perf-baseline",
});

const ORPHANED_REFUSAL: WorktreeRefusal = {
  downgrade: false,
  details: null,
  code: "worktree_checkout_orphaned",
  message:
    "Its Git metadata (.git) is missing, so this folder is left over from an interrupted removal. Nothing was removed.",
  risk: RISK,
};

/** #953 — a checkout that lost its .git link: the reason, the leftover inventory, and force as the way on. */
export const RemoveOrphanedRefusal: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <WorktreeRemoveDialog
        open
        onOpenChange={fn()}
        worktree={orphanedWorktree}
        refusal={ORPHANED_REFUSAL}
        forceRequested={false}
        onRequestForce={fn()}
        isRemoving={false}
        deleteBranch={false}
        onDeleteBranchChange={fn()}
        onConfirm={fn()}
      />
    </StorySurface>
  ),
};

/** #953 — the force step re-states the leftover files and detached commits before deleting them. */
export const RemoveOrphanedForce: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <WorktreeRemoveDialog
        open
        onOpenChange={fn()}
        worktree={orphanedWorktree}
        refusal={ORPHANED_REFUSAL}
        forceRequested
        onRequestForce={fn()}
        isRemoving={false}
        deleteBranch={false}
        onDeleteBranchChange={fn()}
        onConfirm={fn()}
      />
    </StorySurface>
  ),
};

const orphanedDiscovered = [
  {
    ...discoveredStaleWorktreeFixture,
    name: "detached-b1ca6f67",
    branch: "",
    path: "/private/tmp/perf-baseline-b1ca6f671",
    detached: true,
    reason: "gitdir file points to non-existent location",
  },
  {
    ...discoveredStaleWorktreeFixture,
    name: "fix-backend-performance-followup",
    branch: "fix/backend-performance-followup",
    path: "/private/tmp/backend-performance-followup",
    reason: "gitdir file points to non-existent location",
  },
];

/** #953 — half-removed checkouts fail with a readable reason and offer Force remove…, not a bare code. */
export const BulkRemoveOrphaned: Story = {
  tags: ["play-fn"],
  parameters: {
    msw: {
      handlers: [
        compozyApiMock.post(
          "/api/workspaces/{workspace_id}/worktrees/discovered/remove",
          async ({ request }) => {
            const body = (await request.json()) as { path: string; force?: boolean };
            if (body.path === discoveredStaleWorktreeFixture.path || body.force) {
              return new HttpResponse(null, { status: 204 });
            }
            return HttpResponse.json(
              {
                code: "worktree_checkout_orphaned",
                error:
                  "The checkout's Git metadata (.git) is missing; the directory is left over from an interrupted removal. Force removal deletes the leftover files and clears Git's entry.",
                risk: {
                  changed_files: 0,
                  insertions: 0,
                  deletions: 0,
                  unpushed_commits: body.path === orphanedDiscovered[0].path ? 1 : 0,
                  exists_on_remote: false,
                  leftover_files: body.path === orphanedDiscovered[0].path ? 1284 : 37,
                },
              },
              { status: 409 }
            );
          }
        ),
      ],
    },
  },
  render: () => (
    <WorktreeBulkRemoveDialog
      batch={{
        workspaceId: "ws_alpha",
        worktrees: [],
        discovered: [orphanedDiscovered[0], discoveredStaleWorktreeFixture, orphanedDiscovered[1]],
      }}
      onClose={fn()}
    />
  ),
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(await body.findByRole("button", { name: "Remove 3 worktrees" }));
    await body.findByText("1 of 3 done · 2 failed. Session and run history stay readable.");
    await body.findByRole("button", { name: "Force remove…" });
  },
};
