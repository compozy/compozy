import { describe, expect, it } from "vitest";

import { terminalAttachModeFor, terminalLeaseView } from "../../lib/terminal-lease";
import { terminalScopeKey } from "../../lib/terminal-scope-key";
import { terminalStoreLogic, type TerminalStoreState } from "../terminal-store";

/**
 * Canonical suite for terminal client state (UT-078, UT-113).
 *
 * Invariant: the daemon's `OWNER` frames alone decide the lease read for every
 * `lease_state`, and a scope switch drops the previous scope's panes.
 */

const INTERACTIVE = { interactive: true };
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

describe("terminal lease state", () => {
  it("Should read control from the daemon's frames for every lease state", () => {
    const store = openStore(WORK_KEY, DEV_SERVER);
    store.trigger.attached({
      terminalId: DEV_SERVER,
      lease: "agent_owned",
      mode: "pty",
      cols: 96,
      rows: 28,
    });

    const agentPane = paneOf(store.getSnapshot().context, DEV_SERVER);
    const agentView = terminalLeaseView({
      lease: agentPane.lease,
      controller: { kind: "agent", id: "Claude Code" },
      viewerId: "pedro",
      mode: agentPane.mode,
      capabilities: INTERACTIVE,
    });
    expect(agentView.read).toBe("agent");
    expect(agentView.label).toBe("Claude Code is in control");
    expect(agentView.canTakeControl).toBe(true);
    // Taking over an agent is immediate and never asks (ADR-002).
    expect(agentView.requiresConfirmation).toBe(false);
    expect(agentView.canType).toBe(false);

    // A pipe terminal has no lease to contest, so the chip attributes the run
    // instead of claiming control, and offers no way to take it.
    const pipeView = terminalLeaseView({
      lease: agentPane.lease,
      controller: { kind: "agent", id: "Claude Code" },
      viewerId: "pedro",
      mode: "pipe",
      capabilities: INTERACTIVE,
    });
    expect(pipeView.label).toBe("Claude Code ran this");
    expect(pipeView.canTakeControl).toBe(false);

    store.trigger.leaseObserved({
      terminalId: DEV_SERVER,
      lease: "human_owned",
      controller: { kind: "human", id: "marina" },
      reason: "takeover",
    });
    const otherPane = paneOf(store.getSnapshot().context, DEV_SERVER);
    const otherView = terminalLeaseView({
      lease: otherPane.lease,
      controller: otherPane.controller,
      viewerId: "pedro",
      mode: otherPane.mode,
      capabilities: INTERACTIVE,
    });
    expect(otherView.read).toBe("someone-else");
    expect(otherView.label).toBe("marina is in control");
    expect(otherView.requiresConfirmation).toBe(true);
    expect(otherView.canType).toBe(false);

    const anonymousOperatorView = terminalLeaseView({
      lease: "human_owned",
      controller: { kind: "human", id: "operator" },
      viewerId: "client:web",
      mode: "pty",
      capabilities: INTERACTIVE,
    });
    expect(anonymousOperatorView.requiresConfirmation).toBe(false);

    store.trigger.leaseObserved({
      terminalId: DEV_SERVER,
      lease: "human_owned",
      controller: { kind: "human", id: "pedro" },
      reason: "takeover",
    });
    const minePane = paneOf(store.getSnapshot().context, DEV_SERVER);
    const mineView = terminalLeaseView({
      lease: minePane.lease,
      controller: minePane.controller,
      viewerId: "pedro",
      mode: minePane.mode,
      capabilities: INTERACTIVE,
    });
    expect(mineView.read).toBe("you");
    expect(mineView.label).toBe("You're in control");
    expect(mineView.canType).toBe(true);
    expect(mineView.canTakeControl).toBe(false);
    expect(mineView.canRelease).toBe(true);
    expect(terminalAttachModeFor(mineView)).toBe("write");

    store.trigger.leaseObserved({
      terminalId: DEV_SERVER,
      lease: "available",
      controller: null,
      reason: "grace_expired",
    });
    const freePane = paneOf(store.getSnapshot().context, DEV_SERVER);
    const freeView = terminalLeaseView({
      lease: freePane.lease,
      controller: freePane.controller,
      viewerId: "pedro",
      mode: freePane.mode,
      capabilities: INTERACTIVE,
    });
    expect(freeView.read).toBe("nobody");
    expect(freeView.label).toBe("No one in control");
    expect(freeView.canType).toBe(false);
    // A free lease is not an invitation to type: watching stays the default.
    expect(terminalAttachModeFor(freeView)).toBe("read");
  });

  it("Should never claim control the daemon did not grant", () => {
    const store = openStore(WORK_KEY, DEV_SERVER);
    store.trigger.inputEnabledChanged({ terminalId: DEV_SERVER, enabled: true });

    const pane = paneOf(store.getSnapshot().context, DEV_SERVER);
    // Local input state is a courtesy gate, never evidence about the lease.
    expect(pane.lease).toBe("available");
    expect(pane.controller).toBeNull();
  });

  it("Should clear the controller named by the previous attachment pass", () => {
    const store = openStore(WORK_KEY, DEV_SERVER);
    store.trigger.leaseObserved({
      terminalId: DEV_SERVER,
      lease: "human_owned",
      controller: { kind: "human", id: "marina" },
      reason: "takeover",
    });

    store.trigger.attached({
      terminalId: DEV_SERVER,
      lease: "available",
      mode: "pty",
      cols: 96,
      rows: 28,
    });

    const pane = paneOf(store.getSnapshot().context, DEV_SERVER);
    expect(pane.controller).toBeNull();
    expect(pane.ownerObserved).toBe(true);
    expect(pane.lease).toBe("available");
  });

  it("Should project daemon presence without deriving it from local panes", () => {
    const store = openStore(WORK_KEY, DEV_SERVER);
    store.trigger.presenceObserved({ terminalId: DEV_SERVER, viewers: 2 });
    expect(paneOf(store.getSnapshot().context, DEV_SERVER).viewers).toBe(2);
  });

  it("Should offer no control affordances where control cannot exist", () => {
    const pipeView = terminalLeaseView({
      lease: "agent_owned",
      controller: { kind: "agent", id: "Claude Code" },
      viewerId: "pedro",
      mode: "pipe",
      capabilities: INTERACTIVE,
    });
    expect(pipeView.canTakeControl).toBe(false);
    expect(pipeView.canType).toBe(false);

    const executeOnlyView = terminalLeaseView({
      lease: "available",
      controller: null,
      viewerId: "pedro",
      mode: "pty",
      capabilities: { interactive: false },
    });
    expect(executeOnlyView.canTakeControl).toBe(false);
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
    store.trigger.attached({
      terminalId: DEV_SERVER,
      lease: "human_owned",
      mode: "pty",
      cols: 96,
      rows: 28,
    });
    expect(Object.keys(store.getSnapshot().context.panes)).toEqual([DEV_SERVER]);

    store.trigger.scopeBound({ scopeKey: PERSONAL_KEY });

    // Hidden, not stale: the pane leaves the store rather than lingering with
    // the previous profile's lease on screen.
    expect(store.getSnapshot().context.panes).toEqual({});
    expect(store.getSnapshot().context.scopeKey).toBe(PERSONAL_KEY);
  });

  it("Should keep a re-entered scope from resurrecting its old panes", () => {
    const store = openStore(WORK_KEY, DEV_SERVER);
    store.trigger.scopeBound({ scopeKey: PERSONAL_KEY });
    store.trigger.scopeBound({ scopeKey: WORK_KEY });

    expect(store.getSnapshot().context.panes).toEqual({});
  });

  it("Should ignore frames for a terminal the current scope does not hold", () => {
    const store = openStore(WORK_KEY, DEV_SERVER);
    store.trigger.leaseObserved({
      terminalId: "term-71c0aa9358de",
      lease: "human_owned",
      controller: { kind: "human", id: "marina" },
      reason: "takeover",
    });

    expect(Object.keys(store.getSnapshot().context.panes)).toEqual([DEV_SERVER]);
  });
});
