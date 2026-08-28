// Suite: OS terminal-window controller host gates
// Invariant: a visible journal keeps the host after the last PTY closes; the
// journal query stays disabled until first reveal.
// Boundary IN: close-success decision and journal enable gate used by the host.
// Boundary OUT: recording stop reconcile and elapsed timer, owned by
// terminal-recording-state / use-terminal-recordings.

import { describe, expect, it } from "vitest";

import { decideTerminalCloseHost } from "../lib/terminal-window-close";
import { terminalJournalQueryEnabled } from "../lib/terminal-window-journal";

const LAST = { id: "term-last" };
const NEXT = { id: "term-next" };

describe("useTerminalWindowControllerState close path", () => {
  it("Should keep the terminal app when the last PTY closes on a visible journal", () => {
    expect(
      decideTerminalCloseHost({
        closedId: LAST.id,
        routedId: LAST.id,
        remaining: [],
        journalVisible: true,
      })
    ).toEqual({ kind: "keep" });
  });

  it("Should close the window when the last PTY closes and the journal is not showing", () => {
    expect(
      decideTerminalCloseHost({
        closedId: LAST.id,
        routedId: LAST.id,
        remaining: [],
        journalVisible: false,
      })
    ).toEqual({ kind: "close" });
  });

  it("Should retarget to a remaining terminal instead of tearing down the host", () => {
    expect(
      decideTerminalCloseHost({
        closedId: LAST.id,
        routedId: LAST.id,
        remaining: [NEXT],
        journalVisible: false,
      })
    ).toEqual({ kind: "retarget", terminalId: NEXT.id });
  });

  it("Should leave the route alone when the closed terminal is not the one in the URL", () => {
    expect(
      decideTerminalCloseHost({
        closedId: LAST.id,
        routedId: NEXT.id,
        remaining: [NEXT],
        journalVisible: false,
      })
    ).toEqual({ kind: "noop" });
  });
});

describe("useTerminalWindowControllerState journal and recording host gates", () => {
  it("Should keep the journal query disabled until the journal is first opened", () => {
    expect(terminalJournalQueryEnabled("", false)).toBe(false);
    expect(terminalJournalQueryEnabled("ws-atlas", false)).toBe(false);
    expect(terminalJournalQueryEnabled("ws-atlas", true)).toBe(true);
  });
});
