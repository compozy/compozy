import { waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

import { toast } from "sonner";

import { SessionApiError, type SessionSendEnvelope } from "@/systems/session";
import {
  createSessionPageControlsLogic,
  isStopRetryPending,
  STOP_COMPLETION_NOTE_MS,
  stopVerdictFromResult,
} from "../session-page-controls-store";

const envelope: SessionSendEnvelope = {
  action: "queue",
  attachments: [],
  expectedTurnId: "turn-1",
  identity: { idempotencyKey: "idk-1", messageId: "msg-1" },
  runtime: null,
  text: "Also run the migration equivalence suite",
};

function createDeferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>(res => {
    resolve = res;
  });
  return { promise, resolve };
}

// Suite: Session page-controls workflow ownership.
// Invariant: one accepted busy-input intent owns the client operation until its
// acknowledgement settles, without manufacturing daemon queue state or success notices.
// Boundary IN: accepted control intents and async executor settlements.
// Boundary OUT: store phases and emitted failures.
describe("session page controls store", () => {
  it("Should serialize busy-input intents until the accepted request settles", () => {
    const store = createSessionPageControlsLogic().createStore();
    const pending = () => new Promise<never>(() => undefined);
    let snapshot = store.getInitialSnapshot();
    [snapshot] = store.transition(snapshot, {
      type: "busyInputRequested",
      execute: pending,
      kind: "queue",
      message: "Follow up",
    });
    const requestId = snapshot.context.busyInput.requestId;
    [snapshot] = store.transition(snapshot, {
      type: "busyInputRequested",
      execute: pending,
      kind: "interrupt",
      message: "Replace it",
    });

    expect(snapshot.context.busyInput).toMatchObject({ phase: "pending", requestId });
    expect(snapshot.context.nextRequestId).toBe(requestId);
  });

  it("Should return to idle without a success toast after acknowledgement", () => {
    const store = createSessionPageControlsLogic().createStore();
    let snapshot = store.getInitialSnapshot();
    [snapshot] = store.transition(snapshot, {
      type: "busyInputRequested",
      execute: vi.fn().mockResolvedValue(undefined),
      kind: "steer",
      message: "Use the new constraint",
    });
    [snapshot] = store.transition(snapshot, {
      type: "busyInputSucceeded",
      requestId: snapshot.context.busyInput.requestId,
      result: {
        delivery: "direct",
        disposition: "steering",
        idempotency_key: "idk_1",
        message_id: "msg_1",
        queue_position: 0,
        replayed: false,
        status: "steering",
        steer_delivery: "injected",
      },
    });

    expect(snapshot.context.busyInput.phase).toBe("idle");
    expect(toast.success).not.toHaveBeenCalled();
  });

  it("Should settle the accepted request with the daemon's disposition envelope", async () => {
    const store = createSessionPageControlsLogic().createStore();
    const settled = vi.fn();
    store.on("busyInputSettled", settled);
    const result = {
      delivery: "direct",
      disposition: "queued" as const,
      entry_id: "inp_4d8",
      idempotency_key: "idk_9b02",
      message_id: "msg_01k3",
      queue_position: 2,
      replayed: false,
      status: "queued",
    };

    store.trigger.busyInputRequested({
      execute: vi.fn().mockResolvedValue(result),
      kind: "queue",
      message: "Ship it with tests",
    });

    await waitFor(() => expect(store.getSnapshot().context.busyInput.phase).toBe("idle"));
    expect(settled).toHaveBeenCalledWith(
      expect.objectContaining({ outcome: "succeeded", result, type: "busyInputSettled" })
    );
  });

  it("Should emit the real error when a direct stop executor fails", async () => {
    const store = createSessionPageControlsLogic().createStore();

    store.trigger.stopRequested({
      execute: vi.fn().mockRejectedValue(new Error("daemon disconnected")),
      failureMessage: null,
      scope: "session",
      turnId: "",
    });

    await waitFor(() => expect(store.getSnapshot().context.stop.phase).toBe("idle"));
    expect(toast.error).toHaveBeenCalledWith("daemon disconnected");
  });

  // Invariant (US-009.AC-1/AC-3/EC-1): Stop reads "stopping" from the first
  // activation, takes exactly one request per lifecycle, and only the daemon's
  // own lifecycle evidence — never the request's acknowledgement — settles it.
  // Owning layer: page-controls store. Canonical suite: this file.
  describe("stop truth", () => {
    it("Should hold an accepted turn stop as stopping until the daemon stops reporting the turn", async () => {
      const store = createSessionPageControlsLogic().createStore();
      const execute = vi.fn().mockResolvedValue(undefined);
      store.trigger.lifecycleObserved({ running: true, state: "active", turnId: "turn-1" });

      store.trigger.stopRequested({
        execute,
        failureMessage: null,
        scope: "turn",
        turnId: "turn-1",
      });
      expect(store.getSnapshot().context.stop.phase).toBe("pending");
      // A second activation while the first is landing is dropped.
      store.trigger.stopRequested({
        execute,
        failureMessage: null,
        scope: "turn",
        turnId: "turn-1",
      });
      expect(execute).toHaveBeenCalledOnce();

      await waitFor(() => expect(store.getSnapshot().context.stop.phase).toBe("stopping"));
      // Acceptance alone never reads stopped: the daemon still reports the turn.
      store.trigger.lifecycleObserved({ running: true, state: "active", turnId: "turn-1" });
      expect(store.getSnapshot().context.stop.phase).toBe("stopping");
      store.trigger.stopRequested({
        execute,
        failureMessage: null,
        scope: "turn",
        turnId: "turn-1",
      });
      expect(execute).toHaveBeenCalledOnce();

      store.trigger.lifecycleObserved({ running: false, state: "active", turnId: "" });
      expect(store.getSnapshot().context.stop.phase).toBe("idle");
    });

    it("Should settle a turn stop when the daemon reports a different turn or was already idle", () => {
      const store = createSessionPageControlsLogic().createStore();
      let snapshot = store.getInitialSnapshot();
      [snapshot] = store.transition(snapshot, {
        type: "lifecycleObserved",
        running: true,
        state: "active",
        turnId: "turn-1",
      });
      [snapshot] = store.transition(snapshot, {
        type: "stopRequested",
        execute: () => new Promise<never>(() => undefined),
        failureMessage: null,
        scope: "turn",
        turnId: "turn-1",
      });
      const requestId = snapshot.context.stop.requestId;
      [snapshot] = store.transition(snapshot, { type: "stopSucceeded", requestId });
      expect(snapshot.context.stop.phase).toBe("stopping");

      // Running with no turn identity is unknown, not a replacement: the guard holds.
      const [unknownTurn] = store.transition(snapshot, {
        type: "lifecycleObserved",
        running: true,
        state: "active",
        turnId: "",
      });
      expect(unknownTurn.context.stop.phase).toBe("stopping");

      // A known fresh turn (the killed-and-rebound replacement) means the stopped one is gone.
      const [rebound] = store.transition(unknownTurn, {
        type: "lifecycleObserved",
        running: true,
        state: "active",
        turnId: "turn-2",
      });
      expect(rebound.context.stop.phase).toBe("idle");

      // When the daemon already read idle before the acknowledgement, acceptance settles at once.
      let early = store.getInitialSnapshot();
      [early] = store.transition(early, {
        type: "stopRequested",
        execute: () => new Promise<never>(() => undefined),
        failureMessage: null,
        scope: "turn",
        turnId: "",
      });
      [early] = store.transition(early, {
        type: "stopSucceeded",
        requestId: early.context.stop.requestId,
      });
      expect(early.context.stop.phase).toBe("idle");
    });

    // Invariant (US-009.EC-2): only the daemon's own `nothing-in-flight` verdict
    // on the operator's turn stop reads "Completed · the stop arrived after the
    // turn finished" — it settles the stop at once, never claims canceled, and
    // the store clears the note after its window without any remount.
    it("Should read a nothing-in-flight verdict as a completed turn with a transient note", async () => {
      vi.useFakeTimers();
      try {
        const store = createSessionPageControlsLogic().createStore();
        store.trigger.lifecycleObserved({ running: true, state: "active", turnId: "turn-1" });
        store.trigger.stopRequested({
          execute: () =>
            Promise.resolve({ outcome: "nothing-in-flight", session_id: "sess-1", turn_id: "" }),
          failureMessage: null,
          scope: "turn",
          turnId: "turn-1",
        });
        await vi.advanceTimersByTimeAsync(0);
        expect(store.getSnapshot().context.stop.phase).toBe("idle");
        expect(store.getSnapshot().context.stopCompletion).toMatchObject({ requestId: 1 });

        // A stale lifecycle that still reports the turn does not resurrect the stop.
        store.trigger.lifecycleObserved({ running: true, state: "active", turnId: "turn-1" });
        expect(store.getSnapshot().context.stop.phase).toBe("idle");

        await vi.advanceTimersByTimeAsync(STOP_COMPLETION_NOTE_MS);
        expect(store.getSnapshot().context.stopCompletion).toBeNull();
      } finally {
        vi.useRealTimers();
      }
    });

    it("Should keep a canceled verdict stopping until the daemon drops the turn and clear a note on a new stop", async () => {
      const store = createSessionPageControlsLogic().createStore();
      store.trigger.lifecycleObserved({ running: true, state: "active", turnId: "turn-1" });
      store.trigger.stopRequested({
        execute: () =>
          Promise.resolve({ outcome: "canceled", session_id: "sess-1", turn_id: "turn-1" }),
        failureMessage: null,
        scope: "turn",
        turnId: "turn-1",
      });
      await waitFor(() => expect(store.getSnapshot().context.stop.phase).toBe("stopping"));
      expect(store.getSnapshot().context.stopCompletion).toBeNull();
      store.trigger.lifecycleObserved({ running: false, state: "active", turnId: "" });
      expect(store.getSnapshot().context.stop.phase).toBe("idle");

      // A note from an earlier verdict is superseded by the next request.
      let snapshot = store.getSnapshot();
      [snapshot] = store.transition(
        { ...snapshot, context: { ...snapshot.context, stopCompletion: { requestId: 1, at: 0 } } },
        {
          type: "stopRequested",
          execute: () => new Promise<never>(() => undefined),
          failureMessage: null,
          scope: "turn",
          turnId: "turn-2",
        }
      );
      expect(snapshot.context.stopCompletion).toBeNull();
      expect(stopVerdictFromResult({ outcome: "nothing-in-flight" })).toBe("nothing-in-flight");
      expect(stopVerdictFromResult(undefined)).toBeNull();
      expect(stopVerdictFromResult({ outcome: "elsewhere" })).toBeNull();
    });

    it("Should settle a session stop once the daemon itself reads stopping or stopped", () => {
      const store = createSessionPageControlsLogic().createStore();
      let snapshot = store.getInitialSnapshot();
      [snapshot] = store.transition(snapshot, {
        type: "lifecycleObserved",
        running: true,
        state: "active",
        turnId: "turn-1",
      });
      [snapshot] = store.transition(snapshot, {
        type: "stopRequested",
        execute: () => new Promise<never>(() => undefined),
        failureMessage: null,
        scope: "session",
        turnId: "turn-1",
      });
      [snapshot] = store.transition(snapshot, {
        type: "stopSucceeded",
        requestId: snapshot.context.stop.requestId,
      });
      expect(snapshot.context.stop.phase).toBe("stopping");
      // Losing the turn is not enough for a session stop: the session itself must move.
      [snapshot] = store.transition(snapshot, {
        type: "lifecycleObserved",
        running: false,
        state: "active",
        turnId: "",
      });
      expect(snapshot.context.stop.phase).toBe("stopping");
      [snapshot] = store.transition(snapshot, {
        type: "lifecycleObserved",
        running: false,
        state: "stopping",
        turnId: "",
      });
      expect(snapshot.context.stop.phase).toBe("idle");
    });

    it("Should return a failed stop to idle so the operator can retry", async () => {
      const store = createSessionPageControlsLogic().createStore();
      const execute = vi
        .fn()
        .mockRejectedValueOnce(new Error("daemon disconnected"))
        .mockResolvedValue(undefined);
      store.trigger.lifecycleObserved({ running: true, state: "active", turnId: "turn-1" });

      store.trigger.stopRequested({
        execute,
        failureMessage: null,
        scope: "turn",
        turnId: "turn-1",
      });
      await waitFor(() => expect(store.getSnapshot().context.stop.phase).toBe("idle"));

      store.trigger.stopRequested({
        execute,
        failureMessage: null,
        scope: "turn",
        turnId: "turn-1",
      });
      expect(execute).toHaveBeenCalledTimes(2);
      await waitFor(() => expect(store.getSnapshot().context.stop.phase).toBe("stopping"));
    });

    // Invariant (US-009.AC-3, ADR-004 invariant 3): a stop the daemon could not
    // verify stays `stopping` with durable attention; the explicit retry is the
    // same session stop through the same guard, waited on until the daemon's
    // settled answer arrives. Only that answer — verified, unverified, or a
    // request error — releases the guard: not acceptance, not a reread, not an
    // unrelated metadata change on the session, not a timer.
    it("Should hold a waited retry of an unverified stop until its own answer settles", async () => {
      const store = createSessionPageControlsLogic().createStore();
      const stopping = { running: false, state: "stopping" as const, turnId: "" };
      store.trigger.lifecycleObserved(stopping);
      expect(store.getSnapshot().context.stop.phase).toBe("idle");

      const settled = createDeferred<{ status: string; verified: boolean }>();
      const execute = vi.fn(() => settled.promise);
      store.trigger.stopRequested({
        execute,
        failureMessage: null,
        retry: true,
        scope: "session",
        turnId: "",
      });
      expect(store.getSnapshot().context.stop).toMatchObject({ phase: "pending", retry: true });
      expect(isStopRetryPending(store.getSnapshot().context)).toBe(true);

      // A second activation while the retry is pending is dropped, like any stop.
      store.trigger.stopRequested({
        execute,
        failureMessage: null,
        retry: true,
        scope: "session",
        turnId: "",
      });
      expect(execute).toHaveBeenCalledOnce();

      // Unrelated rereads while the daemon still reads `stopping` (a rename, a
      // presence lease, any metadata stamp) do not release the guard.
      store.trigger.lifecycleObserved(stopping);
      store.trigger.lifecycleObserved({ ...stopping });
      expect(isStopRetryPending(store.getSnapshot().context)).toBe(true);
      expect(store.getSnapshot().context.stop.phase).toBe("pending");

      // The daemon settled this retry unverified: the guard releases, the read
      // model still carries the attention, and the operator may retry again.
      settled.resolve({ status: "stopping", verified: false });
      await waitFor(() => expect(store.getSnapshot().context.stop.phase).toBe("idle"));
      expect(isStopRetryPending(store.getSnapshot().context)).toBe(false);
      store.trigger.stopRequested({
        execute,
        failureMessage: null,
        retry: true,
        scope: "session",
        turnId: "",
      });
      expect(execute).toHaveBeenCalledTimes(2);
    });

    it("Should release a failed retry with its diagnostic so the operator can retry again", async () => {
      const store = createSessionPageControlsLogic().createStore();
      store.trigger.lifecycleObserved({ running: false, state: "stopping", turnId: "" });
      const execute = vi
        .fn()
        .mockRejectedValueOnce(new Error("daemon disconnected"))
        .mockImplementation(() => new Promise<never>(() => undefined));

      store.trigger.stopRequested({
        execute,
        failureMessage: null,
        retry: true,
        scope: "session",
        turnId: "",
      });
      await waitFor(() => expect(store.getSnapshot().context.stop.phase).toBe("idle"));
      expect(toast.error).toHaveBeenCalledWith("daemon disconnected");
      expect(isStopRetryPending(store.getSnapshot().context)).toBe(false);

      store.trigger.stopRequested({
        execute,
        failureMessage: null,
        retry: true,
        scope: "session",
        turnId: "",
      });
      expect(execute).toHaveBeenCalledTimes(2);
      expect(isStopRetryPending(store.getSnapshot().context)).toBe(true);
    });
  });

  it("Should retain a send whose acknowledgment was lost and resolve it on a replayed outcome (UT-128)", () => {
    const store = createSessionPageControlsLogic().createStore();
    const pending = () => new Promise<never>(() => undefined);
    let snapshot = store.getInitialSnapshot();
    [snapshot] = store.transition(snapshot, {
      type: "busyInputRequested",
      execute: pending,
      kind: "queue",
      message: envelope.text,
      send: envelope,
    });
    [snapshot] = store.transition(snapshot, {
      type: "busyInputFailed",
      requestId: snapshot.context.busyInput.requestId,
      error: new TypeError("Failed to fetch"),
    });
    expect(snapshot.context.unconfirmedSends).toEqual([
      { ...envelope, id: "msg-1", phase: "unconfirmed" },
    ]);

    // Retry replays the same identity; the row waits as "retrying" until the answer.
    [snapshot] = store.transition(snapshot, {
      type: "busyInputRequested",
      execute: pending,
      kind: "queue",
      message: envelope.text,
      retryOf: "msg-1",
      send: envelope,
    });
    expect(snapshot.context.busyInput).toMatchObject({ phase: "pending", retryOf: "msg-1" });
    expect(snapshot.context.unconfirmedSends[0]?.phase).toBe("retrying");
    // A second Retry while the replay is in flight is dropped, as is a discard.
    const inFlight = snapshot;
    [snapshot] = store.transition(snapshot, { type: "unconfirmedSendDiscarded", id: "msg-1" });
    expect(snapshot.context.unconfirmedSends).toEqual(inFlight.context.unconfirmedSends);

    [snapshot] = store.transition(snapshot, {
      type: "busyInputSucceeded",
      requestId: snapshot.context.busyInput.requestId,
      result: {
        delivery: "after_turn",
        disposition: "queued",
        idempotency_key: "idk-1",
        message_id: "msg-1",
        queue_position: 2,
        replayed: true,
        status: "queued",
      },
    });
    expect(snapshot.context.unconfirmedSends).toEqual([]);
  });

  it("Should treat a daemon refusal as an outcome and remember the queue cap it named", () => {
    const store = createSessionPageControlsLogic().createStore();
    const pending = () => new Promise<never>(() => undefined);
    let snapshot = store.getInitialSnapshot();
    [snapshot] = store.transition(snapshot, {
      type: "busyInputRequested",
      execute: pending,
      kind: "queue",
      message: envelope.text,
      send: envelope,
    });
    [snapshot] = store.transition(snapshot, {
      type: "busyInputFailed",
      requestId: snapshot.context.busyInput.requestId,
      error: new SessionApiError("queue full", 409, "sess-1", { code: "queue_full", queueCap: 10 }),
    });
    // The daemon answered: nothing is unconfirmed, and the cap is now known.
    expect(snapshot.context.unconfirmedSends).toEqual([]);
    expect(snapshot.context.queueCap).toBe(10);

    // A lost replay returns to waiting; a refused replay resolves the row.
    [snapshot] = store.transition(snapshot, {
      type: "busyInputRequested",
      execute: pending,
      kind: "queue",
      message: envelope.text,
      send: envelope,
    });
    [snapshot] = store.transition(snapshot, {
      type: "busyInputFailed",
      requestId: snapshot.context.busyInput.requestId,
      error: new SessionApiError("gateway", 504),
    });
    [snapshot] = store.transition(snapshot, {
      type: "busyInputRequested",
      execute: pending,
      kind: "queue",
      message: envelope.text,
      retryOf: "msg-1",
      send: envelope,
    });
    [snapshot] = store.transition(snapshot, {
      type: "busyInputFailed",
      requestId: snapshot.context.busyInput.requestId,
      error: new SessionApiError("conflict", 409, "sess-1", { code: "send_conflict" }),
    });
    expect(snapshot.context.unconfirmedSends).toEqual([]);
  });

  it("Should drop a waiting unconfirmed row on discard without asking the daemon", () => {
    const store = createSessionPageControlsLogic().createStore();
    let snapshot = store.getInitialSnapshot();
    [snapshot] = store.transition(snapshot, {
      type: "busyInputRequested",
      execute: () => new Promise<never>(() => undefined),
      kind: "steer",
      message: envelope.text,
      send: { ...envelope, action: "steer" },
    });
    [snapshot] = store.transition(snapshot, {
      type: "busyInputFailed",
      requestId: snapshot.context.busyInput.requestId,
      error: new SessionApiError("gateway", 502),
    });
    expect(snapshot.context.unconfirmedSends).toHaveLength(1);
    [snapshot] = store.transition(snapshot, { type: "unconfirmedSendDiscarded", id: "msg-1" });
    expect(snapshot.context.unconfirmedSends).toEqual([]);
  });

  it("Should retain a streaming prompt that got no answer as unconfirmed with its own identity", () => {
    const store = createSessionPageControlsLogic().createStore();
    let snapshot = store.getInitialSnapshot();
    const prompt: SessionSendEnvelope = {
      ...envelope,
      action: "prompt",
      identity: { idempotencyKey: "idk-p", messageId: "msg-p" },
    };
    [snapshot] = store.transition(snapshot, { type: "sendUnconfirmedObserved", send: prompt });
    expect(snapshot.context.unconfirmedSends).toEqual([
      { ...prompt, id: "msg-p", phase: "unconfirmed" },
    ]);
    // The same identity observed twice is still one row.
    [snapshot] = store.transition(snapshot, { type: "sendUnconfirmedObserved", send: prompt });
    expect(snapshot.context.unconfirmedSends).toHaveLength(1);
  });
});
