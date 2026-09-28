import { delay, HttpResponse, type HttpHandler } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";

import type {
  ContinueSessionRequest,
  SessionDerivePreview,
  SessionDeriveResult,
} from "../adapters/session-derive-api";
import { derivePreviewFixture, deriveResultFixture } from "./derive-fixtures";

export type SessionDerivePreviewMock = SessionDerivePreview | "error" | "pending";

export interface SessionDeriveHandlerOptions {
  preview?: SessionDerivePreviewMock;
  /** The continue outcome; `"pending"` never answers (the pending state). */
  result?: SessionDeriveResult | "pending" | { status: number; error: string; code?: string };
  /** Replayed outcomes answer `200` instead of `201`. */
  onContinue?: (request: ContinueSessionRequest) => void;
}

/** MSW handlers for the derive preview and the continue route (stories and tests). */
export function sessionDeriveHandlers({
  preview = derivePreviewFixture,
  result = deriveResultFixture(),
  onContinue,
}: SessionDeriveHandlerOptions = {}): HttpHandler[] {
  return [
    compozyApiMock.get(
      "/api/workspaces/{workspace_id}/sessions/{session_id}/derive/preview",
      async () => {
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
        if (result === "pending") {
          await delay("infinite");
          return HttpResponse.json({ error: "never" }, { status: 500 });
        }
        if ("error" in result) {
          return HttpResponse.json(
            { error: result.error, ...(result.code ? { code: result.code } : {}) },
            { status: result.status }
          );
        }
        return HttpResponse.json(result, { status: result.derived.replayed ? 200 : 201 });
      }
    ),
  ];
}
