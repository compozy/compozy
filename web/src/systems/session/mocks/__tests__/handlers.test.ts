// Suite: Session Storybook handlers
// Invariant: concrete session endpoints keep their API semantics ahead of parameterized IDs.
// Boundary IN: the MSW boundary used by Session and full-app route stories.
// Boundary OUT: EventSource reconciliation owned by hooks/__tests__/use-session-catalog-streams.test.tsx.
import { setupServer } from "msw/node";
import { afterAll, beforeAll, beforeEach, describe, expect, it } from "vitest";

import { buildLocalNetworkParticipationFixture } from "@/test/network-participation-fixtures";

import {
  handlers,
  resetSessionAttachmentMock,
  resetSessionAttentionMock,
  seedSessionAttachmentMock,
} from "../handlers";
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
  resetSessionAttentionMock();
});

afterAll(() => {
  server.close();
});

describe("session MSW handlers", () => {
  it("Should enforce workspace and operation validation for Goal control", async () => {
    const sample = sessionFixtures[0]!;
    const workspaceId = sample.workspace_id;
    if (!workspaceId) throw new Error("session fixture must include workspace_id");

    const foreignWorkspace = await fetch(
      `${API}/api/workspaces/workspace-foreign/sessions/${sample.id}/goal`,
      {
        body: JSON.stringify({ operation: "status" }),
        headers: { "content-type": "application/json" },
        method: "POST",
      }
    );
    expect(foreignWorkspace.status).toBe(404);

    const invalidOperation = await fetch(
      `${API}/api/workspaces/${workspaceId}/sessions/${sample.id}/goal`,
      {
        body: JSON.stringify({ operation: "unknown" }),
        headers: { "content-type": "application/json" },
        method: "POST",
      }
    );
    expect(invalidOperation.status).toBe(400);
    await expect(invalidOperation.json()).resolves.toEqual({ error: "Invalid Goal operation" });
  });

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
      `${API}/api/sessions?workspace_id=${encodeURIComponent(workspace)}&agent=${encodeURIComponent(agent)}`
    );
    expect(filtered.status).toBe(200);
    const body = (await filtered.json()) as {
      sessions: Array<{ agent_name: string; workspace_id: string }>;
    };
    expect(body.sessions.length).toBeGreaterThan(0);
    expect(body.sessions.every(session => session.workspace_id === workspace)).toBe(true);
    expect(body.sessions.every(session => session.agent_name === agent)).toBe(true);

    const workspaceOnly = await fetch(
      `${API}/api/sessions?workspace_id=${encodeURIComponent(workspace)}`
    );
    const workspaceBody = (await workspaceOnly.json()) as {
      sessions: Array<{ workspace_id: string }>;
    };
    expect(workspaceBody.sessions.every(session => session.workspace_id === workspace)).toBe(true);
    expect(workspaceBody.sessions.length).toBeGreaterThanOrEqual(body.sessions.length);
  });

  it("Should filter listSessions by a named profile and widen only for aggregate reads", async () => {
    const marketingResponse = await fetch(`${API}/api/sessions?profile=marketing`);
    const marketing = (await marketingResponse.json()) as {
      sessions: Array<{ profile_name: string }>;
    };
    expect(marketing.sessions.length).toBeGreaterThan(0);
    expect(marketing.sessions.every(session => session.profile_name === "marketing")).toBe(true);

    const aggregateResponse = await fetch(`${API}/api/sessions?all_profiles=true`);
    const aggregate = (await aggregateResponse.json()) as {
      sessions: Array<{ profile_name: string }>;
    };
    expect(new Set(aggregate.sessions.map(session => session.profile_name))).toEqual(
      new Set(["default", "marketing"])
    );
  });

  it("Should expose canonical attention filters, summary, interactions, and presence", async () => {
    const attentionResponse = await fetch(`${API}/api/sessions?attention=true`);
    const attention = (await attentionResponse.json()) as {
      sessions: Array<{ badge: string; id: string }>;
    };
    expect(attentionResponse.status).toBe(200);
    expect(attention.sessions.map(session => session.badge).sort()).toEqual([
      "waiting-for-auth",
      "waiting-for-input",
    ]);

    const doneResponse = await fetch(`${API}/api/sessions?badge=done`);
    const done = (await doneResponse.json()) as { sessions: Array<{ badge: string }> };
    expect(done.sessions).toHaveLength(1);
    expect(done.sessions[0]?.badge).toBe("done");

    const summaryResponse = await fetch(`${API}/api/sessions/attention-summary`);
    await expect(summaryResponse.json()).resolves.toMatchObject({
      needs_you: 2,
      finished: 1,
    });

    const clarify = sessionFixtures.find(session => session.badge === "waiting-for-input");
    if (!clarify?.workspace_id) throw new Error("clarify fixture must include workspace_id");
    const interactionsResponse = await fetch(
      `${API}/api/workspaces/${clarify.workspace_id}/sessions/${clarify.id}/interactions?status=pending`
    );
    await expect(interactionsResponse.json()).resolves.toMatchObject({
      interactions: [
        {
          interaction_id: "interaction_story_clarify",
          kind: "clarify",
          status: "pending",
        },
      ],
    });

    const presenceURL = `${API}/api/workspaces/${clarify.workspace_id}/sessions/${clarify.id}/presence`;
    const acquire = await fetch(presenceURL, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ visible: true }),
    });
    const acquired = (await acquire.json()) as { lease_id: string };
    expect(acquire.status).toBe(200);
    const renew = await fetch(presenceURL, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ visible: true, lease_id: acquired.lease_id }),
    });
    expect(renew.status).toBe(204);
    const release = await fetch(presenceURL, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ visible: false, lease_id: acquired.lease_id }),
    });
    expect(release.status).toBe(204);
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
