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
  body: CreateCallRequest
): CallPayload {
  const template = store.snapshotCalls()[0];
  const prompt = body.prompt;
  const caller = template?.caller.id ?? "ses_operator";
  const childSessionId = body.target.session_id ?? `ses_${callId}`;
  return {
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
    result_preview: undefined,
    result_bytes: 0,
    repair_attempts: 0,
  };
}

function deliveredMessage(
  store: AgentCommsMockStore,
  messageId: string,
  body: SendCallMessageRequest
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
    profile_name: template?.profile_name ?? "default",
    scope: template?.scope ?? "workspace",
    ...(body.call_id ? { call_id: body.call_id } : {}),
  } as CallMessagePayload;
}

export function buildAgentCommsHandlers(store: AgentCommsMockStore): HttpHandler[] {
  let created = 0;
  return [
    compozyApiMock.get("/api/workspaces/{workspace_id}/calls", ({ request }) =>
      HttpResponse.json(store.pageCalls(new URL(request.url)))
    ),
    // The typed `response(status)` helper is what keeps a mocked error honest:
    // the body has to match the shape that status actually declares, so a handler
    // cannot invent an error envelope the daemon would never send.
    compozyApiMock.get("/api/workspaces/{workspace_id}/calls/{call_id}", ({ params, response }) => {
      const callId = String(params.call_id);
      const call = store.findCall(callId);
      if (!call) return response(404).json(notFound(callId));
      return response(200).json(call);
    }),
    compozyApiMock.get(
      "/api/workspaces/{workspace_id}/calls/{call_id}/prompt",
      ({ params, response }) => {
        const callId = String(params.call_id);
        const call = store.findCall(callId);
        if (!call) return response(404).json(notFound(callId));
        return response(200).json({ call_id: callId, prompt: call.prompt_preview ?? "" });
      }
    ),
    compozyApiMock.get(
      "/api/workspaces/{workspace_id}/calls/{call_id}/result",
      ({ params, response }) => {
        const callId = String(params.call_id);
        const call = store.findCall(callId);
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
      ({ params, response }) => {
        const callId = String(params.call_id);
        const call = store.findCall(callId);
        if (!call || call.superseded_preview === undefined) {
          return response(404).json({
            error: `no superseded evidence for ${callId}`,
            code: NOT_FOUND_CODE,
          });
        }
        return response(200).json({ call_id: callId, result: call.superseded_preview });
      }
    ),
    compozyApiMock.post("/api/workspaces/{workspace_id}/calls", async ({ request, response }) => {
      created += 1;
      const callId = `call_mock_${created}`;
      const body = (await request.json()) as CreateCallRequest;
      const call = acceptedCall(store, callId, body);
      store.addCall(call);
      return response(201).json({
        call_id: callId,
        state: "queued",
        idle_expires_at: call.idle_expires_at,
        replayed: false,
        ...(call.child_session_id ? { child_session_id: call.child_session_id } : {}),
      });
    }),
    compozyApiMock.post(
      "/api/workspaces/{workspace_id}/calls/{call_id}/cancel",
      ({ params, response }) => {
        const callId = String(params.call_id);
        const state = store.cancelCall(callId);
        if (state === undefined) return response(404).json(notFound(callId));
        return response(200).json({ state });
      }
    ),
    compozyApiMock.get("/api/workspaces/{workspace_id}/messages", ({ request }) =>
      HttpResponse.json(store.pageMessages(new URL(request.url)))
    ),
    compozyApiMock.post(
      "/api/workspaces/{workspace_id}/messages",
      async ({ request, response }) => {
        created += 1;
        const messageId = `msg_mock_${created}`;
        const body = (await request.json()) as SendCallMessageRequest;
        store.addMessage(deliveredMessage(store, messageId, body));
        return response(202).json({ message_id: messageId, delivery: "queued" });
      }
    ),
  ];
}

/** One isolated mock server: its own data, its own writes. */
export function createAgentCommsHandlers(dataset: AgentCommsDataset): HttpHandler[] {
  return buildAgentCommsHandlers(createAgentCommsMockStore(dataset));
}
