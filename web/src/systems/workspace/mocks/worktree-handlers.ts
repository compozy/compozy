import { HttpResponse, type HttpHandler } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";
import { storyWorkspaceIds } from "@/storybook/fintech-scenario";

import {
  buildWorktreeFixture,
  discoveredWorktreeFixture,
  nonGitWorktreeListingFixture,
  worktreeListingFixture,
} from "./worktree-fixtures";

/**
 * Every worktree route a mounted hook can reach. The Storybook `guard` group
 * errors loudly on any unhandled `/api` call, so a missing handler here is a
 * broken story rather than a silent empty list.
 *
 * Only the story's git-backed workspace has worktrees; the rest answer
 * truthfully with a non-git repo so their nests stay absent.
 */
export const worktreeHandlers: HttpHandler[] = [
  compozyApiMock.get("/api/workspaces/{workspace_id}/worktrees", ({ params }) => {
    const workspaceId = String(params.workspace_id);
    return HttpResponse.json(
      workspaceId === storyWorkspaceIds.hq ? worktreeListingFixture : nonGitWorktreeListingFixture
    );
  }),

  compozyApiMock.post("/api/workspaces/{workspace_id}/worktrees", async ({ request, params }) => {
    const body = (await request.json()) as { name?: string; branch?: string };
    const name = body.name?.trim() || "steady-heron";

    if (worktreeListingFixture.worktrees.some(worktree => worktree.name === name)) {
      return HttpResponse.json(
        { code: "worktree_name_taken", error: `A worktree named ${name} already exists.` },
        { status: 409 }
      );
    }

    // 202: the row is durable and pending; materialization continues daemon-side.
    return HttpResponse.json(
      {
        worktree: buildWorktreeFixture({
          name,
          workspace_id: String(params.workspace_id),
          state: "pending",
          pending_phase: "branch",
          setup_state: "none",
          dirty: null,
          ahead: null,
          behind: null,
          ...(body.branch?.trim() ? { branch: body.branch.trim() } : {}),
        }),
      },
      { status: 202 }
    );
  }),

  compozyApiMock.post("/api/workspaces/{workspace_id}/worktrees/adopt", async ({ request }) => {
    const body = (await request.json()) as { path?: string };
    const path = body.path?.trim();

    if (path !== discoveredWorktreeFixture.path) {
      return HttpResponse.json(
        {
          code: "adoption_main_checkout",
          error: "Its metadata resolves into the main checkout, not a linked worktree.",
        },
        { status: 409 }
      );
    }

    return HttpResponse.json({
      worktree: buildWorktreeFixture({
        name: discoveredWorktreeFixture.name,
        branch: discoveredWorktreeFixture.branch,
        // Adopted externals keep their foreign path.
        path: discoveredWorktreeFixture.path,
        origin: "adopted",
        setup_state: "none",
      }),
    });
  }),

  compozyApiMock.post(
    "/api/workspaces/{workspace_id}/worktrees/{worktree_id}/cancel",
    () => new HttpResponse(null, { status: 204 })
  ),

  compozyApiMock.post(
    "/api/workspaces/{workspace_id}/worktrees/{worktree_id}/dismiss",
    () => new HttpResponse(null, { status: 204 })
  ),

  compozyApiMock.delete(
    "/api/workspaces/{workspace_id}/worktrees/{worktree_id}",
    ({ request, params }) => {
      const force = new URL(request.url).searchParams.get("force") === "true";
      const worktreeId = String(params.worktree_id);
      const dirty = worktreeListingFixture.worktrees.find(
        worktree => worktree.id === worktreeId
      )?.dirty;

      // Without force, a dirty tree refuses with the quantified risk payload.
      if (dirty === true && !force) {
        return HttpResponse.json(
          {
            code: "worktree_dirty_requires_force",
            error: "The worktree holds work that exists nowhere else.",
            downgrade: false,
            risk: {
              changed_files: 6,
              insertions: 142,
              deletions: 18,
              unpushed_commits: 3,
              exists_on_remote: false,
            },
          },
          { status: 409 }
        );
      }

      return new HttpResponse(null, { status: 204 });
    }
  ),
];
