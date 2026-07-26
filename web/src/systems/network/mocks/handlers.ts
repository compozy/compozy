import { HttpResponse, type HttpHandler } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";

import {
  createNetworkChannelFixture,
  networkChannelFixture,
  networkChannelsFixture,
  networkDirectRoomDetailFixture,
  networkDirectRoomMessagesFixture,
  networkDirectRoomsFixture,
  networkPeerFixture,
  networkPeersFixture,
  networkSettlementPeerFixture,
  networkStatusFixture,
  networkThreadDetailFixture,
  networkThreadMessagesFixture,
  networkThreadsFixture,
  networkWorkFixture,
} from "./fixtures";
import { coordinationHandlers } from "./coordination-handlers";

function readRecord(value: unknown): Record<string, unknown> | null {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return null;
  }

  return value as Record<string, unknown>;
}

function readRequiredString(record: Record<string, unknown> | null, key: string): string | null {
  const value = record?.[key];
  if (typeof value !== "string") {
    return null;
  }

  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : null;
}

function readOptionalString(
  record: Record<string, unknown> | null,
  key: string
): string | undefined {
  const value = record?.[key];
  if (typeof value !== "string") {
    return undefined;
  }

  return value.trim() || undefined;
}

function readStringArray(record: Record<string, unknown> | null, key: string): string[] {
  const value = record?.[key];
  if (!Array.isArray(value)) {
    return [];
  }
  return value.filter((candidate): candidate is string => typeof candidate === "string");
}

function readTaskPriority(record: Record<string, unknown> | null) {
  const priority = readOptionalString(record, "priority");
  return priority === "urgent" || priority === "high" || priority === "low" ? priority : "medium";
}

export const handlers: HttpHandler[] = [
  aghApiMock.get("/api/network/status", () => HttpResponse.json({ network: networkStatusFixture })),
  ...coordinationHandlers,
  aghApiMock.get("/api/workspaces/{workspace_id}/network/usage", () =>
    HttpResponse.json({
      workspace_id: "ws-fixture",
      details: [],
      total: {
        wake_count: 0,
        reserved_wake_count: 0,
        actual_wake_count: 0,
        unavailable_wake_count: 0,
        charged_wall_time: "0s",
        input_tokens: 0,
        output_tokens: 0,
      },
    })
  ),
  aghApiMock.get("/api/workspaces/{workspace_id}/network/channels", () =>
    HttpResponse.json(networkChannelsFixture)
  ),
  aghApiMock.get("/api/workspaces/{workspace_id}/network/channels/{channel}", ({ params }) => {
    const channel = String(params.channel);

    if (!networkChannelsFixture.channels.some(candidate => candidate.channel === channel)) {
      return HttpResponse.json({ error: `Channel not found: ${channel}` }, { status: 404 });
    }

    return HttpResponse.json({
      channel: {
        ...networkChannelFixture,
        channel,
      },
    });
  }),
  aghApiMock.patch(
    "/api/workspaces/{workspace_id}/network/channels/{channel}",
    async ({ params, request }) => {
      const channel = String(params.channel);

      if (!networkChannelsFixture.channels.some(candidate => candidate.channel === channel)) {
        return HttpResponse.json({ error: `Channel not found: ${channel}` }, { status: 404 });
      }

      const body = readRecord(await request.json());
      return HttpResponse.json({
        channel: {
          ...networkChannelFixture,
          channel,
          coordinator_peer_id:
            readOptionalString(body, "coordinator_peer_id") ??
            networkChannelFixture.coordinator_peer_id,
          fanout_policy:
            readOptionalString(body, "fanout_policy") ?? networkChannelFixture.fanout_policy,
          purpose: readOptionalString(body, "purpose") ?? networkChannelFixture.purpose,
        },
      });
    }
  ),
  aghApiMock.get(
    "/api/workspaces/{workspace_id}/network/channels/{channel}/subscriptions",
    ({ params }) =>
      HttpResponse.json({
        subscriptions: [
          {
            channel: String(params.channel),
            created_at: "2026-04-17T18:10:00Z",
            mode: "full",
            session_id: "session_launch_coordination",
            thread_id: "thread_launch_command",
            updated_at: "2026-04-17T18:12:00Z",
            workspace_id: String(params.workspace_id),
          },
        ],
      })
  ),
  aghApiMock.put(
    "/api/workspaces/{workspace_id}/network/channels/{channel}/subscriptions",
    async ({ params, request }) => {
      const body = readRecord(await request.json());
      const sessionId = readRequiredString(body, "session_id");
      const mode = readRequiredString(body, "mode");

      if (!sessionId || !mode) {
        return HttpResponse.json({ error: "session_id and mode are required." }, { status: 400 });
      }

      return HttpResponse.json({
        subscription: {
          channel: String(params.channel),
          created_at: "2026-04-17T18:10:00Z",
          mode,
          session_id: sessionId,
          thread_id: readOptionalString(body, "thread_id"),
          updated_at: "2026-04-17T18:12:00Z",
          workspace_id: String(params.workspace_id),
        },
      });
    }
  ),
  aghApiMock.delete(
    "/api/workspaces/{workspace_id}/network/channels/{channel}/subscriptions/{session_id}",
    () => new HttpResponse(null, { status: 204 })
  ),
  aghApiMock.get(
    "/api/workspaces/{workspace_id}/network/channels/{channel}/threads",
    ({ params }) => {
      const channel = String(params.channel);

      if (!networkChannelsFixture.channels.some(candidate => candidate.channel === channel)) {
        return HttpResponse.json({ error: `Channel not found: ${channel}` }, { status: 404 });
      }

      return HttpResponse.json({
        threads: networkThreadsFixture.map(thread => ({ ...thread, channel })),
        page: {
          has_more: false,
          limit: networkThreadsFixture.length,
          total: networkThreadsFixture.length,
        },
      });
    }
  ),
  aghApiMock.get(
    "/api/workspaces/{workspace_id}/network/channels/{channel}/threads/{thread_id}",
    ({ params }) => {
      const channel = String(params.channel);
      const threadId = String(params.thread_id);

      if (!networkChannelsFixture.channels.some(candidate => candidate.channel === channel)) {
        return HttpResponse.json({ error: `Channel not found: ${channel}` }, { status: 404 });
      }

      return HttpResponse.json({
        thread: {
          ...networkThreadDetailFixture,
          channel,
          thread_id: threadId,
        },
        task_links: networkThreadDetailFixture.task_links ?? [],
      });
    }
  ),
  aghApiMock.post(
    "/api/workspaces/{workspace_id}/network/channels/{channel}/threads/{thread_id}/promote-task",
    async ({ params, request }) => {
      const channel = String(params.channel);
      const threadId = String(params.thread_id);
      const body = readRecord(await request.json());
      const originMessageId = readRequiredString(body, "origin_message_id");

      if (!originMessageId) {
        return HttpResponse.json({ error: "origin_message_id is required." }, { status: 400 });
      }

      const taskId = "task_story_thread_promoted";
      const now = "2026-04-17T18:20:00Z";
      const title =
        readOptionalString(body, "title") ?? networkThreadDetailFixture.title ?? "Thread follow-up";
      return HttpResponse.json(
        {
          origin: {
            channel,
            created_at: now,
            digest: title,
            origin_message_id: originMessageId,
            source_message_ids: [originMessageId],
            task_id: taskId,
            thread_id: threadId,
            updated_at: now,
            workspace_id: String(params.workspace_id),
          },
          task: {
            created_at: now,
            created_by: { kind: "network_peer", ref: "peer_northstar_launch_control" },
            description:
              readOptionalString(body, "description") ??
              "Task promoted from an AGH Network thread.",
            id: taskId,
            latest_event_seq: 1,
            resolved_network_participation: null,
            origin: { kind: "network", ref: `${channel}/${threadId}` },
            priority: readTaskPriority(body),
            scope: "workspace",
            status: "draft",
            title,
            updated_at: now,
            wake_creator: false,
            workspace_id: String(params.workspace_id),
          },
        },
        { status: 201 }
      );
    }
  ),
  aghApiMock.get(
    "/api/workspaces/{workspace_id}/network/channels/{channel}/threads/{thread_id}/messages",
    ({ params }) => {
      const channel = String(params.channel);
      const threadId = String(params.thread_id);

      if (!networkChannelsFixture.channels.some(candidate => candidate.channel === channel)) {
        return HttpResponse.json({ error: `Channel not found: ${channel}` }, { status: 404 });
      }

      return HttpResponse.json({
        messages: networkThreadMessagesFixture.map(message => ({
          ...message,
          channel,
          surface: "thread",
          thread_id: threadId,
        })),
        page: { has_more: false, limit: networkThreadMessagesFixture.length },
      });
    }
  ),
  aghApiMock.get(
    "/api/workspaces/{workspace_id}/network/channels/{channel}/directs",
    ({ params }) => {
      const channel = String(params.channel);

      if (!networkChannelsFixture.channels.some(candidate => candidate.channel === channel)) {
        return HttpResponse.json({ error: `Channel not found: ${channel}` }, { status: 404 });
      }

      return HttpResponse.json({
        directs: networkDirectRoomsFixture.map(direct => ({ ...direct, channel })),
        page: {
          has_more: false,
          limit: networkDirectRoomsFixture.length,
          total: networkDirectRoomsFixture.length,
        },
      });
    }
  ),
  aghApiMock.post(
    "/api/workspaces/{workspace_id}/network/channels/{channel}/directs/resolve",
    async ({ params, request }) => {
      const channel = String(params.channel);
      const body = readRecord(await request.json());
      const peerId = readRequiredString(body, "peer_id");
      const sessionId = readRequiredString(body, "session_id");

      if (!networkChannelsFixture.channels.some(candidate => candidate.channel === channel)) {
        return HttpResponse.json({ error: `Channel not found: ${channel}` }, { status: 404 });
      }

      if (!peerId || !sessionId) {
        return HttpResponse.json(
          { error: "peer_id and session_id are required to resolve a direct room." },
          { status: 400 }
        );
      }

      return HttpResponse.json({
        direct: {
          ...networkDirectRoomDetailFixture,
          channel,
        },
      });
    }
  ),
  aghApiMock.get(
    "/api/workspaces/{workspace_id}/network/channels/{channel}/directs/{direct_id}",
    ({ params }) => {
      const channel = String(params.channel);
      const directId = String(params.direct_id);

      if (!networkChannelsFixture.channels.some(candidate => candidate.channel === channel)) {
        return HttpResponse.json({ error: `Channel not found: ${channel}` }, { status: 404 });
      }

      return HttpResponse.json({
        direct: {
          ...networkDirectRoomDetailFixture,
          channel,
          direct_id: directId,
        },
      });
    }
  ),
  aghApiMock.get(
    "/api/workspaces/{workspace_id}/network/channels/{channel}/directs/{direct_id}/messages",
    ({ params }) => {
      const channel = String(params.channel);
      const directId = String(params.direct_id);

      if (!networkChannelsFixture.channels.some(candidate => candidate.channel === channel)) {
        return HttpResponse.json({ error: `Channel not found: ${channel}` }, { status: 404 });
      }

      return HttpResponse.json({
        messages: networkDirectRoomMessagesFixture.map(message => ({
          ...message,
          channel,
          surface: "direct",
          direct_id: directId,
        })),
        page: { has_more: false, limit: networkDirectRoomMessagesFixture.length },
      });
    }
  ),
  aghApiMock.get("/api/workspaces/{workspace_id}/network/work/{work_id}", ({ params }) => {
    const workId = String(params.work_id);
    return HttpResponse.json({
      work: {
        ...networkWorkFixture,
        work_id: workId,
      },
    });
  }),
  aghApiMock.get("/api/workspaces/{workspace_id}/network/peers", ({ request }) => {
    const channel = new URL(request.url).searchParams.get("channel");
    const peers = channel
      ? networkPeersFixture.filter(peer => peer.channel === channel)
      : networkPeersFixture;

    return HttpResponse.json({ peers });
  }),
  aghApiMock.get("/api/workspaces/{workspace_id}/network/peers/{peer_id}", ({ params }) => {
    const peerId = String(params.peer_id);
    const peerSummary = networkPeersFixture.find(peer => peer.peer_id === peerId);

    if (!peerSummary) {
      return HttpResponse.json({ error: `Peer not found: ${peerId}` }, { status: 404 });
    }

    const baseDetail =
      peerSummary.peer_id === networkPeerFixture.peer_id
        ? networkPeerFixture
        : networkSettlementPeerFixture;

    return HttpResponse.json({
      peer: {
        ...baseDetail,
        channel: peerSummary.channel,
        joined_at: peerSummary.joined_at,
        local: peerSummary.local,
        peer_id: peerId,
        display_name: peerSummary.display_name ?? baseDetail.display_name,
        peer_card: peerSummary.peer_card,
        session_id: peerSummary.session_id,
      },
    });
  }),
  aghApiMock.post("/api/workspaces/{workspace_id}/network/channels", async ({ request }) => {
    const body = (await request.json()) as {
      agent_names?: string[];
      channel?: string;
      purpose?: string;
      workspace_id?: string;
    };

    if (!body.channel?.trim() || !body.workspace_id?.trim() || !body.purpose?.trim()) {
      return HttpResponse.json(
        { error: "Channel, workspace, and purpose are required." },
        { status: 400 }
      );
    }

    return HttpResponse.json(
      {
        channel: {
          ...createNetworkChannelFixture.channel,
          channel: body.channel.trim(),
          purpose: body.purpose.trim(),
          sessions: createNetworkChannelFixture.channel.sessions?.map((session, index) => ({
            ...session,
            id: `sess-created-${index + 1}`,
            agent_name: body.agent_names?.[index] ?? session.agent_name,
            workspace_id: body.workspace_id,
          })),
        },
      },
      { status: 201 }
    );
  }),
  aghApiMock.post("/api/workspaces/{workspace_id}/network/send", async ({ request }) => {
    const body = readRecord(await request.json());
    const sessionId = readRequiredString(body, "session_id");
    const channel = readRequiredString(body, "channel");
    const kind = readRequiredString(body, "kind");
    const surface = readOptionalString(body, "surface");

    if ((body != null && Object.hasOwn(body, "interaction_id")) || kind === "direct") {
      return HttpResponse.json(
        { error: "Use surface/direct_id/thread_id/work_id; legacy direct kind is not accepted." },
        { status: 400 }
      );
    }

    if (!sessionId || !channel || !kind) {
      return HttpResponse.json(
        { error: "Session, channel, and kind are required." },
        { status: 400 }
      );
    }

    // Test/storybook hook for thread collision: the special channel
    // `__collision__` rejects every thread creation so the UI can render the
    // collision toast pathway per `_design.md` §5.7.1.
    if (surface === "thread" && channel === "__collision__") {
      return HttpResponse.json({ error: "Thread already exists." }, { status: 409 });
    }

    return HttpResponse.json({
      message: {
        id: readOptionalString(body, "id") ?? "msg_risk_ops_sent",
        session_id: sessionId,
        channel,
        kind,
        surface: readOptionalString(body, "surface"),
        thread_id: readOptionalString(body, "thread_id"),
        direct_id: readOptionalString(body, "direct_id"),
        work_id: readOptionalString(body, "work_id"),
        mentions: readStringArray(body, "mentions"),
        to: readOptionalString(body, "to"),
        reply_to: readOptionalString(body, "reply_to"),
        trace_id: readOptionalString(body, "trace_id"),
        causation_id: readOptionalString(body, "causation_id"),
        expires_at: typeof body?.expires_at === "number" ? body.expires_at : undefined,
      },
    });
  }),
];
