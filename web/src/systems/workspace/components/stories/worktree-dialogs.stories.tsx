import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn, userEvent, within } from "storybook/test";
import { HttpResponse } from "msw";
import { compozyApiMock } from "@/storybook/openapi-msw";
import { WorktreeBulkRemoveDialog } from "../worktree-bulk-remove-dialog";

import { StorySurface } from "@/storybook/story-layout";
import {
  LONG_WORKTREE_PATH,
  buildWorktreeFixture,
  discoveredStaleWorktreeFixture,
  discoveredWorktreeFixture,
  worktreeMissingFixture,
  worktreeReadyDirtyRunningFixture,
} from "@/systems/workspace/mocks";

import type { WorktreeRefusal } from "../../lib/worktree-refusal";
import { WorktreeAdoptDialog } from "../worktree-adopt-dialog";
import { WorktreeMissingResolutionDialog } from "../worktree-missing-resolution-dialog";
import { WorktreeRemoveDialog } from "../worktree-remove-dialog";

const DIRTY_RISK = {
  changedFiles: 6,
  insertions: 142,
  deletions: 18,
  unpushedCommits: 3,
  existsOnRemote: false,
};

function refusal(overrides: Partial<WorktreeRefusal>): WorktreeRefusal {
  return {
    code: "worktree_dirty_requires_force",
    message: "The worktree holds work that exists nowhere else.",
    risk: DIRTY_RISK,
    downgrade: false,
    details: null,
    ...overrides,
  };
}

const cleanWorktree = buildWorktreeFixture({
  name: "fix-flaky-e2e",
  branch: "run/fix-flaky-e2e",
  path: LONG_WORKTREE_PATH,
});

const meta: Meta<typeof WorktreeAdoptDialog> = {
  title: "systems/workspace/components/WorktreeLifecycleDialogs",
  component: WorktreeAdoptDialog,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Adoption, removal, and missing-resolution dialogs. Every destructive transition is explicit, quantified, and refusable (ADR-006).",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-13 — adoption confirm naming the validation. */
export const AdoptConfirm: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <WorktreeAdoptDialog
        open
        onOpenChange={fn()}
        discovered={discoveredWorktreeFixture}
        refusal={null}
        isAdopting={false}
        onConfirm={fn()}
      />
    </StorySurface>
  ),
};

/** VC-14 — adoption refused because the metadata resolves into the main checkout. */
export const AdoptRefusal: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <WorktreeAdoptDialog
        open
        onOpenChange={fn()}
        discovered={discoveredWorktreeFixture}
        refusal={refusal({
          code: "adoption_main_checkout",
          message:
            "Its metadata resolves into the main checkout, not a linked worktree. Nothing was changed.",
          risk: null,
        })}
        isAdopting={false}
        onConfirm={fn()}
      />
    </StorySurface>
  ),
};

/** VC-28 — clean removal: the branch is explicitly not deleted. */
export const RemoveClean: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <WorktreeRemoveDialog
        open
        onOpenChange={fn()}
        worktree={cleanWorktree}
        refusal={null}
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

/** Issue #813 — the explicit opt-in to delete the local branch after removal. */
export const RemoveWithBranchDeletion: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <WorktreeRemoveDialog
        open
        onOpenChange={fn()}
        worktree={cleanWorktree}
        refusal={null}
        forceRequested={false}
        onRequestForce={fn()}
        isRemoving={false}
        deleteBranch
        onDeleteBranchChange={fn()}
        onConfirm={fn()}
      />
    </StorySurface>
  ),
};

/** VC-29 — a bound idle session will be stopped, and its history survives. */
export const RemoveBoundIdle: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <WorktreeRemoveDialog
        open
        onOpenChange={fn()}
        worktree={cleanWorktree}
        refusal={null}
        forceRequested={false}
        onRequestForce={fn()}
        boundSessions={{ running: 0, idle: 1 }}
        isRemoving={false}
        deleteBranch={false}
        onDeleteBranchChange={fn()}
        onConfirm={fn()}
      />
    </StorySurface>
  ),
};

/** VC-30 — a session mid-turn blocks removal outright. */
export const RemoveBoundRunning: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <WorktreeRemoveDialog
        open
        onOpenChange={fn()}
        worktree={cleanWorktree}
        refusal={refusal({ code: "worktree_session_active", risk: null })}
        forceRequested={false}
        onRequestForce={fn()}
        boundSessions={{ running: 1, idle: 0 }}
        isRemoving={false}
        deleteBranch={false}
        onDeleteBranchChange={fn()}
        onConfirm={fn()}
        onOpenSession={fn()}
      />
    </StorySurface>
  ),
};

/** VC-31 — dirty refusal with quantified risk rows and the exit flow as primary. */
export const RemoveDirtyRefusal: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <WorktreeRemoveDialog
        open
        onOpenChange={fn()}
        worktree={worktreeReadyDirtyRunningFixture}
        refusal={refusal({})}
        forceRequested={false}
        onRequestForce={fn()}
        isRemoving={false}
        deleteBranch={false}
        onDeleteBranchChange={fn()}
        onConfirm={fn()}
        onCommitAndPush={fn()}
      />
    </StorySurface>
  ),
};

/** VC-32 — force confirm re-states the exact quantities before destroying them. */
export const RemoveForce: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <WorktreeRemoveDialog
        open
        onOpenChange={fn()}
        worktree={worktreeReadyDirtyRunningFixture}
        refusal={refusal({})}
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

/** VC-33 — the branch also exists on a remote, so the refusal downgrades. */
export const RemoveRemoteDowngrade: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <WorktreeRemoveDialog
        open
        onOpenChange={fn()}
        worktree={buildWorktreeFixture({ name: "data-migration" })}
        refusal={refusal({
          downgrade: true,
          risk: { ...DIRTY_RISK, changedFiles: 0, unpushedCommits: 1, existsOnRemote: true },
        })}
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

/** VC-34 — missing resolution, choice view: dismiss the record, or say it's back. */
export const MissingResolution: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <WorktreeMissingResolutionDialog
        open
        onOpenChange={fn()}
        worktree={worktreeMissingFixture}
        outcome={null}
        refusal={null}
        isPending={false}
        deleteBranch={false}
        onDeleteBranchChange={fn()}
        onDismissRecord={fn()}
        onRestore={fn()}
      />
    </StorySurface>
  ),
};

/** VC-34 — missing resolution, outcome view: the idempotent no-op, rendered verbatim. */
export const MissingResolutionIdempotentOutcome: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <WorktreeMissingResolutionDialog
        open
        onOpenChange={fn()}
        worktree={worktreeMissingFixture}
        outcome="Nothing to clean up — no branch, worktree directory, or instance data found."
        refusal={null}
        isPending={false}
        deleteBranch={false}
        onDeleteBranchChange={fn()}
        onDismissRecord={fn()}
        onRestore={fn()}
      />
    </StorySurface>
  ),
};

const bulkRows = [
  buildWorktreeFixture({
    id: "wt_bulk_dirty",
    name: "draft-settings",
    branch: "run/draft-settings",
    dirty: true,
    agent_activity: "idle",
  }),
  buildWorktreeFixture({
    id: "wt_bulk_unique",
    name: "local-fix",
    branch: "run/local-fix",
    dirty: false,
    agent_activity: "idle",
  }),
];

export const BulkForceConfirmation: Story = {
  tags: ["play-fn"],
  parameters: {
    msw: {
      handlers: [
        compozyApiMock.get(
          "/api/workspaces/{workspace_id}/worktrees/{worktree_id}",
          ({ params }) => {
            const worktree = bulkRows.find(row => row.id === params.worktree_id);
            return worktree
              ? HttpResponse.json({
                  worktree,
                  bindings: { agent_activity: "idle" },
                  repo: { git_backed: true, git_available: true },
                  status: null,
                  forge: null,
                })
              : HttpResponse.json({ error: "Worktree not found." }, { status: 404 });
          }
        ),
        compozyApiMock.delete(
          "/api/workspaces/{workspace_id}/worktrees/{worktree_id}",
          ({ params, request }) => {
            const query = new URL(request.url).searchParams;
            if (query.get("force") === "true") {
              const row = bulkRows.find(row => row.id === params.worktree_id);
              return query.get("delete_branch") === "true" && row
                ? HttpResponse.json({ branch: { name: row.branch, deleted: true } })
                : new HttpResponse(null, { status: 204 });
            }
            const dirty = params.worktree_id === bulkRows[0].id;
            return HttpResponse.json(
              {
                code: dirty ? "worktree_dirty_requires_force" : "worktree_unpushed_requires_force",
                error: "Removing this worktree would discard local work.",
                risk: {
                  changed_files: dirty ? 2 : 0,
                  insertions: dirty ? 14 : 0,
                  deletions: dirty ? 3 : 0,
                  unpushed_commits: dirty ? 0 : 1,
                  exists_on_remote: false,
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
      batch={{ workspaceId: bulkRows[0].workspace_id, worktrees: bulkRows }}
      onClose={fn()}
    />
  ),
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(
      await body.findByRole("checkbox", { name: "Also delete local branches" })
    );
    await userEvent.click(await body.findByRole("button", { name: "Remove selected" }));
    await userEvent.click(await body.findByRole("button", { name: "Force remove… (2)" }));
    await body.findByTestId("worktree-bulk-force-dialog");
  },
};

/** Discovered checkouts join the batch by Git path: stale entries clear, live loss still needs force. */
export const BulkRemoveDiscovered: Story = {
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
                code: "worktree_unpushed_requires_force",
                error: "Removing this worktree would discard local work.",
                risk: {
                  changed_files: 0,
                  insertions: 0,
                  deletions: 0,
                  unpushed_commits: 2,
                  exists_on_remote: false,
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
        discovered: [discoveredWorktreeFixture, discoveredStaleWorktreeFixture],
      }}
      onClose={fn()}
    />
  ),
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(await body.findByRole("button", { name: "Remove selected" }));
    await body.findByText("Stale Git entry cleared. Branch kept.");
    await userEvent.click(await body.findByRole("button", { name: "Force remove… (1)" }));
    await body.findByTestId("worktree-bulk-force-dialog");
  },
};
