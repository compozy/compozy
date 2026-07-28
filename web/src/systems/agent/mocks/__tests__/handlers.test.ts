import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";

import { handlers, resetAgentMockState } from "../handlers";
import { FIXTURE_AGENT_DEFINITION_DIGEST, primaryAgentFixture } from "../fixtures";

const server = setupServer(...handlers);
const API = "http://localhost";

beforeAll(() => {
  server.listen({ onUnhandledRequest: "error" });
});

afterEach(() => {
  resetAgentMockState();
  server.resetHandlers();
});

afterAll(() => {
  server.close();
});

describe("agent MSW handlers", () => {
  it("Should filter the workspace catalog before returning its counted page", async () => {
    const response = await fetch(
      `${API}/api/agents/catalog?workspace=ws-test&q=${encodeURIComponent(primaryAgentFixture.name)}&status=active&limit=1`
    );

    expect(response.status).toBe(200);
    const body = (await response.json()) as {
      agents: Array<{
        agent: { name: string };
        sessions: { active: number; failed: number; runtime_seconds: number; total: number };
      }>;
      page: { limit: number; total: number };
      sessions_available: boolean;
    };
    expect(body.agents).toHaveLength(1);
    expect(body.agents[0]?.agent.name).toBe(primaryAgentFixture.name);
    expect(body.agents[0]?.sessions.active).toBeGreaterThan(0);
    expect(body.agents[0]?.sessions).toMatchObject({
      failed: 0,
      runtime_seconds: 0,
    });
    expect(body.page).toMatchObject({ limit: 1, total: 1 });
    expect(body.sessions_available).toBe(true);
  });

  it("Should round-trip update, delete, and duplicate", async () => {
    const name = primaryAgentFixture.name;

    const conflict = await fetch(`${API}/api/agents/${name}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        expected_digest: "stale",
        agent: { ...primaryAgentFixture, prompt: "Nope" },
      }),
    });
    expect(conflict.status).toBe(409);

    const updated = await fetch(`${API}/api/agents/${name}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        expected_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
        agent: { ...primaryAgentFixture, prompt: "Updated via MSW" },
      }),
    });
    expect(updated.status).toBe(200);
    const updatedBody = (await updated.json()) as {
      agent: { prompt: string; definition_digest: string };
    };
    expect(updatedBody.agent.prompt).toBe("Updated via MSW");
    expect(updatedBody.agent.definition_digest).not.toBe(FIXTURE_AGENT_DEFINITION_DIGEST);

    const duplicated = await fetch(`${API}/api/agents/${name}/duplicate`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        name: `${name}-copy`,
        scope: "global",
        overrides: { prompt: "Copy prompt" },
      }),
    });
    expect(duplicated.status).toBe(201);
    const dupBody = (await duplicated.json()) as { agent: { name: string; prompt: string } };
    expect(dupBody.agent.name).toBe(`${name}-copy`);
    expect(dupBody.agent.prompt).toBe("Copy prompt");

    const deleted = await fetch(`${API}/api/agents/${name}-copy`, { method: "DELETE" });
    expect(deleted.status).toBe(200);
    const deletedBody = (await deleted.json()) as { name: string };
    expect(deletedBody.name).toBe(`${name}-copy`);
  });

  it("Should force duplicate origin from explicit scope across the workspace and global boundary", async () => {
    const name = primaryAgentFixture.name;

    const toWorkspace = await fetch(`${API}/api/agents/${name}/duplicate`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        name: `${name}-ws`,
        scope: "workspace",
        workspace: "ws-test",
      }),
    });
    expect(toWorkspace.status).toBe(201);
    const workspaceCopy = (await toWorkspace.json()) as {
      agent: { name: string; origin: string };
    };
    expect(workspaceCopy.agent.origin).toBe("workspace");

    const toGlobal = await fetch(`${API}/api/agents/${name}-ws/duplicate`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        name: `${name}-global`,
        scope: "global",
      }),
    });
    expect(toGlobal.status).toBe(201);
    const globalCopy = (await toGlobal.json()) as { agent: { name: string; origin: string } };
    expect(globalCopy.agent.origin).toBe("global");

    const inherited = await fetch(`${API}/api/agents/${name}-ws/duplicate`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        name: `${name}-inherited`,
      }),
    });
    expect(inherited.status).toBe(201);
    const inheritedCopy = (await inherited.json()) as { agent: { origin: string } };
    expect(inheritedCopy.agent.origin).toBe("workspace");
  });

  it("Should round-trip soul and heartbeat authored-file routes", async () => {
    const name = primaryAgentFixture.name;

    const soulGet = await fetch(`${API}/api/agents/${name}/soul`);
    expect(soulGet.status).toBe(200);

    const soulPut = await fetch(`${API}/api/agents/${name}/soul`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        body: "Be helpful.",
        expected_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
      }),
    });
    expect(soulPut.status).toBe(200);
    const soulBody = (await soulPut.json()) as { soul: { digest: string; body: string } };
    expect(soulBody.soul.body).toBe("Be helpful.");

    const soulConflict = await fetch(`${API}/api/agents/${name}/soul`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ body: "stale", expected_digest: "nope" }),
    });
    expect(soulConflict.status).toBe(409);

    const hbPut = await fetch(`${API}/api/agents/${name}/heartbeat`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        body: "---\nenabled: true\n---\n",
        expected_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
      }),
    });
    expect(hbPut.status).toBe(200);

    const status = await fetch(`${API}/api/agents/${name}/heartbeat/status`);
    expect(status.status).toBe(200);
    const statusBody = (await status.json()) as { active: boolean };
    expect(statusBody.active).toBe(true);

    const wake = await fetch(`${API}/api/agents/${name}/heartbeat/wake`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ session_id: "sess-1", source: "manual" }),
    });
    expect(wake.status).toBe(200);
  });

  it("Should 404 authored-file mutations for unknown agents", async () => {
    const missing = "missing-agent";
    const putSoul = await fetch(`${API}/api/agents/${missing}/soul`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        body: "orphan",
        expected_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
      }),
    });
    expect(putSoul.status).toBe(404);

    const wake = await fetch(`${API}/api/agents/${missing}/heartbeat/wake`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ session_id: "sess-1", source: "manual" }),
    });
    expect(wake.status).toBe(404);

    const status = await fetch(`${API}/api/agents/${missing}/heartbeat/status`);
    expect(status.status).toBe(404);
  });
});
