// Suite: OS terminal-window controller close path
// Invariant: a visible journal keeps the terminal host after the last PTY closes;
// the window closes only when the journal is not visible and no terminals remain.
// Boundary IN: close-success host decision used by use-terminal-window-controller-state.
// Boundary OUT: shouldKeepTerminalJournalHost unit cases, owned by the journal helper.

import { describe, expect, it } from "vitest";

import { decideTerminalCloseHost } from "../lib/terminal-window-close";

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
