import { HttpResponse, type HttpHandler } from "msw";
import { compozyApiMock } from "@/storybook/openapi-msw";
import { storyWorkspaceIds, storyWorkspaceNames } from "@/storybook/fintech-scenario";
import {
  buildLiveNetworkParticipationFixture,
  buildLocalNetworkParticipationFixture,
} from "@/test/network-participation-fixtures";

import {
  primarySessionFixture,
  sessionApprovalFixture,
  operatorNotificationFixture,
  sessionAttentionChangedFixture,
  sessionCatalogChangedFixture,
  sessionEventsFixture,
  sessionFixtures,
  sessionHistoryFixture,
  sessionRepairFixture,
  sessionTranscriptFixture,
} from "./fixtures";
import { uploadedAttachmentFromRequest } from "./session-attachment-upload";
import type { CreateSessionParams, SessionAttachment } from "../types";

const sessionById = new Map(sessionFixtures.map(session => [session.id, session]));
const storyWorkspaceNameById = new Map(
  Object.entries(storyWorkspaceIds).map(([key, id]) => [
    id as string,
    storyWorkspaceNames[key as keyof typeof storyWorkspaceNames],
  ])
);
const sessionCatalogStreamEncoder = new TextEncoder();
const attachmentBytes = new Map<string, { bytes: Uint8Array; attachment: SessionAttachment }>();
const presenceLeases = new Map<string, string>();

export function seedSessionAttachmentMock(
  workspaceId: string,
  sessionId: string,
  attachment: SessionAttachment,
  bytes: Uint8Array
): void {
  attachmentBytes.set(attachmentStoreKey(workspaceId, sessionId, attachment.id), {
    attachment,
    bytes: Uint8Array.from(bytes),
  });
}

export function resetSessionAttachmentMock(): void {
  attachmentBytes.clear();
}

export function resetSessionAttentionMock(): void {
  presenceLeases.clear();
}

function attachmentStoreKey(workspaceId: string, sessionId: string, attachmentId: string): string {
  return `${workspaceId}\u0000${sessionId}\u0000${attachmentId}`;
}

async function sha256Hex(bytes: Uint8Array): Promise<string> {
  const digest = await crypto.subtle.digest("SHA-256", Uint8Array.from(bytes).buffer);
  return Array.from(new Uint8Array(digest), byte => byte.toString(16).padStart(2, "0")).join("");
}

function mockAttachmentClassification(mimeType: string): {
  kind: "file" | "image";
  mimeType: string;
} | null {
  switch (mimeType) {
    case "image/png":
    case "image/jpeg":
    case "image/webp":
      return { kind: "image", mimeType };
    case "application/pdf":
    case "text/markdown":
    case "text/plain":
      return { kind: "file", mimeType };
    default:
      return null;
  }
}

function createSessionCatalogStreamResponse(): Response {
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(
        sessionCatalogStreamEncoder.encode(
          [
            ": storybook session catalog stream",
            "",
            "event: session_catalog_changed",
            `data: ${JSON.stringify(sessionCatalogChangedFixture)}`,
            "",
            "event: session_attention_changed",
            `data: ${JSON.stringify(sessionAttentionChangedFixture)}`,
            "",
            "event: operator_notification",
            `data: ${JSON.stringify(operatorNotificationFixture)}`,
            "",
            "",
          ].join("\n")
        )
      );
    },
  });

  return new Response(stream, {
    status: 200,
    headers: {
      "Cache-Control": "no-cache",
      Connection: "keep-alive",
      "Content-Type": "text/event-stream",
    },
  });
}

export const handlers: HttpHandler[] = [
  compozyApiMock.get("/api/sessions", ({ request }) => {
    const url = new URL(request.url);
    const workspace = url.searchParams.get("workspace_id")?.trim();
    const allProfiles = url.searchParams.get("all_profiles") === "true";
    const profile = url.searchParams.get("profile")?.trim() || "default";
    const agent = url.searchParams.get("agent")?.trim();
    const state = url.searchParams.get("state")?.trim();
    const resumable = url.searchParams.get("resumable");
    const attentionOnly = url.searchParams.get("attention") === "true";
    const badges = new Set(
      url.searchParams
        .getAll("badge")
        .flatMap(value => value.split(","))
        .map(value => value.trim())
        .filter(Boolean)
    );
    const archive = url.searchParams.get("archive");
    const limit = Math.max(1, Number(url.searchParams.get("limit") ?? 50));
    const sessions = [...sessionById.values()].filter(session => {
      if (!allProfiles && session.profile_name !== profile) return false;
      if (workspace && session.workspace_id !== workspace && session.workspace_path !== workspace) {
        return false;
      }
      if (agent && session.agent_name !== agent) return false;
      if (state && session.state !== state) return false;
      if (resumable === "true" && !session.attachable) return false;
      if (
        attentionOnly &&
        session.badge !== "waiting-for-input" &&
        session.badge !== "waiting-for-auth" &&
        session.badge !== "failed"
      ) {
        return false;
      }
      if (badges.size > 0 && !badges.has(session.badge)) return false;
      if (archive === "only" && session.archived_at === null) return false;
      if (archive !== "include" && archive !== "only" && session.archived_at !== null) return false;
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
  compozyApiMock.get("/api/sessions/catalog-stream", ({ response }) =>
    response.untyped(createSessionCatalogStreamResponse())
  ),
  compozyApiMock.get("/api/sessions/attention-summary", () => {
    const byWorkspace = new Map<string, { needs_you: number; finished: number }>();
    let needsYou = 0;
    let finished = 0;
    for (const session of sessionById.values()) {
      const workspaceId = session.workspace_id ?? "";
      if (workspaceId === "") continue;
      const totals = byWorkspace.get(workspaceId) ?? { needs_you: 0, finished: 0 };
      if (
        session.badge === "waiting-for-input" ||
        session.badge === "waiting-for-auth" ||
        session.badge === "failed"
      ) {
        needsYou += 1;
        totals.needs_you += 1;
      } else if (session.badge === "done") {
        finished += 1;
        totals.finished += 1;
      }
      byWorkspace.set(workspaceId, totals);
    }
    return HttpResponse.json({
      needs_you: needsYou,
      finished,
      by_workspace: [...byWorkspace.entries()]
        .map(([workspace_id, totals]) => ({ workspace_id, ...totals }))
        .sort((left, right) => left.workspace_id.localeCompare(right.workspace_id)),
    });
  }),
  compozyApiMock.get("/api/sessions/{session_id}", ({ params }) => {
    const id = String(params.session_id);
    const session = sessionById.get(id);

    if (!session) {
      return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json({ session });
  }),
  compozyApiMock.get("/api/sessions/{session_id}/owner", ({ params }) => {
    const id = String(params.session_id);
    const session = sessionById.get(id);

    if (!session) {
      return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
    }

    const workspaceId = session.workspace_id ?? "";
    return HttpResponse.json({
      session_id: id,
      workspace_id: workspaceId,
      workspace_name: storyWorkspaceNameById.get(workspaceId) ?? workspaceId,
    });
  }),
  compozyApiMock.post("/api/sessions", async ({ request }) => {
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

    // Creation accepts a durable, unbound session; prompt dispatch owns runtime binding.
    return HttpResponse.json(
      {
        session: {
          ...primarySessionFixture,
          id: `sess_${(body.name ?? body.agent_name ?? "story").replace(/[^a-zA-Z0-9]+/g, "_").toLowerCase()}`,
          name: body.name ?? primarySessionFixture.name,
          agent_name: body.agent_name ?? primarySessionFixture.agent_name,
          state: "active",
          runtime: { status: "unbound", selection_revision: 0 },
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
  compozyApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}", ({ params }) => {
    const id = String(params.session_id);
    const session = sessionById.get(id);

    if (!session) {
      return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json({ session });
  }),
  compozyApiMock.get(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/interactions",
    ({ params, request }) => {
      const workspaceId = String(params.workspace_id);
      const id = String(params.session_id);
      const session = sessionById.get(id);
      if (!session || session.workspace_id !== workspaceId) {
        return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
      }
      const status = new URL(request.url).searchParams.get("status")?.trim();
      return HttpResponse.json({
        interactions: session.pending_interactions.filter(
          interaction => !status || interaction.status === status
        ),
      });
    }
  ),
  compozyApiMock.post(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/presence",
    async ({ params, request }) => {
      const workspaceId = String(params.workspace_id);
      const id = String(params.session_id);
      const session = sessionById.get(id);
      if (!session || session.workspace_id !== workspaceId) {
        return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
      }
      const body = await request.json();
      if (body === null || typeof body !== "object") {
        return HttpResponse.json({ error: "presence request is required" }, { status: 400 });
      }
      const visible = Reflect.get(body, "visible");
      const leaseIdValue = Reflect.get(body, "lease_id");
      const leaseId = typeof leaseIdValue === "string" ? leaseIdValue.trim() : "";
      if (visible === true && leaseId === "") {
        const issued = `lease:${id}`;
        presenceLeases.set(issued, id);
        return HttpResponse.json({ lease_id: issued });
      }
      if ((visible === true || visible === false) && presenceLeases.get(leaseId) === id) {
        if (visible === false) presenceLeases.delete(leaseId);
        return new HttpResponse(null, { status: 204 });
      }
      return HttpResponse.json({ error: "presence lease not found" }, { status: 404 });
    }
  ),
  compozyApiMock.delete("/api/workspaces/{workspace_id}/sessions/{session_id}", ({ params }) => {
    const id = String(params.session_id);

    if (!sessionById.has(id)) {
      return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
    }

    return new HttpResponse(null, { status: 204 });
  }),
  compozyApiMock.post(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/archive",
    ({ params }) => {
      const id = String(params.session_id);
      const session = sessionById.get(id);

      if (!session) {
        return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
      }

      const archivedSession = {
        ...session,
        archived_at: "2026-04-17T18:15:00Z",
      };
      sessionById.set(id, archivedSession);
      return HttpResponse.json({ session: archivedSession });
    }
  ),
  compozyApiMock.post(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/unarchive",
    ({ params }) => {
      const id = String(params.session_id);
      const session = sessionById.get(id);

      if (!session) {
        return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
      }

      const unarchivedSession = { ...session, archived_at: null };
      sessionById.set(id, unarchivedSession);
      return HttpResponse.json({ session: unarchivedSession });
    }
  ),
  compozyApiMock.post(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/attachments",
    async ({ params, request }) => {
      const id = String(params.session_id);
      const workspaceId = String(params.workspace_id);
      const session = sessionById.get(id);
      if (!session || session.workspace_id !== workspaceId) {
        return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
      }
      const uploaded = await uploadedAttachmentFromRequest(request);
      if (!uploaded) {
        return HttpResponse.json({ error: "file is required" }, { status: 400 });
      }
      const classification = mockAttachmentClassification(uploaded.mimeType);
      if (!classification) {
        return HttpResponse.json({ error: "unsupported mime type" }, { status: 415 });
      }
      const bytes = uploaded.bytes;
      const digest = await sha256Hex(bytes);
      const attachment: SessionAttachment = {
        bytes: bytes.byteLength,
        created_at: "2026-04-17T18:11:00Z",
        height: 0,
        id: `att_${digest}`,
        kind: classification.kind,
        mime_type: classification.mimeType,
        name: uploaded.name,
        sha256: digest,
        width: 0,
      };
      seedSessionAttachmentMock(workspaceId, id, attachment, bytes);
      return HttpResponse.json({ attachment }, { status: 201 });
    }
  ),
  compozyApiMock.get(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/attachments/{attachment_id}/bytes",
    ({ params }) => {
      const workspaceId = String(params.workspace_id);
      const sessionId = String(params.session_id);
      const attachmentId = String(params.attachment_id);
      const stored = attachmentBytes.get(attachmentStoreKey(workspaceId, sessionId, attachmentId));
      if (!stored) {
        return HttpResponse.json({ error: "Attachment not found" }, { status: 404 });
      }
      return new Response(Uint8Array.from(stored.bytes).buffer, {
        status: 200,
        headers: { "Content-Type": stored.attachment.mime_type },
      });
    }
  ),
  compozyApiMock.delete(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/attachments/{attachment_id}",
    ({ params }) => {
      const id = String(params.session_id);
      const workspaceId = String(params.workspace_id);
      const session = sessionById.get(id);
      if (!session || session.workspace_id !== workspaceId) {
        return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
      }
      const attachmentId = String(params.attachment_id);
      const key = attachmentStoreKey(workspaceId, id, attachmentId);
      if (!attachmentBytes.delete(key)) {
        return HttpResponse.json({ error: "Attachment not found" }, { status: 404 });
      }
      return new HttpResponse(null, { status: 204 });
    }
  ),
  compozyApiMock.post(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/attach",
    ({ params }) => {
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
    }
  ),
  compozyApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}/recap", ({ params }) => {
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
  compozyApiMock.post(
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
  compozyApiMock.post(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/approve",
    ({ params }) => {
      const id = String(params.session_id);

      if (!sessionById.has(id)) {
        return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
      }

      return HttpResponse.json(sessionApprovalFixture);
    }
  ),
  compozyApiMock.get(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/events",
    ({ params }) => {
      const id = String(params.session_id);

      if (!sessionById.has(id)) {
        return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
      }

      return HttpResponse.json({ events: sessionEventsFixture });
    }
  ),
  compozyApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}/goal", ({ params }) => {
    const id = String(params.session_id);

    if (!sessionById.has(id)) {
      return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json({ goal: null });
  }),
  compozyApiMock.post(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/goal",
    async ({ params, request, response }) => {
      const id = String(params.session_id);
      const workspaceId = String(params.workspace_id);
      const session = sessionById.get(id);

      if (!session || session.workspace_id !== workspaceId) {
        return response.untyped(
          HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 })
        );
      }

      let body: unknown;
      try {
        body = await request.json();
      } catch {
        return response.untyped(
          HttpResponse.json({ error: "Invalid Goal request body" }, { status: 400 })
        );
      }

      const operation =
        typeof body === "object" && body !== null && "operation" in body
          ? body.operation
          : undefined;
      if (
        operation !== "set" &&
        operation !== "replace" &&
        operation !== "status" &&
        operation !== "pause" &&
        operation !== "resume" &&
        operation !== "clear"
      ) {
        return response.untyped(
          HttpResponse.json({ error: "Invalid Goal operation" }, { status: 400 })
        );
      }
      const outcome =
        operation === "set"
          ? "started"
          : operation === "replace"
            ? "replaced"
            : operation === "clear"
              ? "cleared"
              : operation === "pause"
                ? "paused"
                : operation === "resume"
                  ? "resumed"
                  : "status";
      return response.untyped(
        HttpResponse.json({
          outcome,
          reason_code: null,
          replaced_run_id: null,
          snapshot: null,
        })
      );
    }
  ),
  compozyApiMock.get(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/clarifications",
    ({ params }) => {
      const id = String(params.session_id);
      const workspaceId = String(params.workspace_id);
      const session = sessionById.get(id);

      if (!session || session.workspace_id !== workspaceId) {
        return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
      }

      return HttpResponse.json({ clarifications: [] });
    }
  ),
  compozyApiMock.get(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/history",
    ({ params }) => {
      const id = String(params.session_id);

      if (!sessionById.has(id)) {
        return HttpResponse.json({ error: `Session not found: ${id}` }, { status: 404 });
      }

      return HttpResponse.json({ history: sessionHistoryFixture });
    }
  ),
  compozyApiMock.get(
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
