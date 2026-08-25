import { describe, expect, it } from "vitest";

import { projectTerminalBadge } from "../../lib/terminal-badge";
import { terminalAttachModeFor, terminalLeaseView } from "../../lib/terminal-lease";
import { terminalKeys } from "../../lib/query-keys";
import {
  terminalInstanceKey,
  terminalInstanceKeyInScope,
  terminalScopeKey,
} from "../../lib/terminal-scope-key";
import type { TerminalInputRequest } from "../../types";
import { terminalStoreLogic, type TerminalStoreState } from "../terminal-store";

/**
 * Canonical suite for terminal client state (UT-078, UT-113).
 *
 * Invariant: the daemon's `OWNER` frames alone decide the lease read for every
 * `lease_state`, and a profile switch drops the previous profile's panes and
 * re-keys both the catalog cache identity and the dock-badge projection.
 */

const INTERACTIVE = { interactive: true };
const WORK_SCOPE = { workspaceId: "ws-atlas", profileKey: "work" };
const PERSONAL_SCOPE = { workspaceId: "ws-atlas", profileKey: "personal" };
const WORK_KEY = terminalScopeKey(WORK_SCOPE.workspaceId, WORK_SCOPE.profileKey);
const PERSONAL_KEY = terminalScopeKey(PERSONAL_SCOPE.workspaceId, PERSONAL_SCOPE.profileKey);
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

function inputRequest(overrides: Partial<TerminalInputRequest> = {}): TerminalInputRequest {
  return {
    id: "req-3f8a",
    terminal_id: "term-9cd7e14b2a66",
    profile_id: "profile-work",
    profile_name: "work",
    reason: "sudo password",
    prompt_excerpt: "Password:",
    redacted: true,
    requested_at: "2026-08-25T12:44:00Z",
    ...overrides,
  };
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
  it("Should drop the previous profile's panes and re-key every scoped read", () => {
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
    expect(terminalKeys.catalog(WORK_SCOPE)).not.toEqual(terminalKeys.catalog(PERSONAL_SCOPE));
    expect(terminalKeys.inputRequests(WORK_SCOPE)).not.toEqual(
      terminalKeys.inputRequests(PERSONAL_SCOPE)
    );
    expect(terminalKeys.journal(WORK_SCOPE, {})).not.toEqual(
      terminalKeys.journal(PERSONAL_SCOPE, {})
    );
  });

  it("Should key one terminal's buffer to one profile", () => {
    const workBuffer = terminalInstanceKey("ws-atlas", "work", DEV_SERVER);
    const personalBuffer = terminalInstanceKey("ws-atlas", "personal", DEV_SERVER);

    expect(workBuffer).not.toEqual(personalBuffer);
    expect(terminalInstanceKeyInScope(workBuffer, WORK_KEY)).toBe(true);
    expect(terminalInstanceKeyInScope(workBuffer, PERSONAL_KEY)).toBe(false);
  });

  it("Should never let two different scopes collide into one key", () => {
    // Length-prefixed segments: a delimiter-joined key would make these two
    // scopes identical, and a pane would inherit the wrong profile's buffer.
    expect(terminalScopeKey("ws-a", "b-c")).not.toEqual(terminalScopeKey("ws-a-b", "c"));
    expect(terminalScopeKey("ws", "atlas work")).not.toEqual(terminalScopeKey("ws atlas", "work"));
  });

  it("Should re-key the dock badge and never count another profile's rows", () => {
    const workBadge = projectTerminalBadge({
      scopeKey: WORK_KEY,
      profileId: "profile-work",
      inputRequests: [inputRequest(), inputRequest({ id: "req-9c11" })],
      pendingApprovals: [{ profileId: "profile-work" }],
    });
    expect(workBadge).toEqual({ scopeKey: WORK_KEY, count: 3 });

    const personalBadge = projectTerminalBadge({
      scopeKey: PERSONAL_KEY,
      profileId: "profile-personal",
      inputRequests: [inputRequest()],
      pendingApprovals: [{ profileId: "profile-work" }],
    });
    // The switch re-keys the projection, and the previous profile's rows do not
    // survive into the new count.
    expect(personalBadge).toEqual({ scopeKey: PERSONAL_KEY, count: undefined });
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
