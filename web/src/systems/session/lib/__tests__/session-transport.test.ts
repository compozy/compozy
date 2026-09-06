import { describe, expect, it } from "vitest";

import {
  isSessionTransportDisconnected,
  SESSION_TRANSPORT_GRACE_MS,
  SESSION_TRANSPORT_LIVE,
  sessionTransportChip,
  transportGraceElapsed,
  type SessionTransportSnapshot,
} from "../session-transport";

// Suite: live-view transport read model (UT-102 phase → chip mapping, UT-103 grace helper,
// disconnected send guard derivation).
// Invariant: the chip says exactly what the store phase means — nothing for a healthy
// stream or a blip under the grace, the attempt count after a drop, danger only once
// retries ran out, paused for a background window — and "disconnected" for a send is
// only a drop, never the first connect of a fresh window.
// Boundary IN: live-tail store transport facts. Boundary OUT: chip, composer guard, sidebar.
describe("session transport read model", () => {
  const live = (overrides: Partial<SessionTransportSnapshot> = {}): SessionTransportSnapshot => ({
    ...SESSION_TRANSPORT_LIVE,
    lastLiveAt: 1_000,
    ...overrides,
  });
  const shown = { graceElapsed: true, windowLive: true };

  it("Should show nothing for a healthy stream and only after the grace while degraded (UT-102/103)", () => {
    expect(sessionTransportChip(live(), shown)).toBeNull();
    const dropped = live({ degradedAt: 5_000, phase: "waiting-reconnect", reconnectAttempt: 1 });
    expect(sessionTransportChip(dropped, { graceElapsed: false, windowLive: true })).toBeNull();
    expect(sessionTransportChip(dropped, shown)).toEqual({ kind: "reconnecting", attempt: 1 });
    expect(transportGraceElapsed(5_000, 5_000 + SESSION_TRANSPORT_GRACE_MS - 1)).toBe(false);
    expect(transportGraceElapsed(5_000, 5_000 + SESSION_TRANSPORT_GRACE_MS)).toBe(true);
    expect(transportGraceElapsed(null, 10_000)).toBe(false);
  });

  it("Should read Connecting only for a fresh window and Reconnecting with the count after a drop", () => {
    const fresh = live({ degradedAt: 0, lastLiveAt: null, phase: "connecting" });
    expect(sessionTransportChip(fresh, shown)).toEqual({ kind: "connecting" });
    const reopened = live({ degradedAt: 9_000, phase: "connecting", reconnectAttempt: 3 });
    expect(sessionTransportChip(reopened, shown)).toEqual({ kind: "reconnecting", attempt: 3 });
    // A fresh window whose first attempt failed is already a reconnect: t3code's rule.
    const freshRetry = live({ lastLiveAt: null, phase: "waiting-reconnect", reconnectAttempt: 1 });
    expect(sessionTransportChip(freshRetry, shown)).toEqual({ kind: "reconnecting", attempt: 1 });
  });

  it("Should read catching up while the replay drains and disconnected once retries ran out", () => {
    expect(sessionTransportChip(live({ catchingUp: true }), shown)).toEqual({
      kind: "catching-up",
    });
    expect(
      sessionTransportChip(live({ failure: { at: 9_000, attempts: 6 }, phase: "failed" }), shown)
    ).toEqual({ kind: "disconnected", attempts: 6 });
    expect(sessionTransportChip(live({ phase: "terminal" }), shown)).toBeNull();
    expect(sessionTransportChip(live({ phase: "disabled" }), shown)).toBeNull();
  });

  it("Should read paused for a background window whatever the stream does (US-018.EC-2)", () => {
    const paused = { graceElapsed: true, windowLive: false };
    expect(sessionTransportChip(live({ phase: "disabled" }), paused)).toEqual({
      kind: "paused",
      sinceMs: 1_000,
    });
    expect(sessionTransportChip(live({ phase: "waiting-reconnect" }), paused)).toEqual({
      kind: "paused",
      sinceMs: 1_000,
    });
  });

  it("Should call a send disconnected only after a drop, never on the first connect (US-018.AC-3)", () => {
    expect(isSessionTransportDisconnected(live())).toBe(false);
    expect(isSessionTransportDisconnected(live({ lastLiveAt: null, phase: "connecting" }))).toBe(
      false
    );
    expect(isSessionTransportDisconnected(live({ phase: "connecting" }))).toBe(true);
    expect(
      isSessionTransportDisconnected(
        live({ lastLiveAt: null, phase: "connecting", reconnectAttempt: 1 })
      )
    ).toBe(true);
    expect(isSessionTransportDisconnected(live({ phase: "waiting-reconnect" }))).toBe(true);
    expect(isSessionTransportDisconnected(live({ phase: "failed" }))).toBe(true);
    expect(isSessionTransportDisconnected(live({ phase: "terminal" }))).toBe(false);
    expect(isSessionTransportDisconnected(live({ phase: "disabled" }))).toBe(false);
  });
});
