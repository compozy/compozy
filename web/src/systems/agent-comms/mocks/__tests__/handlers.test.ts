import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { createMswFetch } from "@/test/msw-fetch";

import {
  buildCallFixture,
  buildCallMessageFixture,
  buildLargeTreeFixture,
  callFixtureRootSessionId,
  callFixtureWorkspaceId,
  completedCallFixture,
  handlers,
  invalidResultCallFixture,
  nineStateCallsFixture,
  resetAgentCommsMockState,
  runningCallFixture,
  setAgentCommsMockCalls,
  setAgentCommsMockMessages,
  silentFinishCallFixture,
} from "..";

const fetchMsw = createMswFetch(() => handlers);

beforeEach(() => {
  resetAgentCommsMockState();
});

afterEach(() => {
  resetAgentCommsMockState();
});

function callsUrl(query: Record<string, string>): string {
  const params = new URLSearchParams({ profile: "default", ...query });
  return `http://localhost/api/workspaces/${callFixtureWorkspaceId}/calls?${params}`;
}

/** This workspace surface uses workspace routes; Global records have separate routes. */
function callUrl(callId: string, suffix = ""): string {
  return `http://localhost/api/workspaces/${callFixtureWorkspaceId}/calls/${callId}${suffix}?profile=default`;
}

async function readCalls(query: Record<string, string>) {
  const response = await fetchMsw(callsUrl(query));
  expect(response.status).toBe(200);
  return (await response.json()) as {
    items: { call_id: string; state: string }[];
    total: number;
    next_cursor?: string;
  };
}

describe("agent-comms handlers — counted pages", () => {
  it("Should count the whole filtered population, not the page it returned", () => {
    setAgentCommsMockCalls(buildLargeTreeFixture(150));
    return readCalls({ limit: "10" }).then(body => {
      expect(body.items).toHaveLength(10);
      expect(body.total).toBe(150);
      expect(body.next_cursor).toBe("10");
    });
  });

  it("Should keep the total steady while the cursor advances", async () => {
    setAgentCommsMockCalls(buildLargeTreeFixture(150));
    const first = await readCalls({ limit: "10" });
    const second = await readCalls({ limit: "10", cursor: first.next_cursor! });

    expect(second.total).toBe(first.total);
    expect(second.items[0]!.call_id).not.toBe(first.items[0]!.call_id);
  });

  it("Should answer a limit=1 count probe with one row and the real total", async () => {
    setAgentCommsMockCalls(buildLargeTreeFixture(47));
    const body = await readCalls({ limit: "1" });

    expect(body.items).toHaveLength(1);
    expect(body.total).toBe(47);
  });

  it("Should stop offering a cursor on the last page", async () => {
    setAgentCommsMockCalls(buildLargeTreeFixture(3));
    const body = await readCalls({ limit: "10" });

    expect(body.total).toBe(3);
    expect(body.next_cursor).toBeUndefined();
  });
});

describe("agent-comms handlers — filters", () => {
  it("Should honour a comma-separated state subset", async () => {
    setAgentCommsMockCalls(nineStateCallsFixture);
    const body = await readCalls({ state: "invalid-result,completed-without-result" });

    expect(body.total).toBe(2);
    expect(body.items.map(item => item.state).sort()).toEqual([
      "completed-without-result",
      "invalid-result",
    ]);
  });

  it("Should separate the Made and Received directions", async () => {
    const made = await readCalls({ caller: callFixtureRootSessionId });
    const received = await readCalls({
      child_session_id: completedCallFixture.child_session_id!,
    });

    expect(made.total).toBeGreaterThan(0);
    expect(received.total).toBe(1);
    expect(received.items[0]!.call_id).toBe(completedCallFixture.call_id);
  });

  it("Should scope one delegation tree by its governed root", async () => {
    const body = await readCalls({ root_session_id: callFixtureRootSessionId });

    expect(body.total).toBeGreaterThan(0);
    expect(body.items.every(item => item.call_id.length > 0)).toBe(true);
  });

  it("Should scope one agent definition by name", async () => {
    const body = await readCalls({ agent: completedCallFixture.agent! });

    expect(body.total).toBeGreaterThan(0);
  });

  it("Should return an empty counted page when nothing matches", async () => {
    const body = await readCalls({ agent: "no-such-agent" });

    expect(body.items).toEqual([]);
    expect(body.total).toBe(0);
  });
});

/**
 * Invariant: `attention=true` is the *unresolved* subset, not a state list.
 * Owned here because the mock is what every attention hook and story reads —
 * if it matched on state alone, a "badge clears" test would pass while the badge
 * stayed lit forever.
 */
describe("agent-comms handlers — the attention population", () => {
  const child = invalidResultCallFixture.child_session_id!;

  // Each case stages its own mailbox: the default fixture mailbox already holds
  // a later operator message to this child, which legitimately resolves it.
  beforeEach(() => {
    setAgentCommsMockMessages([]);
  });

  it("Should hold a terminal call with no usable answer", async () => {
    setAgentCommsMockCalls([
      invalidResultCallFixture,
      silentFinishCallFixture,
      completedCallFixture,
    ]);
    const body = await readCalls({ attention: "true" });

    expect(body.total).toBe(2);
    expect(body.items.map(item => item.call_id).sort()).toEqual(
      [invalidResultCallFixture.call_id, silentFinishCallFixture.call_id].sort()
    );
  });

  it("Should drop it once a later call addresses the same child", async () => {
    setAgentCommsMockCalls([
      invalidResultCallFixture,
      buildCallFixture({
        call_id: "call_retry",
        child_session_id: child,
        state: "running",
        created_at: "2026-08-20T19:00:00Z",
      }),
    ]);
    const body = await readCalls({ attention: "true" });

    expect(body.total).toBe(0);
    // The historical call is still queryable by state — resolved, not erased.
    expect((await readCalls({ state: "invalid-result" })).total).toBe(1);
  });

  it("Should drop it once a later message reaches the same child", async () => {
    setAgentCommsMockCalls([invalidResultCallFixture]);
    setAgentCommsMockMessages([
      buildCallMessageFixture({
        message_id: "msg_followup",
        to_session_id: child,
        created_at: "2026-08-20T19:05:00Z",
      }),
    ]);

    expect((await readCalls({ attention: "true" })).total).toBe(0);
  });

  it("Should ignore a message that predates the call it would resolve", async () => {
    setAgentCommsMockCalls([invalidResultCallFixture]);
    setAgentCommsMockMessages([
      buildCallMessageFixture({
        message_id: "msg_earlier",
        to_session_id: child,
        created_at: "2026-08-20T17:00:00Z",
      }),
    ]);

    expect((await readCalls({ attention: "true" })).total).toBe(1);
  });
});

describe("agent-comms handlers — one call", () => {
  it("Should answer 404 with the typed code for an unknown call", async () => {
    const response = await fetchMsw(callUrl("call_missing"));
    expect(response.status).toBe(404);
    expect(await response.json()).toMatchObject({ code: "call_target_not_found" });
  });

  it("Should refuse a result before the call has one", async () => {
    setAgentCommsMockCalls([runningCallFixture]);
    const response = await fetchMsw(callUrl(runningCallFixture.call_id, "/result"));

    expect(response.status).toBe(409);
    expect(await response.json()).toMatchObject({ code: "call_not_settled" });
  });

  it("Should treat cancelling a settled call as an idempotent no-op", async () => {
    const response = await fetchMsw(callUrl(completedCallFixture.call_id, "/cancel"), {
      method: "POST",
      body: JSON.stringify({ reason: "too late" }),
    });

    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ state: "completed" });
  });

  it("Should refuse records outside the routed workspace and profile", async () => {
    const wrongWorkspace = callUrl(completedCallFixture.call_id).replace(
      callFixtureWorkspaceId,
      "ws_other"
    );
    const wrongProfile = callUrl(completedCallFixture.call_id).replace(
      "profile=default",
      "profile=other"
    );

    expect((await fetchMsw(wrongWorkspace)).status).toBe(404);
    expect((await fetchMsw(wrongProfile)).status).toBe(404);
    const wrongProfileCancel = callUrl(completedCallFixture.call_id, "/cancel").replace(
      "profile=default",
      "profile=other"
    );
    expect((await fetchMsw(wrongProfileCancel, { method: "POST", body: "{}" })).status).toBe(404);
    const wrongProfileList = callsUrl({}).replace("profile=default", "profile=other");
    expect(((await (await fetchMsw(wrongProfileList)).json()) as { total: number }).total).toBe(0);
  });
});

describe("agent-comms handlers — messages", () => {
  it("Should page the mailbox without a total, because the daemon computes none", async () => {
    const response = await fetchMsw(
      `http://localhost/api/workspaces/${callFixtureWorkspaceId}/messages?profile=default`
    );
    const body = (await response.json()) as Record<string, unknown>;

    expect(response.status).toBe(200);
    expect(Array.isArray(body.items)).toBe(true);
    expect(body).not.toHaveProperty("total");
  });

  it("Should read the nested wire target when creating a call", async () => {
    const response = await fetchMsw(
      `http://localhost/api/workspaces/${callFixtureWorkspaceId}/calls?profile=default`,
      {
        method: "POST",
        body: JSON.stringify({ target: { agent: "reviewer" }, prompt: "Review this" }),
      }
    );

    expect(response.status).toBe(201);
    const created = (await response.json()) as { call_id: string; child_session_id?: string };
    expect(created.child_session_id).toBe(`ses_${created.call_id}`);
    const reviewerCalls = await readCalls({ agent: "reviewer" });
    expect(reviewerCalls.items.some(call => call.call_id === created.call_id)).toBe(true);
    const detail = await fetchMsw(callUrl(created.call_id));
    expect(await detail.json()).not.toHaveProperty("result_bytes");
  });

  it("Should read the nested wire recipient when sending a message", async () => {
    const childSessionId = completedCallFixture.child_session_id!;
    const response = await fetchMsw(
      `http://localhost/api/workspaces/${callFixtureWorkspaceId}/messages?profile=default`,
      {
        method: "POST",
        body: JSON.stringify({
          call_id: completedCallFixture.call_id,
          to: { session_id: childSessionId },
          text: "Please retry",
        }),
      }
    );

    expect(response.status).toBe(202);
    const messages = await fetchMsw(
      `http://localhost/api/workspaces/${callFixtureWorkspaceId}/messages?profile=default&session=${childSessionId}`
    );
    const body = (await messages.json()) as {
      items: { call_id?: string; text: string; to_session_id: string }[];
    };
    expect(body.items).toContainEqual(
      expect.objectContaining({
        call_id: completedCallFixture.call_id,
        text: "Please retry",
        to_session_id: childSessionId,
      })
    );
  });
});
