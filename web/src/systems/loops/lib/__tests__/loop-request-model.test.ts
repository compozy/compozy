import { describe, expect, it } from "vitest";

import {
  answeredAskRequest,
  canceledReviewRequest,
  expiredAskRequest,
  nearExpiryAskRequest,
  pendingAskRequest,
  pendingReviewRequest,
} from "../../mocks/fixture-graph-eng-requests";
import {
  loopRequestDecisionCarriesPayload,
  loopRequestDecisionSchema,
  loopRequestExpiry,
  pendingLoopRequestCount,
  projectLoopRequest,
} from "../loop-request-model";
import {
  LOOP_REQUEST_NEAR_EXPIRY_SIGNAL,
  LOOP_REQUEST_STATE_SIGNAL,
} from "../loop-request-vocabulary";

const NOW = Date.parse("2026-08-17T09:00:00Z");

describe("projectLoopRequest", () => {
  it("Should keep a pending request unanswerable once its run has terminated", () => {
    const live = projectLoopRequest(pendingAskRequest, { nowMs: NOW, runStatus: "running" });
    expect(live.isAnswerable).toBe(true);

    for (const status of ["done", "failed", "canceled"] as const) {
      const terminated = projectLoopRequest(pendingAskRequest, { nowMs: NOW, runStatus: status });
      expect(terminated.state).toBe("pending");
      expect(terminated.isAnswerable).toBe(false);
    }
  });

  it("Should narrow decisions to the closed vocabulary so an unknown daemon string never renders", () => {
    const view = projectLoopRequest(
      { ...pendingReviewRequest, decisions: ["approve", "escalate", "reject", "delegate"] },
      { nowMs: NOW, runStatus: "running" }
    );
    expect(view.decisions).toEqual(["approve", "reject"]);
  });

  it("Should narrow an unknown kind and state rather than rendering the daemon's raw word", () => {
    const view = projectLoopRequest(
      { ...pendingAskRequest, kind: "interrogate", state: "escalating" },
      { nowMs: NOW, runStatus: "running" }
    );
    expect(view.kind).toBe("ask");
    expect(view.state).toBe("pending");
    expect(view.title).toBe("Answer requested");
    expect(view.waitSentence).toBe("is waiting for an answer");
  });

  it("Should swap the signal near expiry while keeping the pending tone", () => {
    const calm = projectLoopRequest(pendingAskRequest, { nowMs: NOW, runStatus: "running" });
    expect(calm.signal).toBe(LOOP_REQUEST_STATE_SIGNAL.pending);

    const soon = projectLoopRequest(nearExpiryAskRequest, { nowMs: NOW, runStatus: "running" });
    expect(soon.signal).toBe(LOOP_REQUEST_NEAR_EXPIRY_SIGNAL);
    expect(soon.signal.tone).toBe(LOOP_REQUEST_STATE_SIGNAL.pending.tone);
    expect(soon.signal.word).toBe("expires soon");
    expect(soon.expiry).toMatchObject({ isNearExpiry: true, isPast: false, label: "expires 4m" });
  });

  it("Should drop the near-expiry countdown once the run can no longer be answered", () => {
    const terminated = projectLoopRequest(nearExpiryAskRequest, {
      nowMs: NOW,
      runStatus: "canceled",
    });
    expect(terminated.isAnswerable).toBe(false);
    expect(terminated.signal).toBe(LOOP_REQUEST_STATE_SIGNAL.pending);
  });

  it("Should carry resolution provenance and no form on an answered, expired, or canceled request", () => {
    const answered = projectLoopRequest(answeredAskRequest, { nowMs: NOW, runStatus: "running" });
    expect(answered.isAnswerable).toBe(false);
    expect(answered.signal).toBe(LOOP_REQUEST_STATE_SIGNAL.answered);
    expect(answered.resolution).toEqual({
      decision: "respond",
      actorKind: "operator",
      actorId: "pedro",
      at: "2026-08-17T09:12:00Z",
    });

    const expired = projectLoopRequest(expiredAskRequest, { nowMs: NOW, runStatus: "running" });
    expect(expired.isAnswerable).toBe(false);
    expect(expired.signal).toBe(LOOP_REQUEST_STATE_SIGNAL.expired);

    expect(expired.resolution).toEqual({
      decision: "",
      actorKind: "",
      actorId: "",
      at: "2026-08-17T09:02:00Z",
    });

    const canceled = projectLoopRequest(canceledReviewRequest, { nowMs: NOW, runStatus: "done" });
    expect(canceled.isAnswerable).toBe(false);
    expect(canceled.signal).toBe(LOOP_REQUEST_STATE_SIGNAL.canceled);
    expect(canceled.resolution).toMatchObject({ actorId: "pedro", at: "2026-08-17T09:30:00Z" });

    expect(
      projectLoopRequest(pendingAskRequest, { nowMs: NOW, runStatus: "running" }).resolution
    ).toBeNull();
  });

  it("Should name a fan-out lane only when the request belongs to one", () => {
    expect(projectLoopRequest(pendingAskRequest, { nowMs: NOW }).laneLabel).toBe("");
    expect(
      projectLoopRequest({ ...pendingReviewRequest, item_index: 2 }, { nowMs: NOW }).laneLabel
    ).toBe("lane 2");
  });

  it("Should permit agent responders only on the nodes that opted in", () => {
    expect(projectLoopRequest(pendingAskRequest, { nowMs: NOW }).agentsPermitted).toBe(true);
    expect(projectLoopRequest(pendingReviewRequest, { nowMs: NOW }).agentsPermitted).toBe(false);
  });
});

describe("loopRequestExpiry", () => {
  it("Should return null when the daemon declared no readable deadline", () => {
    expect(loopRequestExpiry(null, NOW)).toBeNull();
    expect(loopRequestExpiry(undefined, NOW)).toBeNull();
    expect(loopRequestExpiry("not-a-timestamp", NOW)).toBeNull();
  });

  it("Should phrase the remaining window in one unit and never round a live request to zero", () => {
    expect(loopRequestExpiry("2026-08-17T09:00:30Z", NOW)?.label).toBe("expires 1m");
    expect(loopRequestExpiry("2026-08-17T09:40:00Z", NOW)?.label).toBe("expires 40m");
    expect(loopRequestExpiry("2026-08-17T10:30:00Z", NOW)?.label).toBe("expires 1h");
    expect(loopRequestExpiry("2026-08-19T15:00:00Z", NOW)?.label).toBe("expires 2d");
  });

  it("Should mark a passed deadline expired rather than counting down past zero", () => {
    const past = loopRequestExpiry("2026-08-17T08:59:00Z", NOW);
    expect(past).toMatchObject({ label: "expired", isPast: true, isNearExpiry: false });
    expect(past?.remainingMs).toBeLessThan(0);
  });
});

describe("loopRequestDecisionSchema", () => {
  it("Should validate an edit against the action's edit_schema", () => {
    expect(loopRequestDecisionSchema(pendingReviewRequest, "edit")).toBe(
      pendingReviewRequest.edit_schema
    );
  });

  it("Should validate a respond against respond_schema on a review and expect on an ask", () => {
    expect(loopRequestDecisionSchema(pendingReviewRequest, "respond")).toBe(
      pendingReviewRequest.respond_schema
    );
    expect(loopRequestDecisionSchema(pendingAskRequest, "respond")).toBe(pendingAskRequest.expect);
  });

  it("Should give approve and reject no payload schema at all", () => {
    expect(loopRequestDecisionSchema(pendingReviewRequest, "approve")).toBeUndefined();
    expect(loopRequestDecisionSchema(pendingReviewRequest, "reject")).toBeUndefined();
    expect(loopRequestDecisionCarriesPayload("approve")).toBe(false);
    expect(loopRequestDecisionCarriesPayload("reject")).toBe(false);
    expect(loopRequestDecisionCarriesPayload("edit")).toBe(true);
    expect(loopRequestDecisionCarriesPayload("respond")).toBe(true);
  });
});

describe("pendingLoopRequestCount", () => {
  it("Should count only the requests still waiting on a person", () => {
    expect(
      pendingLoopRequestCount([
        pendingAskRequest,
        pendingReviewRequest,
        answeredAskRequest,
        expiredAskRequest,
        canceledReviewRequest,
      ])
    ).toBe(2);
    expect(pendingLoopRequestCount([])).toBe(0);
  });
});
