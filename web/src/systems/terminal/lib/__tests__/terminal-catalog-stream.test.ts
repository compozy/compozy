import { describe, expect, it } from "vitest";

import {
  parseTerminalCatalogEvent,
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

  it("Should ignore an unregistered event name", () => {
    expect(parseTerminalCatalogEvent("terminal.future", { anything: true })).toBeNull();
  });

  it.each([
    ["available with a controller", "available", "human", "viewer-1"],
    ["human ownership without an actor", "human_owned", "", ""],
    ["human ownership with an agent", "human_owned", "agent", "agent-1"],
    ["agent ownership without an actor id", "agent_owned", "agent", ""],
    ["agent ownership with a human", "agent_owned", "human", "viewer-1"],
  ])("Should reject %s", (_case, lease, controllerKind, controllerId) => {
    expect(() =>
      parseTerminalCatalogEvent("terminal.lease_changed", {
        terminal_id: "term-1",
        lease,
        controller_kind: controllerKind,
        controller_id: controllerId,
        reason: "ownership changed",
      })
    ).toThrow(TerminalCatalogProtocolError);
  });

  it.each([
    ["available", "", "", null],
    ["human_owned", "human", "viewer-1", { kind: "human", id: "viewer-1" }],
    ["agent_owned", "agent", "agent-1", { kind: "agent", id: "agent-1" }],
  ])("Should accept the %s controller tuple", (lease, controllerKind, controllerId, controller) => {
    expect(
      parseTerminalCatalogEvent("terminal.lease_changed", {
        terminal_id: "term-1",
        lease,
        controller_kind: controllerKind,
        controller_id: controllerId,
        reason: "ownership changed",
      })
    ).toEqual({
      name: "terminal.lease_changed",
      terminalId: "term-1",
      lease,
      controller,
      reason: "ownership changed",
    });
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
