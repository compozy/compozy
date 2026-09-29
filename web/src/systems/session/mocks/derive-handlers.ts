import { delay, HttpResponse, type HttpHandler } from "msw";

import type { OperationResponse } from "@/lib/api-contract";
import { compozyApiMock } from "@/storybook/openapi-msw";

import type {
  ContinueSessionRequest,
  ForkSessionRequest,
  SessionDerivePreview,
  SessionDeriveResult,
} from "../adapters/session-derive-api";
import type { SessionPayload } from "../types";
import { derivePreviewFixture, deriveResultFixture } from "./derive-fixtures";

export type SessionDerivePreviewMock = SessionDerivePreview | "error" | "pending";

export type SessionDeriveMockResult = SessionDeriveResult | "pending" | SessionDeriveMockError;

/** A derive refusal body; `diagnostic` is the structured item a runtime refusal carries. */
export interface SessionDeriveMockError {
  status: number;
  error: string;
  code?: string;
  child_session_id?: string;
  diagnostic?: OperationResponse<"continueSession", 422>["diagnostic"];
}

export interface SessionDeriveHandlerOptions {
  preview?: SessionDerivePreviewMock;
  /**
   * The continue or fork outcome; `"pending"` never answers (the pending
   * state). Replayed outcomes answer `200` instead of `201`. A list answers
   * successive submits in order and repeats its last entry.
   */
  result?: SessionDeriveMockResult | readonly SessionDeriveMockResult[];
  /** Served on the session detail route: the child a post-commit refusal names. */
  committedChild?: SessionPayload;
  onContinue?: (request: ContinueSessionRequest) => void;
  onFork?: (request: ForkSessionRequest) => void;
  /** Sees each preview request's `message_id` (absent for the whole session). */
  onPreview?: (messageId: string | null) => void;
}

/** MSW handlers for the derive preview and the continue and fork routes (stories and tests). */
export function sessionDeriveHandlers({
  preview = derivePreviewFixture,
  result = deriveResultFixture(),
  onContinue,
  onFork,
  onPreview,
  committedChild,
}: SessionDeriveHandlerOptions = {}): HttpHandler[] {
  const results: readonly SessionDeriveMockResult[] = Array.isArray(result) ? result : [result];
  let submits = 0;
  const nextResult = (): SessionDeriveMockResult =>
    results[Math.min(submits++, results.length - 1)] ?? deriveResultFixture();
  return [
    ...(committedChild
      ? [
          compozyApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}", () =>
            HttpResponse.json({ session: committedChild })
          ),
        ]
      : []),
    compozyApiMock.get(
      "/api/workspaces/{workspace_id}/sessions/{session_id}/derive/preview",
      async ({ request }) => {
        onPreview?.(new URL(request.url).searchParams.get("message_id"));
        if (preview === "pending") {
          await delay("infinite");
        }
        if (preview === "error" || preview === "pending") {
          return HttpResponse.json({ error: "derive preview failed" }, { status: 500 });
        }
        return HttpResponse.json(preview);
      }
    ),
    compozyApiMock.post(
      "/api/workspaces/{workspace_id}/sessions/{session_id}/continue",
      async ({ request }) => {
        onContinue?.((await request.json()) as ContinueSessionRequest);
        const result = nextResult();
        if (result === "pending") {
          await delay("infinite");
          return HttpResponse.json({ error: "never" }, { status: 500 });
        }
        if ("error" in result) {
          return HttpResponse.json(deriveErrorBody(result), { status: result.status });
        }
        return HttpResponse.json(result, { status: result.derived.replayed ? 200 : 201 });
      }
    ),
    compozyApiMock.post(
      "/api/workspaces/{workspace_id}/sessions/{session_id}/fork",
      async ({ request }) => {
        onFork?.((await request.json()) as ForkSessionRequest);
        const result = nextResult();
        if (result === "pending") {
          await delay("infinite");
          return HttpResponse.json({ error: "never" }, { status: 500 });
        }
        if ("error" in result) {
          return HttpResponse.json(deriveErrorBody(result), { status: result.status });
        }
        return HttpResponse.json(result, { status: result.derived.replayed ? 200 : 201 });
      }
    ),
  ];
}

function deriveErrorBody(result: SessionDeriveMockError) {
  return {
    error: result.error,
    ...(result.code ? { code: result.code } : {}),
    ...(result.diagnostic ? { diagnostic: result.diagnostic } : {}),
    ...(result.child_session_id ? { child_session_id: result.child_session_id } : {}),
  };
}
