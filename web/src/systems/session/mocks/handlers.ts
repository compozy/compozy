import { HttpResponse, type HttpHandler } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";
import {
  buildLiveNetworkParticipationFixture,
  buildLocalNetworkParticipationFixture,
} from "@/test/network-participation-fixtures";

import {
  primarySessionFixture,
  sessionApprovalFixture,
  sessionEventsFixture,
  sessionFixtures,
  sessionHistoryFixture,
  sessionRepairFixture,
  sessionTranscriptFixture,
} from "./fixtures";
import type { CreateSessionParams } from "../types";

const sessionById = new Map(sessionFixtures.map(session => [session.id, session]));

export const handlers: HttpHandler[] = [
  aghApiMock.get("/api/sessions", ({ request }) => {
    const url = new URL(request.url);
    const workspace = url.searchParams.get("workspace")?.trim();
    const agent = url.searchParams.get("agent")?.trim();
    const state = url.searchParams.get("state")?.trim();
    const resumable = url.searchParams.get("resumable");
    const limit = Math.max(1, Number(url.searchParams.get("limit") ?? 50));
    const sessions = sessionFixtures.filter(session => {
      if (workspace && session.workspace_id !== workspace && session.workspace_path !== workspace) {
        return false;
      }
      if (agent && session.agent_name !== agent) return false;
      if (state && session.state !== state) return false;
      if (resumable === "true" && !session.attachable) return false;
      return true;
    });
    const pageSessions = sessions.slice(0, limit);
    return HttpResponse.json({
      sessions: pageSessions,
      page: {
        has_more: sessions.length > pageSessions.length,
        limit,
        total: sessions.length,
        ...(sessions.length > pageSessions.length ? { next_cursor: "story-page-2" } : {}),
      },
    });
  }),
  aghApiMock.get("/api/sessions/{session_id}", ({ params }) => {
    const id = String(params.session_id);
    const session = sessionById.get(id);

    if (!session) {
      return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json({ session });
  }),
  aghApiMock.post("/api/sessions", async ({ request }) => {
    // Use the adapter's generated request contract rather than a mirrored DTO.
    const body: CreateSessionParams = await request.json();

    const workspaceId = body.workspace ?? primarySessionFixture.workspace_id ?? "";
    const participation = body.network_participation;
    const namedLive =
      participation?.mode === "live" && participation.channel_strategy === "named"
        ? participation
        : undefined;
    const channelId = namedLive?.channel_id.trim() ?? "";
    if (participation?.mode === "live" && (!namedLive || !channelId)) {
      return HttpResponse.json(
        { error: "Session Live participation requires a named channel." },
        { status: 422 }
      );
    }

    // The durable 201 remains asynchronous; prompt dispatch belongs to the daemon.
    return HttpResponse.json(
      {
        session: {
          ...primarySessionFixture,
          id: `sess_${(body.name ?? body.agent_name ?? "story").replace(/[^a-zA-Z0-9]+/g, "_").toLowerCase()}`,
          name: body.name ?? primarySessionFixture.name,
          agent_name: body.agent_name ?? primarySessionFixture.agent_name,
          state: "starting",
          workspace_id: workspaceId,
          workspace_path:
            body.workspace_path ?? body.workspace ?? primarySessionFixture.workspace_path,
          resolved_network_participation:
            participation?.mode === "live"
              ? buildLiveNetworkParticipationFixture({
                  workspaceId,
                  channelId,
                })
              : buildLocalNetworkParticipationFixture(),
        },
      },
      { status: 201 }
    );
  }),
  aghApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}", ({ params }) => {
    const id = String(params.session_id);
    const session = sessionById.get(id);

    if (!session) {
      return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json({ session });
  }),
  aghApiMock.delete("/api/workspaces/{workspace_id}/sessions/{session_id}", ({ params }) => {
    const id = String(params.session_id);

    if (!sessionById.has(id)) {
      return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
    }

    return new HttpResponse(null, { status: 204 });
  }),
  aghApiMock.post("/api/workspaces/{workspace_id}/sessions/{session_id}/attach", ({ params }) => {
    const id = String(params.session_id);
    const session = sessionById.get(id);

    if (!session) {
      return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json({
      session: {
        ...session,
        attached_to: "web:storybook",
        attach_expires_at: "2026-04-17T18:12:00Z",
      },
      attach: {
        session_id: id,
        attached_to: "web:storybook",
        attach_expires_at: "2026-04-17T18:12:00Z",
        attached_at: "2026-04-17T18:11:00Z",
      },
    });
  }),
  aghApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}/recap", ({ params }) => {
    const id = String(params.session_id);
    const session = sessionById.get(id);

    if (!session) {
      return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json({
      recap: {
        session,
        recent_markers: [],
        recent_messages: sessionTranscriptFixture,
        pending_inputs: 0,
        pending_markers: 0,
        snapshot: {
          generated_at: "2026-04-17T18:11:00Z",
          event_cursor: sessionEventsFixture.length,
          transcript_cursor: sessionTranscriptFixture.length,
          queue_generation: 0,
          consistency: "read_snapshot",
        },
      },
    });
  }),
  aghApiMock.post(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/repair",
    ({ params, request }) => {
      const id = String(params.session_id);

      if (!sessionById.has(id)) {
        return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
      }

      const url = new URL(request.url);
      const dryRun = url.searchParams.get("dry_run") === "true";

      return HttpResponse.json({
        repair: {
          ...sessionRepairFixture,
          session_id: id,
          persisted: !dryRun,
          actions: sessionRepairFixture.actions.map(action => ({
            ...action,
            persisted: !dryRun,
          })),
        },
      });
    }
  ),
  aghApiMock.post("/api/workspaces/{workspace_id}/sessions/{session_id}/approve", ({ params }) => {
    const id = String(params.session_id);

    if (!sessionById.has(id)) {
      return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json(sessionApprovalFixture);
  }),
  aghApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}/events", ({ params }) => {
    const id = String(params.session_id);

    if (!sessionById.has(id)) {
      return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json({ events: sessionEventsFixture });
  }),
  aghApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}/goal", ({ params }) => {
    const id = String(params.session_id);

    if (!sessionById.has(id)) {
      return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json({ goal: null });
  }),
  aghApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}/history", ({ params }) => {
    const id = String(params.session_id);

    if (!sessionById.has(id)) {
      return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json({ history: sessionHistoryFixture });
  }),
  aghApiMock.get(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/transcript",
    ({ params }) => {
      const id = String(params.session_id);

      if (!sessionById.has(id)) {
        return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
      }

      return HttpResponse.json({
        entries: sessionTranscriptFixture.map((message, index) => ({
          message,
          sequence: index + 1,
          start_sequence: index + 1,
        })),
        epoch: 1,
        generation: 1,
        has_older: false,
        limit: 200,
        max_sequence: sessionTranscriptFixture.length,
      });
    }
  ),
];
