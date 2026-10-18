import { HttpResponse, type HttpHandler } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";

import type { SessionFolderPayload, SessionFoldersResponse, SessionPayload } from "../types";

const STORY_TIME = "2026-10-17T12:00:00Z";
const folders = new Map<string, SessionFolderPayload[]>();

/** Clears the in-memory folder layout between stories and tests. */
export function resetSessionFolderMock(): void {
  folders.clear();
}

/** Seeds one workspace's folders for a story or test. */
export function seedSessionFolderMock(workspaceId: string, seeded: SessionFolderPayload[]): void {
  folders.set(
    workspaceId,
    seeded.map((folder, position) => ({ ...folder, position }))
  );
}

/**
 * Session folder and manual-order routes over the shared session fixtures:
 * membership lives on each session's `folder_id`, so the catalog handler serves
 * moves without extra wiring.
 */
export function sessionFolderHandlers(sessions: Map<string, SessionPayload>): HttpHandler[] {
  const list = (workspaceId: string) => {
    const current = folders.get(workspaceId) ?? [];
    return current.map((folder, position) => ({
      ...folder,
      position,
      session_count: [...sessions.values()].filter(
        session => session.folder_id === folder.id && session.archived_at === null
      ).length,
    }));
  };
  const respond = (workspaceId: string) =>
    HttpResponse.json<SessionFoldersResponse>({ folders: list(workspaceId) });
  const setFolder = (session: SessionPayload, folderId: string | null) => {
    const { folder_id: _previous, ...rest } = session;
    sessions.set(
      session.id,
      folderId ? { ...rest, folder_id: folderId } : (rest as SessionPayload)
    );
  };
  return [
    compozyApiMock.get("/api/workspaces/{workspace_id}/session-folders", ({ params }) =>
      respond(String(params.workspace_id))
    ),
    compozyApiMock.post(
      "/api/workspaces/{workspace_id}/session-folders",
      async ({ params, request }) => {
        const workspaceId = String(params.workspace_id);
        const body = (await request.json()) as { name: string };
        const folder: SessionFolderPayload = {
          id: `sfold-${Math.random().toString(36).slice(2, 10)}`,
          workspace_id: workspaceId,
          profile_id: "default",
          name: body.name.trim(),
          position: 0,
          session_count: 0,
          created_at: STORY_TIME,
          updated_at: STORY_TIME,
        };
        folders.set(workspaceId, [folder, ...(folders.get(workspaceId) ?? [])]);
        return HttpResponse.json<{ folder: SessionFolderPayload }>({ folder }, { status: 201 });
      }
    ),
    compozyApiMock.patch(
      "/api/workspaces/{workspace_id}/session-folders/{folder_id}",
      async ({ params, request }) => {
        const workspaceId = String(params.workspace_id);
        const body = (await request.json()) as { name: string };
        const current = folders.get(workspaceId) ?? [];
        const folder = current.find(candidate => candidate.id === params.folder_id);
        if (!folder) return HttpResponse.json({ error: "Folder not found" }, { status: 404 });
        folder.name = body.name.trim();
        return HttpResponse.json<{ folder: SessionFolderPayload }>({ folder });
      }
    ),
    compozyApiMock.delete(
      "/api/workspaces/{workspace_id}/session-folders/{folder_id}",
      ({ params }) => {
        const workspaceId = String(params.workspace_id);
        const folderId = String(params.folder_id);
        folders.set(
          workspaceId,
          (folders.get(workspaceId) ?? []).filter(folder => folder.id !== folderId)
        );
        for (const session of sessions.values()) {
          if (session.folder_id === folderId) setFolder(session, null);
        }
        return respond(workspaceId);
      }
    ),
    compozyApiMock.put(
      "/api/workspaces/{workspace_id}/session-order/folders",
      async ({ params, request }) => {
        const workspaceId = String(params.workspace_id);
        const body = (await request.json()) as { folder_ids: string[] };
        const byId = new Map((folders.get(workspaceId) ?? []).map(folder => [folder.id, folder]));
        folders.set(
          workspaceId,
          body.folder_ids.flatMap(id => {
            const folder = byId.get(id);
            return folder ? [folder] : [];
          })
        );
        return respond(workspaceId);
      }
    ),
    compozyApiMock.put(
      "/api/workspaces/{workspace_id}/session-order/sessions",
      async ({ params, request }) => {
        const body = (await request.json()) as { folder_id?: string; session_ids: string[] };
        for (const id of body.session_ids) {
          const session = sessions.get(id);
          if (session) setFolder(session, body.folder_id ?? null);
        }
        return respond(String(params.workspace_id));
      }
    ),
    compozyApiMock.post(
      "/api/workspaces/{workspace_id}/sessions/{session_id}/move",
      async ({ params, request }) => {
        const body = (await request.json()) as { folder_id?: string };
        const session = sessions.get(String(params.session_id));
        if (!session) return HttpResponse.json({ error: "Session not found" }, { status: 404 });
        setFolder(session, body.folder_id ?? null);
        return respond(String(params.workspace_id));
      }
    ),
  ];
}
