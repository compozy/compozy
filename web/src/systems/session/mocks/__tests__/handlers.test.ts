// Suite: Session Storybook handlers
// Invariant: concrete session endpoints keep their API semantics ahead of parameterized IDs.
// Boundary IN: the MSW boundary used by Session and full-app route stories.
// Boundary OUT: EventSource reconciliation owned by hooks/__tests__/use-session-catalog-streams.test.tsx.
import { setupServer } from "msw/node";
import { afterAll, beforeAll, beforeEach, describe, expect, it } from "vitest";

import { buildLocalNetworkParticipationFixture } from "@/test/network-participation-fixtures";

import { handlers, resetSessionAttachmentMock, seedSessionAttachmentMock } from "../handlers";
import { sessionFixtures } from "../fixtures";

const server = setupServer(...handlers);
const API = "http://localhost";

function multipartUpload(filename: string, mimeType: string, contents: string) {
  const boundary = "compozy-session-attachment-test";
  const body = new TextEncoder().encode(
    [
      `--${boundary}`,
      `Content-Disposition: form-data; name="file"; filename="${filename}"`,
      `Content-Type: ${mimeType}`,
      "",
      contents,
      `--${boundary}--`,
      "",
    ].join("\r\n")
  );
  return {
    body: body.buffer,
    headers: { "content-type": `multipart/form-data; boundary=${boundary}` },
  };
}

beforeAll(() => {
  server.listen({ onUnhandledRequest: "error" });
});

beforeEach(() => {
  server.resetHandlers();
  resetSessionAttachmentMock();
});

afterAll(() => {
  server.close();
});

describe("session MSW handlers", () => {
  it("Should upload, read, and delete one attachment within its workspace", async () => {
    const sample = sessionFixtures[0]!;
    const workspaceId = sample.workspace_id;
    if (!workspaceId) throw new Error("session fixture must include workspace_id");
    const uploadBody = multipartUpload("notes.txt", "text/plain", "notes");

    const upload = await fetch(
      `${API}/api/workspaces/${workspaceId}/sessions/${sample.id}/attachments`,
      { ...uploadBody, method: "POST" }
    );
    const uploaded = (await upload.json()) as { attachment: { id: string; mime_type: string } };

    expect(upload.status).toBe(201);
    expect(uploaded.attachment).toMatchObject({ mime_type: "text/plain" });
    const bytes = await fetch(
      `${API}/api/workspaces/${workspaceId}/sessions/${sample.id}/attachments/${uploaded.attachment.id}/bytes`
    );
    expect(bytes.status).toBe(200);
    expect(bytes.headers.get("content-type")).toBe("text/plain");
    await expect(bytes.text()).resolves.toBe("notes");

    const remove = await fetch(
      `${API}/api/workspaces/${workspaceId}/sessions/${sample.id}/attachments/${uploaded.attachment.id}`,
      { method: "DELETE" }
    );
    expect(remove.status).toBe(204);
    const missing = await fetch(
      `${API}/api/workspaces/${workspaceId}/sessions/${sample.id}/attachments/${uploaded.attachment.id}/bytes`
    );
    expect(missing.status).toBe(404);
  });

  it("Should reject unsupported attachment uploads", async () => {
    const sample = sessionFixtures[0]!;
    const workspaceId = sample.workspace_id;
    if (!workspaceId) throw new Error("session fixture must include workspace_id");
    const uploadBody = multipartUpload("archive.zip", "application/zip", "binary");

    const response = await fetch(
      `${API}/api/workspaces/${workspaceId}/sessions/${sample.id}/attachments`,
      { ...uploadBody, method: "POST" }
    );

    expect(response.status).toBe(415);
    await expect(response.json()).resolves.toEqual({ error: "unsupported mime type" });
  });

  it("Should resolve omitted or Local participation to the canonical Local snapshot", async () => {
    const response = await fetch(`${API}/api/sessions`, {
      body: JSON.stringify({
        agent_name: "codex",
        network_participation: { mode: "local" },
        workspace: "workspace_local",
      }),
      headers: { "content-type": "application/json" },
      method: "POST",
    });
    const body = (await response.json()) as {
      session: {
        resolved_network_participation: {
          bounds: { max_wakes: number };
          mode: string;
          source: string;
        };
      };
    };

    expect(response.status).toBe(201);
    expect(body.session.resolved_network_participation).toEqual(
      buildLocalNetworkParticipationFixture()
    );
  });

  it("Should return a durable unbound record without accepting a first message", async () => {
    const createSession = async () => {
      const response = await fetch(`${API}/api/sessions`, {
        body: JSON.stringify({
          agent_name: "codex",
          workspace: "workspace_local",
        }),
        headers: { "content-type": "application/json" },
        method: "POST",
      });
      const payload = (await response.json()) as {
        session: { runtime: { status: string }; state: string };
      };
      return {
        runtimeStatus: payload.session.runtime.status,
        state: payload.session.state,
        status: response.status,
      };
    };

    expect(await createSession()).toEqual({
      runtimeStatus: "unbound",
      state: "active",
      status: 201,
    });
  });

  it("Should resolve a valid named Live request to a workspace-owned bounded snapshot", async () => {
    const response = await fetch(`${API}/api/sessions`, {
      body: JSON.stringify({
        agent_name: "codex",
        network_participation: {
          channel_id: " release-room ",
          channel_strategy: "named",
          mode: "live",
        },
        workspace: "workspace_live",
      }),
      headers: { "content-type": "application/json" },
      method: "POST",
    });
    const body = (await response.json()) as {
      session: {
        resolved_network_participation: {
          bounds: { max_wakes: number };
          channel_id: string;
          channel_strategy: string;
          mode: string;
          source: string;
          workspace_id: string;
        };
      };
    };

    expect(response.status).toBe(201);
    expect(body.session.resolved_network_participation).toMatchObject({
      bounds: { max_wakes: 8 },
      channel_id: "release-room",
      channel_strategy: "named",
      mode: "live",
      source: "explicit_request",
      workspace_id: "workspace_live",
    });
  });

  it("Should filter listSessions by workspace and agent", async () => {
    const sample = sessionFixtures[0]!;
    const workspace = sample.workspace_id!;
    const agent = sample.agent_name!;

    const filtered = await fetch(
      `${API}/api/sessions?workspace=${encodeURIComponent(workspace)}&agent=${encodeURIComponent(agent)}`
    );
    expect(filtered.status).toBe(200);
    const body = (await filtered.json()) as {
      sessions: Array<{ agent_name: string; workspace_id: string }>;
    };
    expect(body.sessions.length).toBeGreaterThan(0);
    expect(body.sessions.every(session => session.workspace_id === workspace)).toBe(true);
    expect(body.sessions.every(session => session.agent_name === agent)).toBe(true);

    const workspaceOnly = await fetch(
      `${API}/api/sessions?workspace=${encodeURIComponent(workspace)}`
    );
    const workspaceBody = (await workspaceOnly.json()) as {
      sessions: Array<{ workspace_id: string }>;
    };
    expect(workspaceBody.sessions.every(session => session.workspace_id === workspace)).toBe(true);
    expect(workspaceBody.sessions.length).toBeGreaterThanOrEqual(body.sessions.length);
  });

  it("Should preserve the not-found response for an unknown session ID", async () => {
    const response = await fetch(`${API}/api/sessions/session_missing`);

    expect(response.status).toBe(404);
    await expect(response.json()).resolves.toEqual({
      error: "Session not found: session_missing",
    });
  });

  it("Should expose an empty clarification projection for a known session", async () => {
    const sample = sessionFixtures[0]!;
    const response = await fetch(
      `${API}/api/workspaces/${sample.workspace_id}/sessions/${sample.id}/clarifications`
    );

    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toEqual({ clarifications: [] });
  });

  it("Should hide a known session's clarifications from another workspace", async () => {
    const sample = sessionFixtures[0]!;
    const response = await fetch(
      `${API}/api/workspaces/workspace_other/sessions/${sample.id}/clarifications`
    );

    expect(response.status).toBe(404);
    await expect(response.json()).resolves.toEqual({
      error: `Session not found: ${sample.id}`,
    });
  });

  it("Should scope attachment deletion to the session workspace", async () => {
    const sample = sessionFixtures[0]!;
    const workspaceId = sample.workspace_id;
    if (!workspaceId) {
      throw new Error("session fixture must include workspace_id");
    }
    const attachmentId = `att_${"a".repeat(64)}`;
    seedSessionAttachmentMock(
      workspaceId,
      sample.id,
      {
        bytes: 16,
        created_at: "2026-04-17T18:11:00Z",
        height: 0,
        id: attachmentId,
        kind: "file",
        mime_type: "text/plain",
        name: "owned.txt",
        sha256: "a".repeat(64),
        width: 0,
      },
      new TextEncoder().encode("owned attachment")
    );

    const wrongWorkspace = await fetch(
      `${API}/api/workspaces/workspace_other/sessions/${sample.id}/attachments/${attachmentId}`,
      { method: "DELETE" }
    );

    expect(wrongWorkspace.status).toBe(404);
    await expect(wrongWorkspace.json()).resolves.toEqual({
      error: `Session not found: ${sample.id}`,
    });

    const owned = await fetch(
      `${API}/api/workspaces/${sample.workspace_id}/sessions/${sample.id}/attachments/${attachmentId}`,
      { method: "DELETE" }
    );
    expect(owned.status).toBe(204);
    await expect(owned.text()).resolves.toBe("");
  });
});
