import { describe, expect, it } from "vitest";

import { SessionApiError } from "../../adapters/session-api-errors";
import {
  oppositeSessionBusyInputMode,
  sessionBusyInputDefaultMode,
  sessionSteerDelivery,
} from "../session-busy-input";
import {
  classifySessionBusyInputFailure,
  describeSessionBusyInputRefusal,
  describeSessionBusyInputUnconfirmed,
  SessionBusyInputRefusalError,
  sessionBusyInputRefusalFromError,
} from "../session-busy-input-refusal";
import { sessionSendOutcomeFromResult } from "../session-send-outcome";

// Suite: busy-send read models (default mode, disposition envelope, refusal reasons).
// Invariant: the composer's Enter default, the inline disposition, and every refusal
// reason derive from daemon truth through one closed vocabulary — no client invention.
// Boundary IN: session resource `busy_input`, the 202 envelope, typed API errors.
// Boundary OUT: composer rendering and route-hook gates.
describe("session busy-input read models", () => {
  it("Should resolve the daemon follow-up default and its one-shot opposite", () => {
    expect(sessionBusyInputDefaultMode({ busy_input: undefined })).toBe("steer");
    expect(sessionBusyInputDefaultMode({ busy_input: null })).toBe("steer");
    expect(
      sessionBusyInputDefaultMode({
        busy_input: { default_mode: "queue", steer_capability: "none" },
      })
    ).toBe("queue");
    expect(
      sessionBusyInputDefaultMode({
        busy_input: { default_mode: "interrupt", steer_capability: "none" },
      })
    ).toBe("steer");
    expect(oppositeSessionBusyInputMode("steer")).toBe("queue");
    expect(oppositeSessionBusyInputMode("queue")).toBe("steer");
  });

  it("Should predict the steer delivery from the resolved capability before any send", () => {
    // A fresh session has a capability but no delivery record yet (US-002.AC-3).
    expect(
      sessionSteerDelivery({ busy_input: { default_mode: "steer", steer_capability: "steer_ext" } })
    ).toBe("injected");
    expect(
      sessionSteerDelivery({
        busy_input: { default_mode: "steer", steer_capability: "concurrent_prompt" },
      })
    ).toBe("pending_injection");
    expect(
      sessionSteerDelivery({ busy_input: { default_mode: "steer", steer_capability: "none" } })
    ).toBe("interrupt_fallback");
  });

  it("Should let the current capability win over the last send's delivery", () => {
    // The provider changed since the last steer: the record is stale, the capability is not.
    expect(
      sessionSteerDelivery({
        busy_input: {
          default_mode: "steer",
          steer_capability: "steer_ext",
          steer_delivery: "interrupt_fallback",
        },
      })
    ).toBe("injected");
    expect(
      sessionSteerDelivery({
        busy_input: { default_mode: "steer", steer_capability: "none", steer_delivery: "injected" },
      })
    ).toBe("interrupt_fallback");
  });

  it("Should answer null when the session resource carries no busy-input block", () => {
    // Without a capability there is nothing honest to predict — never a guess
    // from history. The contract's capability enum is closed, so an absent
    // block is the only unknown the client can see.
    expect(sessionSteerDelivery({ busy_input: undefined })).toBeNull();
    expect(sessionSteerDelivery({ busy_input: null })).toBeNull();
  });

  it("Should flatten the disposition envelope, falling back to the legacy status", () => {
    expect(
      sessionSendOutcomeFromResult({
        delivery: "direct",
        disposition: "steering",
        entry_id: "inp_4d9",
        idempotency_key: "idk_1f77",
        message_id: "msg_01k4",
        queue_position: 0,
        replayed: false,
        status: "steering",
        steer_delivery: "interrupt_fallback",
        turn_id: "t_9f2",
      })
    ).toEqual({
      disposition: "steering",
      entryId: "inp_4d9",
      idempotencyKey: "idk_1f77",
      messageId: "msg_01k4",
      queuePosition: null,
      replayed: false,
      steerDelivery: "interrupt_fallback",
      turnId: "t_9f2",
    });
    expect(
      sessionSendOutcomeFromResult({
        delivery: "after_turn",
        idempotency_key: "idk",
        message_id: "msg",
        queue_entry_id: "inq-1",
        queue_position: 3,
        replayed: true,
        status: "queued",
      })
    ).toMatchObject({ disposition: "queued", entryId: "inq-1", queuePosition: 3, replayed: true });
    expect(
      sessionSendOutcomeFromResult({ direct_turn: true, idempotency_key: "idk", message_id: "msg" })
    ).toMatchObject({ disposition: "direct", turnId: null });
    expect(
      sessionSendOutcomeFromResult({
        outcome: "cleared",
        reason_code: null,
        replaced_run_id: null,
        snapshot: null,
      })
    ).toBeNull();
  });

  it("Should map daemon refusal codes, aborts, and transport failures to one reason set", () => {
    expect(sessionBusyInputRefusalFromError(new DOMException("gone", "AbortError"))).toBeNull();
    expect(
      sessionBusyInputRefusalFromError(
        new SessionApiError("mismatch", 409, "sess-1", {
          code: "active_turn_mismatch",
          currentTurnId: "t_9f3",
        })
      )
    ).toMatchObject({ code: "active_turn_mismatch", currentTurnId: "t_9f3" });
    // A fence refusal with no live turn means the turn settled: send normally.
    expect(
      sessionBusyInputRefusalFromError(
        new SessionApiError("mismatch", 409, "sess-1", { code: "active_turn_mismatch" })
      )
    ).toMatchObject({ code: "turn_ended" });
    expect(
      sessionBusyInputRefusalFromError(
        new SessionApiError("files", 422, "sess-1", { code: "steer_attachments_unsupported" }),
        { attachmentCount: 2 }
      )
    ).toMatchObject({ attachmentCount: 2, code: "steer_attachments_unsupported" });
    expect(sessionBusyInputRefusalFromError(new Error("daemon unreachable"))).toEqual({
      attachmentCount: 0,
      code: "not_delivered",
      currentTurnId: null,
      message: "daemon unreachable",
      queueCap: null,
    });
    const gate = new SessionBusyInputRefusalError({ code: "send_in_flight" });
    expect(sessionBusyInputRefusalFromError(gate)).toEqual(gate.refusal);
  });

  // Invariant (task_03/06 truth): only a proven non-delivery reads as a refusal — a client
  // gate or the daemon's own 4xx answer. A transport failure, a 5xx or an unknown error
  // leaves admission unknown (the daemon may have accepted the send before the answer was
  // lost): the same classifier that retains the identity for Retry says "unconfirmed".
  // Owner: busy-input failure classification (lib). Canonical suite: this file.
  it("Should tell a proven refusal from an unacknowledged send with the retention classifier", () => {
    expect(classifySessionBusyInputFailure(new DOMException("gone", "AbortError"))).toBeNull();
    const gate = new SessionBusyInputRefusalError({ code: "send_in_flight" });
    expect(classifySessionBusyInputFailure(gate)).toEqual({
      kind: "refusal",
      refusal: gate.refusal,
    });
    expect(
      classifySessionBusyInputFailure(
        new SessionApiError("full", 409, "sess-1", { code: "queue_full", queueCap: 10 })
      )
    ).toMatchObject({ kind: "refusal", refusal: { code: "queue_full", queueCap: 10 } });
    // A daemon 4xx without a known code still answered: proven, with its own sentence.
    expect(
      classifySessionBusyInputFailure(new SessionApiError("no such session", 404, "sess-1"))
    ).toMatchObject({
      kind: "refusal",
      refusal: { code: "not_delivered", message: "no such session" },
    });
    expect(classifySessionBusyInputFailure(new TypeError("Failed to fetch"))).toEqual({
      kind: "unconfirmed",
      message: "Failed to fetch",
    });
    expect(classifySessionBusyInputFailure(new SessionApiError("upstream", 502, "sess-1"))).toEqual(
      { kind: "unconfirmed", message: "upstream" }
    );
    expect(classifySessionBusyInputFailure(new Error(""))).toEqual({
      kind: "unconfirmed",
      message: null,
    });
    expect(describeSessionBusyInputUnconfirmed("Failed to fetch")).toBe(
      "Not confirmed — Failed to fetch. Retry replays the same message; nothing is sent twice."
    );
    expect(describeSessionBusyInputUnconfirmed(null)).toBe(
      "Not confirmed — CompozyOS didn't answer. Retry replays the same message; nothing is sent twice."
    );
  });

  it("Should describe every refusal as a 'Not sent' sentence with its reason", () => {
    expect(
      describeSessionBusyInputRefusal({
        attachmentCount: 2,
        code: "steer_attachments_unsupported",
        currentTurnId: null,
        message: null,
        queueCap: null,
      })
    ).toBe("Not sent — steer can't carry files on this agent. Queue it, or remove the 2 files.");
    expect(
      describeSessionBusyInputRefusal({
        attachmentCount: 0,
        code: "turn_ended",
        currentTurnId: null,
        message: null,
        queueCap: null,
      })
    ).toBe("Not sent — the turn ended. Send it normally.");
    expect(
      describeSessionBusyInputRefusal({
        attachmentCount: 0,
        code: "not_delivered",
        currentTurnId: null,
        message: null,
        queueCap: null,
      })
    ).toBe("Not sent — CompozyOS didn't answer. Your draft is back.");
  });
});
