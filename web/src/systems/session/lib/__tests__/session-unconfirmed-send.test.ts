import { describe, expect, it } from "vitest";

import { SessionApiError } from "../../adapters/session-api-errors";
import {
  beginUnconfirmedRetry,
  findUnconfirmedSend,
  isSendAcknowledgmentLost,
  markSendUnconfirmed,
  resolveUnconfirmedSend,
  type SessionSendEnvelope,
} from "../session-unconfirmed-send";

// Suite: client-local unconfirmed send derivation (UT-128).
// Invariant: a send whose acknowledgment is lost is retained with its full identity
// and content until an observed outcome or an explicit discard; a daemon answer is
// never mistaken for a lost acknowledgment; Retry replays exactly the same identity.
// Boundary IN: failed send errors and the envelope on the wire.
// Boundary OUT: the page-controls store context and the strip's unconfirmed rows.
describe("session unconfirmed send derivation", () => {
  const envelope: SessionSendEnvelope = {
    action: "queue",
    attachments: [],
    expectedTurnId: "turn-1",
    identity: { idempotencyKey: "idk-1", messageId: "msg-1" },
    runtime: null,
    text: "Also run the migration equivalence suite",
  };

  it("Should retain the identity whenever the daemon's admission is unknown", () => {
    // Network loss, a mid-POST disconnect, gateway timeouts, an abort that may
    // have fired after the bytes left, and a 5xx after durable acceptance all
    // leave the outcome ambiguous: only a replay can settle it.
    expect(isSendAcknowledgmentLost(new TypeError("Failed to fetch"))).toBe(true);
    expect(isSendAcknowledgmentLost(new SessionApiError("gateway", 502))).toBe(true);
    expect(isSendAcknowledgmentLost(new SessionApiError("gateway", 504))).toBe(true);
    expect(isSendAcknowledgmentLost(new SessionApiError("no response", 0))).toBe(true);
    expect(isSendAcknowledgmentLost(new SessionApiError("internal", 500))).toBe(true);
    expect(isSendAcknowledgmentLost(new DOMException("aborted", "AbortError"))).toBe(true);
    expect(isSendAcknowledgmentLost(new Error("Busy input failed"))).toBe(true);
  });

  it("Should treat a daemon 4xx answer as an authoritative outcome, never ack loss", () => {
    expect(
      isSendAcknowledgmentLost(new SessionApiError("conflict", 409, "s", { code: "send_conflict" }))
    ).toBe(false);
    expect(
      isSendAcknowledgmentLost(new SessionApiError("full", 409, "s", { code: "queue_full" }))
    ).toBe(false);
    expect(isSendAcknowledgmentLost(new SessionApiError("gone", 404))).toBe(false);
    expect(isSendAcknowledgmentLost(new SessionApiError("bad", 400))).toBe(false);
    expect(isSendAcknowledgmentLost(null)).toBe(false);
  });

  it("Should retain the full identity and content, keyed by the message id, exactly once", () => {
    const marked = markSendUnconfirmed([], envelope);
    expect(marked).toEqual([{ ...envelope, id: "msg-1", phase: "unconfirmed" }]);
    // The same identity failing again is the same row, never a second one.
    expect(markSendUnconfirmed(marked, envelope)).toHaveLength(1);
    expect(findUnconfirmedSend(marked, "msg-1")?.identity).toEqual({
      idempotencyKey: "idk-1",
      messageId: "msg-1",
    });
  });

  it("Should mark a replay in flight and return it to waiting when the replay is also lost", () => {
    const retrying = beginUnconfirmedRetry(markSendUnconfirmed([], envelope), "msg-1");
    expect(retrying[0]?.phase).toBe("retrying");
    // A second retry request while one is in flight changes nothing.
    expect(beginUnconfirmedRetry(retrying, "msg-1")).toEqual(retrying);
    const lostAgain = markSendUnconfirmed(retrying, envelope);
    expect(lostAgain).toEqual([{ ...envelope, id: "msg-1", phase: "unconfirmed" }]);
  });

  it("Should resolve the marker on an observed outcome or a discard and leave others alone", () => {
    const other: SessionSendEnvelope = {
      ...envelope,
      identity: { idempotencyKey: "idk-2", messageId: "msg-2" },
      text: "second",
    };
    const both = markSendUnconfirmed(markSendUnconfirmed([], envelope), other);
    expect(resolveUnconfirmedSend(both, "msg-1").map(send => send.id)).toEqual(["msg-2"]);
    expect(resolveUnconfirmedSend(both, "msg-missing")).toEqual(both);
  });
});
