// Suite: MCP authorization transition logic
// Invariant: stale completions cannot replace the active attempt, manual exchange requires a full
// redirect URL and is phase-gated, and the accepted request executes its own callback.
// Boundary IN: createMCPAuthorizeLogic transitions and enqueued executor callbacks.
// Boundary OUT: React hook subscriptions and Settings MCP HTTP adapters.
import { describe, expect, it, vi } from "vitest";

import { createMCPAuthorizeLogic } from "../mcp-authorize-store";

const filter = { scope: "workspace" as const, workspace_id: "ws-alpha" };
const prior = { status: "needs_login", tokenPresent: false };
const beginResponse = {
  authorization_url: "https://auth.linear.app/oauth/authorize?state=x",
  callback_url: "http://127.0.0.1:2123/api/mcp/oauth/callback",
  expires_at: "2026-07-15T00:05:00Z",
  manual_supported: true,
  state: "compozy_mcp_x",
};
const pendingBegin = vi.fn(() => new Promise<never>(() => undefined));
const pendingExchange = vi.fn(() => new Promise<never>(() => undefined));
const ignoreConfirmation = vi.fn();

describe("createMCPAuthorizeLogic", () => {
  it("rejects stale completions", () => {
    const store = createMCPAuthorizeLogic().createStore();
    let snapshot = store.getInitialSnapshot();

    [snapshot] = store.transition(snapshot, {
      type: "authorizeRequested",
      begin: pendingBegin,
      dismissOnConfirmation: false,
      server: "linear",
      filter,
      prior,
      mode: "automatic",
    });
    const activeAttemptId = snapshot.context.attemptId;
    const [staleSnapshot] = store.transition(snapshot, {
      type: "beginSucceeded",
      attemptId: activeAttemptId - 1,
      begin: beginResponse,
    });

    expect(staleSnapshot).toBe(snapshot);

    [snapshot] = store.transition(snapshot, {
      type: "beginSucceeded",
      attemptId: activeAttemptId,
      begin: beginResponse,
    });
    expect(snapshot.context.phase).toBe("waiting");
  });

  it("allows a full manual redirect only from the manual phase", () => {
    const store = createMCPAuthorizeLogic().createStore();

    expect(
      store.can.manualRedirectSubmitted({
        exchange: pendingExchange,
        onConfirmed: ignoreConfirmation,
        value: "http://127.0.0.1:2123/api/mcp/oauth/callback?code=example&state=x",
      })
    ).toBe(false);
    store.trigger.authorizeRequested({
      begin: pendingBegin,
      dismissOnConfirmation: false,
      server: "linear",
      filter,
      prior,
      mode: "automatic",
    });
    const automaticAttemptId = store.getSnapshot().context.attemptId;
    store.trigger.beginSucceeded({ attemptId: automaticAttemptId, begin: beginResponse });
    expect(
      store.can.manualRedirectSubmitted({
        exchange: pendingExchange,
        onConfirmed: ignoreConfirmation,
        value: "http://127.0.0.1:2123/api/mcp/oauth/callback?code=example&state=x",
      })
    ).toBe(false);

    store.trigger.manualAuthorizationRequested({ begin: pendingBegin });
    const manualAttemptId = store.getSnapshot().context.attemptId;
    store.trigger.beginSucceeded({ attemptId: manualAttemptId, begin: beginResponse });

    expect(
      store.can.manualRedirectSubmitted({
        exchange: pendingExchange,
        onConfirmed: ignoreConfirmation,
        value: "",
      })
    ).toBe(false);
    expect(
      store.can.manualRedirectSubmitted({
        exchange: pendingExchange,
        onConfirmed: ignoreConfirmation,
        value: "code-only",
      })
    ).toBe(false);
    expect(
      store.can.manualRedirectSubmitted({
        exchange: pendingExchange,
        onConfirmed: ignoreConfirmation,
        value: "http://127.0.0.1:2123/api/mcp/oauth/callback?code=example&state=x",
      })
    ).toBe(true);
  });

  it("executes the begin callback carried by the accepted intent", () => {
    const currentBegin = vi.fn(() => new Promise<never>(() => undefined));
    const store = createMCPAuthorizeLogic().createStore();

    store.trigger.authorizeRequested({
      begin: currentBegin,
      dismissOnConfirmation: false,
      server: "linear",
      filter,
      prior,
      mode: "automatic",
    });

    expect(currentBegin).toHaveBeenCalledWith({ server: "linear", filter, mode: "automatic" });
  });

  it("reviews non-empty scope escalation before beginning with explicit approval", () => {
    pendingBegin.mockClear();
    const store = createMCPAuthorizeLogic().createStore();

    store.trigger.scopeEscalationRequested({
      dismissOnConfirmation: false,
      server: "linear",
      filter,
      prior,
      approvedScopes: [],
    });

    expect(store.getSnapshot().context.phase).toBe("idle");
    expect(pendingBegin).not.toHaveBeenCalled();

    store.trigger.scopeEscalationRequested({
      dismissOnConfirmation: false,
      server: "linear",
      filter,
      prior,
      approvedScopes: ["write"],
    });

    expect(store.getSnapshot().context.phase).toBe("reviewing_scopes");
    expect(pendingBegin).not.toHaveBeenCalled();

    store.trigger.scopeEscalationConfirmed({ begin: pendingBegin });

    expect(store.getSnapshot().context.phase).toBe("beginning");
    expect(pendingBegin).toHaveBeenCalledWith({
      server: "linear",
      filter,
      mode: "automatic",
      scopeApproval: { approveScopeEscalation: true, approvedScopes: ["write"] },
    });
  });
});
