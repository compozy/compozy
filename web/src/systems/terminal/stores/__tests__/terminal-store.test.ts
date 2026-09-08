import { describe, expect, it } from "vitest";

import { terminalScopeKey } from "../../lib/terminal-scope-key";
import { terminalStoreLogic, type TerminalStoreState } from "../terminal-store";

/**
 * Canonical suite for terminal client state (UT-078, UT-113).
 *
 * Invariant: connection state follows the stream, and a scope switch drops the
 * previous profile's panes instead of exposing stale input state.
 */

const WORK_KEY = terminalScopeKey("ws-atlas", "work");
const PERSONAL_KEY = terminalScopeKey("ws-atlas", "personal");
const DEV_SERVER = "term-4f21c9a03b7e";

function openStore(scopeKey: string, terminalId: string) {
  const store = terminalStoreLogic.createStore();
  store.trigger.scopeBound({ scopeKey });
  store.trigger.paneOpened({ terminalId });
  return store;
}

function paneOf(state: TerminalStoreState, terminalId: string) {
  const pane = state.panes[terminalId];
  if (!pane) throw new Error(`pane ${terminalId} is not in the store`);
  return pane;
}

describe("terminal pane state", () => {
  it("Should record attachment dimensions and shared input readiness", () => {
    const store = openStore(WORK_KEY, DEV_SERVER);
    store.trigger.attached({ terminalId: DEV_SERVER, mode: "pty", cols: 96, rows: 28 });
    store.trigger.inputEnabledChanged({ terminalId: DEV_SERVER, enabled: true });

    expect(paneOf(store.getSnapshot().context, DEV_SERVER)).toMatchObject({
      status: "connected",
      mode: "pty",
      cols: 96,
      rows: 28,
      inputEnabled: true,
    });
  });

  it("Should project daemon presence without deriving it from local panes", () => {
    const store = openStore(WORK_KEY, DEV_SERVER);
    store.trigger.presenceObserved({ terminalId: DEV_SERVER, viewers: 2 });
    expect(paneOf(store.getSnapshot().context, DEV_SERVER).viewers).toBe(2);
  });

  it("Should close input and hold the exit when the program ends", () => {
    const store = openStore(WORK_KEY, DEV_SERVER);
    store.trigger.inputEnabledChanged({ terminalId: DEV_SERVER, enabled: true });
    store.trigger.exited({
      terminalId: DEV_SERVER,
      exit: { cause: "exited", code: 1, signal: null },
    });

    const pane = paneOf(store.getSnapshot().context, DEV_SERVER);
    expect(pane.inputEnabled).toBe(false);
    expect(pane.exit).toEqual({ cause: "exited", code: 1, signal: null });
  });
});

describe("terminal profile rebinding", () => {
  it("Should drop the previous profile's panes", () => {
    const store = openStore(WORK_KEY, DEV_SERVER);
    store.trigger.attached({ terminalId: DEV_SERVER, mode: "pty", cols: 96, rows: 28 });

    store.trigger.scopeBound({ scopeKey: PERSONAL_KEY });

    expect(store.getSnapshot().context.panes).toEqual({});
    expect(store.getSnapshot().context.scopeKey).toBe(PERSONAL_KEY);
  });

  it("Should ignore frames for a terminal the current scope does not hold", () => {
    const store = openStore(WORK_KEY, DEV_SERVER);
    store.trigger.presenceObserved({ terminalId: "term-71c0aa9358de", viewers: 4 });

    expect(Object.keys(store.getSnapshot().context.panes)).toEqual([DEV_SERVER]);
  });
});
