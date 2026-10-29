import { HttpResponse, type HttpHandler } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";

import {
  applyWorkspaceFolderOrder,
  applyWorkspaceFolderRemoval,
  applyWorkspaceMove,
  applyWorkspaceOrder,
} from "../lib/workspace-folders";
import type { WorkspaceFolderPayload, WorkspaceLayoutResponse } from "../types";

const STORY_TIME = "2026-10-29T12:00:00Z";
let layout: WorkspaceLayoutResponse = { folders: [], placements: [] };

/** Clears the in-memory project folders between stories and tests. */
export function resetWorkspaceLayoutMock(): void {
  layout = { folders: [], placements: [] };
}

/** Seeds project folders (in order) and placements for a story or test. */
export function seedWorkspaceLayoutMock(seeded: WorkspaceLayoutResponse): void {
  layout = {
    folders: seeded.folders.map((folder, position) => ({ ...folder, position })),
    placements: seeded.placements,
  };
}

/** A project folder fixture with its counts filled in. */
export function workspaceFolderFixture(id: string, name: string): WorkspaceFolderPayload {
  return {
    id,
    name,
    position: 0,
    profile_id: "default",
    workspace_count: 0,
    created_at: STORY_TIME,
    updated_at: STORY_TIME,
  };
}

function served(): WorkspaceLayoutResponse {
  return {
    folders: layout.folders.map((folder, position) => ({
      ...folder,
      position,
      workspace_count: layout.placements.filter(row => row.folder_id === folder.id).length,
    })),
    placements: layout.placements,
  };
}

/** Project folder and project order routes over one in-memory layout. */
export function workspaceLayoutHandlers(): HttpHandler[] {
  const respond = () => HttpResponse.json<WorkspaceLayoutResponse>(served());
  return [
    compozyApiMock.get("/api/workspace-folders", respond),
    compozyApiMock.post("/api/workspace-folders", async ({ request }) => {
      const body = (await request.json()) as { name: string };
      const folder = workspaceFolderFixture(
        `wfold-${Math.random().toString(36).slice(2, 10)}`,
        body.name
      );
      layout = { ...layout, folders: [folder, ...layout.folders] };
      return HttpResponse.json({ folder }, { status: 201 });
    }),
    compozyApiMock.patch("/api/workspace-folders/{folder_id}", async ({ params, request }) => {
      const body = (await request.json()) as { name: string };
      layout = {
        ...layout,
        folders: layout.folders.map(folder =>
          folder.id === params.folder_id ? { ...folder, name: body.name } : folder
        ),
      };
      const folder = served().folders.find(candidate => candidate.id === params.folder_id);
      return folder
        ? HttpResponse.json({ folder })
        : HttpResponse.json({ error: "folder not found" }, { status: 404 });
    }),
    compozyApiMock.delete("/api/workspace-folders/{folder_id}", ({ params }) => {
      layout = applyWorkspaceFolderRemoval(layout, String(params.folder_id));
      return respond();
    }),
    compozyApiMock.put("/api/workspace-order/folders", async ({ request }) => {
      const body = (await request.json()) as { folder_ids: string[] };
      layout = applyWorkspaceFolderOrder(layout, body.folder_ids);
      layout = { ...layout, folders: [...layout.folders].sort((a, b) => a.position - b.position) };
      return respond();
    }),
    compozyApiMock.put("/api/workspace-order/workspaces", async ({ request }) => {
      const body = (await request.json()) as { folder_id?: string; workspace_ids: string[] };
      layout = applyWorkspaceOrder(layout, body.folder_id ?? null, body.workspace_ids);
      return respond();
    }),
    compozyApiMock.post("/api/workspace-order/move", async ({ request }) => {
      const body = (await request.json()) as { folder_id?: string; workspace_id: string };
      layout = applyWorkspaceMove(layout, body.workspace_id, body.folder_id ?? null);
      return respond();
    }),
  ];
}
