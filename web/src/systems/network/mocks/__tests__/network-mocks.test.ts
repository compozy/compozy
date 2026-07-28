import { getResponse } from "msw";
import { describe, expect, it } from "vitest";

import {
  storyDefaultWorkspaceId,
  storyHeroNetworkChannel,
  storyPeerIds,
  storySessionIds,
} from "@/storybook/fintech-scenario";
import {
  networkDirectRoomMessagesFixture,
  networkDirectRoomsFixture,
  networkPeerFixture,
  networkSettlementPeerFixture,
  networkThreadMessagesFixture,
  networkThreadsFixture,
} from "../fixtures";
import { handlers } from "../handlers";

function networkUrl(path: string): string {
  return `http://localhost/api/workspaces/${storyDefaultWorkspaceId}/network${path}`;
}

describe("network mock contracts", () => {
  it("persists task-scoped coordination invitation revisions", async () => {
    const path = "/api/workspaces/ws-coordination/network-coordination";
    const query = "?scope=task&task_id=task-coordination&run_id=run-coordination";
    const initial = await getResponse(handlers, new Request(`http://localhost${path}${query}`), {
      baseUrl: "http://localhost",
    });
    const initialBody = (await initial?.json()) as {
      coordination: { eligibility: { eligible: boolean }; revision: number };
    };
    expect(initialBody.coordination.revision).toBe(0);
    expect(initialBody.coordination.eligibility.eligible).toBe(true);

    const dismissed = await getResponse(
      handlers,
      new Request(`http://localhost${path}/invitation`, {
        body: JSON.stringify({
          scope: "task",
          task_id: "task-coordination",
          run_id: "run-coordination",
          dismissed: true,
          expected_revision: 0,
        }),
        headers: { "content-type": "application/json" },
        method: "PUT",
      }),
      { baseUrl: "http://localhost" }
    );
    expect(dismissed?.status).toBe(200);

    const persisted = await getResponse(handlers, new Request(`http://localhost${path}${query}`), {
      baseUrl: "http://localhost",
    });
    const persistedBody = (await persisted?.json()) as {
      coordination: {
        eligibility: { eligible: boolean };
        invitation: { dismissed: boolean; task_id?: string };
        revision: number;
      };
    };
    expect(persistedBody.coordination).toMatchObject({
      revision: 1,
      invitation: { dismissed: true, task_id: "task-coordination" },
      eligibility: { eligible: false },
    });

    const stale = await getResponse(
      handlers,
      new Request(`http://localhost${path}`, {
        body: JSON.stringify({
          scope: "task",
          task_id: "task-coordination",
          enabled: true,
          expected_revision: 0,
        }),
        headers: { "content-type": "application/json" },
        method: "PUT",
      }),
      { baseUrl: "http://localhost" }
    );
    expect(stale?.status).toBe(409);
  });

  it("keeps thread message fixtures aligned with the conversation contract", () => {
    expect(networkThreadMessagesFixture[0]).toMatchObject({
      direction: "sent",
      display_name: "Northstar Launch Control",
      local: true,
      session_id: storySessionIds.product,
    });
    expect(networkDirectRoomMessagesFixture[0]).toMatchObject({
      direction: "sent",
      display_name: "Launch room command brief",
      local: true,
      session_id: storySessionIds.product,
    });

    for (const message of [
      networkThreadMessagesFixture[1],
      networkThreadMessagesFixture[5],
      networkDirectRoomMessagesFixture[1],
    ]) {
      expect(message).toBeDefined();
      expect(message?.session_id).toBeUndefined();
      expect(message?.direction).toBe("received");
    }

    expect(networkThreadMessagesFixture.length).toBeGreaterThanOrEqual(20);
  });

  it("returns the thread list with surface-aligned summary rows", async () => {
    const response = await getResponse(
      handlers,
      new Request(networkUrl(`/channels/${storyHeroNetworkChannel}/threads`)),
      { baseUrl: "http://localhost" }
    );

    expect(response).not.toBeUndefined();
    expect(response?.ok).toBe(true);

    const payload = (await response?.json()) as { threads: typeof networkThreadsFixture };
    expect(payload.threads.map(thread => thread.thread_id)).toEqual(
      networkThreadsFixture.map(thread => thread.thread_id)
    );
  });

  it("returns the direct room list with two-party membership", async () => {
    const response = await getResponse(
      handlers,
      new Request(networkUrl(`/channels/${storyHeroNetworkChannel}/directs`)),
      { baseUrl: "http://localhost" }
    );

    expect(response).not.toBeUndefined();
    expect(response?.ok).toBe(true);

    const payload = (await response?.json()) as { directs: typeof networkDirectRoomsFixture };
    expect(payload.directs[0]?.session_a < (payload.directs[0]?.session_b ?? "")).toBe(true);
  });

  it("returns thread messages without leaking direct surface fields", async () => {
    const thread = networkThreadsFixture[0];
    expect(thread).toBeDefined();
    if (!thread) {
      return;
    }
    const response = await getResponse(
      handlers,
      new Request(
        networkUrl(`/channels/${storyHeroNetworkChannel}/threads/${thread.thread_id}/messages`)
      ),
      { baseUrl: "http://localhost" }
    );
    expect(response?.ok).toBe(true);
    const payload = (await response?.json()) as {
      messages: { surface: string; thread_id: string; direct_id?: string }[];
    };
    for (const message of payload.messages) {
      expect(message.surface).toBe("thread");
      expect(message.thread_id).toBe(thread.thread_id);
    }
  });

  it("keeps peer detail mocks aligned with the local-only payload shape", async () => {
    const localResponse = await getResponse(
      handlers,
      new Request(networkUrl(`/peers/${storyPeerIds.local}`)),
      { baseUrl: "http://localhost" }
    );
    const partnerResponse = await getResponse(
      handlers,
      new Request(networkUrl(`/peers/${storyPeerIds.partner}`)),
      { baseUrl: "http://localhost" }
    );

    expect(localResponse).not.toBeUndefined();
    expect(partnerResponse).not.toBeUndefined();
    expect(localResponse?.ok).toBe(true);
    expect(partnerResponse?.ok).toBe(true);

    const localPayload = (await localResponse?.json()) as { peer: typeof networkPeerFixture };
    const partnerPayload = (await partnerResponse?.json()) as {
      peer: typeof networkSettlementPeerFixture;
    };

    expect(localPayload.peer.local).toBe(true);
    expect(localPayload.peer.joined_at).toBe("2026-04-17T14:00:00Z");
    expect(localPayload.peer.peer_card.peer_id).toBe(storyPeerIds.local);

    expect(partnerPayload.peer.local).toBe(true);
    expect(partnerPayload.peer.joined_at).toBe("2026-04-17T14:08:00Z");
    expect(partnerPayload.peer.peer_card.peer_id).toBe(storyPeerIds.partner);
  });

  it("rejects legacy direct kind sends and returns the canonical error", async () => {
    const response = await getResponse(
      handlers,
      new Request(networkUrl("/send"), {
        body: JSON.stringify({
          channel: storyHeroNetworkChannel,
          kind: "direct",
          session_id: storySessionIds.product,
          to: storyPeerIds.partner,
          workspace_id: storyDefaultWorkspaceId,
        }),
        method: "POST",
      }),
      { baseUrl: "http://localhost" }
    );
    expect(response?.status).toBe(400);
    const payload = (await response?.json()) as { error: string };
    expect(payload.error).toContain("legacy direct kind is not accepted");
  });

  it("handles network send requests without falling through to the real daemon", async () => {
    const response = await getResponse(
      handlers,
      new Request(networkUrl("/send"), {
        body: JSON.stringify({
          body: { text: "Please confirm whether the BR timeout copy is still blocked." },
          channel: storyHeroNetworkChannel,
          kind: "say",
          mentions: [storyPeerIds.partner],
          session_id: storySessionIds.product,
          surface: "thread",
          thread_id: "thread_launch_command",
          workspace_id: storyDefaultWorkspaceId,
        }),
        method: "POST",
      }),
      { baseUrl: "http://localhost" }
    );

    expect(response).not.toBeUndefined();
    expect(response?.ok).toBe(true);

    const payload = (await response?.json()) as {
      message: {
        channel: string;
        id: string;
        kind: string;
        mentions?: string[];
        session_id: string;
        surface?: string;
      };
    };

    expect(payload.message).toMatchObject({
      channel: storyHeroNetworkChannel,
      kind: "say",
      session_id: storySessionIds.product,
      surface: "thread",
    });
    expect(payload.message.mentions).toEqual([storyPeerIds.partner]);
  });

  it("handles channel policy updates and thread-scoped subscriptions", async () => {
    const policyResponse = await getResponse(
      handlers,
      new Request(networkUrl(`/channels/${storyHeroNetworkChannel}`), {
        body: JSON.stringify({
          fanout_policy: "coordinator",
          coordinator_peer_id: storyPeerIds.partner,
          purpose: "Updated launch policy",
        }),
        method: "PATCH",
      }),
      { baseUrl: "http://localhost" }
    );
    expect(policyResponse?.ok).toBe(true);
    const policyPayload = (await policyResponse?.json()) as {
      channel: { coordinator_peer_id?: string; fanout_policy?: string; purpose?: string };
    };
    expect(policyPayload.channel).toMatchObject({
      coordinator_peer_id: storyPeerIds.partner,
      fanout_policy: "coordinator",
      purpose: "Updated launch policy",
    });

    const subscriptionResponse = await getResponse(
      handlers,
      new Request(networkUrl(`/channels/${storyHeroNetworkChannel}/subscriptions`), {
        body: JSON.stringify({
          mode: "full",
          session_id: storySessionIds.product,
          thread_id: "thread_launch_command",
        }),
        method: "PUT",
      }),
      { baseUrl: "http://localhost" }
    );
    expect(subscriptionResponse?.ok).toBe(true);
    const subscriptionPayload = (await subscriptionResponse?.json()) as {
      subscription: { mode: string; session_id: string; thread_id?: string };
    };
    expect(subscriptionPayload.subscription).toMatchObject({
      mode: "full",
      session_id: storySessionIds.product,
      thread_id: "thread_launch_command",
    });
  });

  it("promotes a public thread to a draft task", async () => {
    const response = await getResponse(
      handlers,
      new Request(
        networkUrl(
          `/channels/${storyHeroNetworkChannel}/threads/thread_launch_command/promote-task`
        ),
        {
          body: JSON.stringify({
            origin_message_id: "msg_launch_001",
            title: "Launch command follow-up",
          }),
          method: "POST",
        }
      ),
      { baseUrl: "http://localhost" }
    );

    expect(response?.status).toBe(201);
    const payload = (await response?.json()) as {
      origin: { origin_message_id: string; task_id: string };
      task: {
        id: string;
        resolved_network_participation: null;
        status: string;
        title: string;
      };
    };
    expect(payload.origin.origin_message_id).toBe("msg_launch_001");
    expect(payload.origin.task_id).toBe(payload.task.id);
    expect(payload.task).toMatchObject({
      resolved_network_participation: null,
      status: "draft",
      title: "Launch command follow-up",
    });
  });

  it("returns a validation response for malformed network send requests", async () => {
    const response = await getResponse(
      handlers,
      new Request(networkUrl("/send"), {
        body: JSON.stringify({
          channel: 123,
          kind: "say",
          session_id: storySessionIds.product,
          workspace_id: storyDefaultWorkspaceId,
        }),
        method: "POST",
      }),
      { baseUrl: "http://localhost" }
    );

    expect(response).not.toBeUndefined();
    expect(response?.status).toBe(400);

    const payload = (await response?.json()) as { error: string };
    expect(payload.error).toBe("Session, channel, and kind are required.");
  });
});
