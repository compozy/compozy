import { describe, expect, it } from "vitest";

import { DEV_SERVER_TERMINAL, PSQL_TERMINAL } from "../../mocks/terminal-fixtures";
import {
  parseTerminalCatalogEvent,
  reconcileTerminalCatalog,
  TerminalCatalogProtocolError,
} from "../terminal-catalog-stream";

describe("parseTerminalCatalogEvent", () => {
  it.each([
    ["a malformed projection", "terminal.created", { terminal: { id: "incomplete" } }],
    ["an unknown enum", "terminal.mode_changed", { terminal_id: "term-1", mode: "future" }],
    [
      "an unknown extra field",
      "terminal.mode_changed",
      { terminal_id: "term-1", mode: "pty", secret: "hidden" },
    ],
  ])("Should make %s explicit for a registered event", (_case, eventName, payload) => {
    expect(() => parseTerminalCatalogEvent(eventName, payload)).toThrow(
      TerminalCatalogProtocolError
    );
  });

  it("Should omit payload details from protocol errors", () => {
    const secret = "do-not-report-this-payload";

    const error = captureProtocolError(() =>
      parseTerminalCatalogEvent("terminal.created", { terminal: { secret } })
    );

    expect(error.message).not.toContain(secret);
  });

  // The daemon names the owner on close and delete frames (aggregate stream, path streams too).
  it("Should accept the owner fields on close and drop a deleted terminal", () => {
    const closed = parseTerminalCatalogEvent("terminal.closed", {
      terminal_id: DEV_SERVER_TERMINAL.id,
      exit: null,
      workspace_id: "ws-atlas",
      profile_id: "prof_1",
    });
    expect(closed).toEqual({
      name: "terminal.closed",
      terminalId: DEV_SERVER_TERMINAL.id,
      exit: null,
    });

    const deleted = parseTerminalCatalogEvent("terminal.deleted", {
      terminal_id: DEV_SERVER_TERMINAL.id,
      workspace_id: "",
      profile_id: "prof_1",
    });
    expect(
      reconcileTerminalCatalog([DEV_SERVER_TERMINAL, PSQL_TERMINAL], deleted!).map(t => t.id)
    ).toEqual([PSQL_TERMINAL.id]);
  });

  it("Should replace the list with an aggregate reset snapshot", () => {
    const snapshot = parseTerminalCatalogEvent("terminal.snapshot", {
      terminals: [PSQL_TERMINAL],
      reset: true,
    });
    expect(reconcileTerminalCatalog([DEV_SERVER_TERMINAL], snapshot!)).toEqual([PSQL_TERMINAL]);
  });

  it("Should ignore an unregistered event name", () => {
    expect(parseTerminalCatalogEvent("terminal.future", { anything: true })).toBeNull();
  });

  it.each([
    "terminal.input_requested",
    "terminal.input_provided",
    "terminal.recording_started",
    "terminal.recording_stopped",
  ] as const)("Should ignore hook event %s — it is not a catalog frame", eventName => {
    expect(
      parseTerminalCatalogEvent(eventName, {
        terminal_id: "term-1",
        request_id: "req-3f8a",
        redacted: true,
      })
    ).toBeNull();
  });
});

function captureProtocolError(run: () => void): TerminalCatalogProtocolError {
  try {
    run();
  } catch (error) {
    if (error instanceof TerminalCatalogProtocolError) return error;
    throw error;
  }
  throw new Error("expected a terminal catalog protocol error");
}
