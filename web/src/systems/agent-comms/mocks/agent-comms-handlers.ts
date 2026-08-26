/**
 * MSW handlers for calls and messages, built over one isolated store.
 *
 * Only the workspace routes are registered, because those are the only ones the
 * product calls: the daemon derives a call's scope from the route it arrives on,
 * so `/api/calls/{id}` addresses Global work regardless of where the call lives.
 * Mocking the global routes as well would let a wrong request render right.
 *
 * `createAgentCommsHandlers(dataset)` gives a caller its own store, so a story
 * can stage its own population without touching anyone else's — Storybook can
 * load two of these at once and neither sees the other's writes.
 */
import { HttpResponse, type HttpHandler } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";

import {
  createAgentCommsMockStore,
  type AgentCommsDataset,
  type AgentCommsMockStore,
} from "./agent-comms-mock-store";
import type {
  CallMessagePayload,
  CallPayload,
  CreateCallRequest,
  SendCallMessageRequest,
} from "../types";

const NOT_FOUND_CODE = "call_target_not_found";

function notFound(callId: string) {
  return { error: `call not found: ${callId}`, code: NOT_FOUND_CODE } as const;
}

/** A newly created call, in the state the daemon admits one: queued, unanswered. */
function acceptedCall(
  store: AgentCommsMockStore,
  callId: string,
  body: CreateCallRequest,
  workspaceId: string,
  profileName: string
): CallPayload {
  const template = store.snapshotCalls()[0];
  const prompt = body.prompt;
  const caller = template?.caller.id ?? "ses_operator";
  const childSessionId = body.target.session_id ?? `ses_${callId}`;
  const accepted = {
    ...(template as CallPayload),
    call_id: callId,
    state: "queued",
    verdict: "pending",
    created_at: new Date().toISOString(),
    settled_at: null,
    caller: { id: caller, kind: "session" },
    actor: { id: caller, kind: "operator" },
    ...(body.target.agent ? { agent: body.target.agent } : {}),
    child_session_id: childSessionId,
    prompt_preview: prompt,
    prompt_bytes: new TextEncoder().encode(prompt).length,
    repair_attempts: 0,
    workspace_id: workspaceId,
    profile_name: profileName,
  };
  delete accepted.result_preview;
  delete accepted.result_bytes;
  return accepted;
}

function deliveredMessage(
  store: AgentCommsMockStore,
  messageId: string,
  body: SendCallMessageRequest,
  workspaceId: string,
  profileName: string
): CallMessagePayload {
  const template = store.snapshotCalls()[0];
  return {
    message_id: messageId,
    to_session_id: body.to.session_id ?? "",
    text: body.text,
    from: { id: template?.caller.id ?? "ses_operator", kind: "operator" },
    delivery: "queued",
    attempts: 0,
    created_at: new Date().toISOString(),
    delivered_at: null,
    profile_id: template?.profile_id ?? "prof_default",
    workspace_id: workspaceId,
    scope: template?.scope ?? "workspace",
    ...(body.call_id ? { call_id: body.call_id } : {}),
    profile_name: profileName,
  } as CallMessagePayload;
}

export function buildAgentCommsHandlers(store: AgentCommsMockStore): HttpHandler[] {
  let created = 0;
  return [
    compozyApiMock.get("/api/workspaces/{workspace_id}/calls", ({ params, request }) =>
      HttpResponse.json(store.pageCalls(String(params.workspace_id), new URL(request.url)))
    ),
    // The typed `response(status)` helper is what keeps a mocked error honest:
    // the body has to match the shape that status actually declares, so a handler
    // cannot invent an error envelope the daemon would never send.
    compozyApiMock.get(
      "/api/workspaces/{workspace_id}/calls/{call_id}",
      ({ params, request, response }) => {
        const callId = String(params.call_id);
        const call = store.findCall(String(params.workspace_id), new URL(request.url), callId);
        if (!call) return response(404).json(notFound(callId));
        return response(200).json(call);
      }
    ),
    compozyApiMock.get(
      "/api/workspaces/{workspace_id}/calls/{call_id}/prompt",
      ({ params, request, response }) => {
        const callId = String(params.call_id);
        const call = store.findCall(String(params.workspace_id), new URL(request.url), callId);
        if (!call) return response(404).json(notFound(callId));
        return response(200).json({ call_id: callId, prompt: call.prompt_preview ?? "" });
      }
    ),
    compozyApiMock.get(
      "/api/workspaces/{workspace_id}/calls/{call_id}/result",
      ({ params, request, response }) => {
        const callId = String(params.call_id);
        const call = store.findCall(String(params.workspace_id), new URL(request.url), callId);
        if (!call) return response(404).json(notFound(callId));
        if (call.result_preview === undefined) {
          return response(409).json({
            error: "call is not settled with a result",
            code: "call_not_settled",
          });
        }
        return response(200).json({ call_id: callId, result: call.result_preview });
      }
    ),
    compozyApiMock.get(
      "/api/workspaces/{workspace_id}/calls/{call_id}/superseded",
      ({ params, request, response }) => {
        const callId = String(params.call_id);
        const call = store.findCall(String(params.workspace_id), new URL(request.url), callId);
        if (!call || call.superseded_preview === undefined) {
          return response(404).json({
            error: `no superseded evidence for ${callId}`,
            code: NOT_FOUND_CODE,
          });
        }
        return response(200).json({ call_id: callId, result: call.superseded_preview });
      }
    ),
    compozyApiMock.post(
      "/api/workspaces/{workspace_id}/calls",
      async ({ params, request, response }) => {
        created += 1;
        const callId = `call_mock_${created}`;
        const body = (await request.json()) as CreateCallRequest;
        const url = new URL(request.url);
        const call = acceptedCall(
          store,
          callId,
          body,
          String(params.workspace_id),
          url.searchParams.get("profile") ?? "default"
        );
        store.addCall(call);
        return response(201).json({
          call_id: callId,
          state: "queued",
          idle_expires_at: call.idle_expires_at,
          replayed: false,
          ...(call.child_session_id ? { child_session_id: call.child_session_id } : {}),
        });
      }
    ),
    compozyApiMock.post(
      "/api/workspaces/{workspace_id}/calls/{call_id}/cancel",
      ({ params, request, response }) => {
        const callId = String(params.call_id);
        const state = store.cancelCall(String(params.workspace_id), new URL(request.url), callId);
        if (state === undefined) return response(404).json(notFound(callId));
        return response(200).json({ state });
      }
    ),
    compozyApiMock.get("/api/workspaces/{workspace_id}/messages", ({ params, request }) =>
      HttpResponse.json(store.pageMessages(String(params.workspace_id), new URL(request.url)))
    ),
    compozyApiMock.post(
      "/api/workspaces/{workspace_id}/messages",
      async ({ params, request, response }) => {
        created += 1;
        const messageId = `msg_mock_${created}`;
        const body = (await request.json()) as SendCallMessageRequest;
        const url = new URL(request.url);
        store.addMessage(
          deliveredMessage(
            store,
            messageId,
            body,
            String(params.workspace_id),
            url.searchParams.get("profile") ?? "default"
          )
        );
        return response(202).json({ message_id: messageId, delivery: "queued" });
      }
    ),
  ];
}

/** One isolated mock server: its own data, its own writes. */
export function createAgentCommsHandlers(dataset: AgentCommsDataset): HttpHandler[] {
  return buildAgentCommsHandlers(createAgentCommsMockStore(dataset));
}
