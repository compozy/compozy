// Suite: OS terminal-window controller host gates
// Invariant: the journal query stays disabled until first reveal, and the
// unlock survives window remounts without crossing workspaces.
// Boundary IN: journal enable gate used by the host.
// Boundary OUT: recording stop reconcile and elapsed timer, owned by
// terminal-recording-state / use-terminal-recordings. Closing a terminal keeps
// the window in place by construction — there is no host decision to test.

import { describe, expect, it } from "vitest";

import { terminalJournalQueryEnabled } from "../lib/terminal-window-journal";
import { terminalJournalUnlockLogic } from "@/systems/terminal/stores/terminal-journal-unlock-store";

describe("useTerminalWindowControllerState journal and recording host gates", () => {
  it("Should keep the journal query disabled until the journal is first opened", () => {
    expect(terminalJournalQueryEnabled("", false)).toBe(false);
    expect(terminalJournalQueryEnabled("ws-atlas", false)).toBe(false);
    expect(terminalJournalQueryEnabled("ws-atlas", true)).toBe(true);
  });

  it("Should preserve first-open state across window remounts without crossing workspaces", () => {
    const store = terminalJournalUnlockLogic.createStore();

    store.trigger.journalOpened({ workspaceId: "ws-atlas" });

    expect(store.getSnapshot().context.unlockedWorkspaces).toEqual({ "ws-atlas": true });
    expect(store.getSnapshot().context.unlockedWorkspaces["ws-other"]).toBeUndefined();
  });
});
