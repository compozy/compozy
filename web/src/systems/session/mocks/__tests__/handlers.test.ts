import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";

import { buildLocalNetworkParticipationFixture } from "@/test/network-participation-fixtures";

import { handlers } from "../handlers";
import { sessionFixtures } from "../fixtures";

const server = setupServer(...handlers);
const API = "http://localhost";

beforeAll(() => {
  server.listen({ onUnhandledRequest: "error" });
});

afterEach(() => {
  server.resetHandlers();
});

afterAll(() => {
  server.close();
});

describe("session MSW handlers", () => {
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

  it("Should return a durable starting record whether or not a first message is staged", async () => {
    const createSession = async (body: Record<string, unknown>) => {
      const response = await fetch(`${API}/api/sessions`, {
        body: JSON.stringify({ agent_name: "codex", workspace: "workspace_local", ...body }),
        headers: { "content-type": "application/json" },
        method: "POST",
      });
      const payload = (await response.json()) as { session: { state: string } };
      return { state: payload.session.state, status: response.status };
    };

    expect(await createSession({ prompt: "Draft the release notes." })).toEqual({
      state: "starting",
      status: 201,
    });
    expect(await createSession({})).toEqual({ state: "starting", status: 201 });
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
});
