import { HttpResponse, http, type HttpHandler } from "msw";

import { JOURNAL_FIXTURES, PASSWORD_REQUEST, TERMINAL_FIXTURES } from "./terminal-fixtures";

/**
 * Storybook's stand-in for the terminal routes.
 *
 * The routes are registered in the public-activation tranche, so stories would
 * otherwise trip the MSW guard on every attach. These answer the reads the
 * surfaces make and mint the attach pass a pane needs before it can connect —
 * the byte stream itself is scripted by the story, not by MSW.
 */
const TERMINALS_PATH = "/api/workspaces/:workspaceId/terminals";

export const handlers: HttpHandler[] = [
  http.get(TERMINALS_PATH, () => HttpResponse.json({ terminals: TERMINAL_FIXTURES })),
  http.get(`${TERMINALS_PATH}/journal`, () =>
    HttpResponse.json({ entries: JOURNAL_FIXTURES, next: null })
  ),
  http.get(`${TERMINALS_PATH}/input-requests`, () =>
    HttpResponse.json({ requests: [PASSWORD_REQUEST] })
  ),
  http.post(`${TERMINALS_PATH}/:terminalId/attach-ticket`, () =>
    HttpResponse.json(
      { ticket: "tkt-storybook", expires_at: "2026-08-25T12:00:30Z" },
      { status: 201 }
    )
  ),
  http.get(`${TERMINALS_PATH}/:terminalId/read`, () =>
    HttpResponse.json({
      content: "current screen",
      seq: 4096,
      truncated: false,
      busy: false,
      untrusted: true,
    })
  ),
];
