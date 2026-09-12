import { HttpResponse, type HttpHandler } from "msw";
import { compozyApiMock } from "@/storybook/openapi-msw";
import type { SessionPayload } from "../types";
import { sessionContextTurnsFixture, sessionContextUsageFixture } from "./context-fixtures";

export function sessionContextHandlers(
  sessions: ReadonlyMap<string, SessionPayload>
): HttpHandler[] {
  return [
    compozyApiMock.get(
      "/api/workspaces/{workspace_id}/sessions/{session_id}/usage",
      ({ params }) => {
        const session = sessions.get(String(params.session_id));
        if (!session || session.workspace_id !== params.workspace_id)
          return HttpResponse.json({ error: "Session not found" }, { status: 404 });
        return HttpResponse.json({ usage: sessionContextUsageFixture });
      }
    ),
    compozyApiMock.get(
      "/api/workspaces/{workspace_id}/sessions/{session_id}/usage/turns",
      ({ params }) => {
        const session = sessions.get(String(params.session_id));
        if (!session || session.workspace_id !== params.workspace_id)
          return HttpResponse.json({ error: "Session not found" }, { status: 404 });
        return HttpResponse.json(sessionContextTurnsFixture);
      }
    ),
  ];
}
