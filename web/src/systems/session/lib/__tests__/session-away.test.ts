// Invariant: the away digest is written from daemon counts and folds only what needs no decision.
import { describe, expect, it } from "vitest";

import type { AgentInstanceAway, AgentInstanceReport } from "@/systems/agent";

import { deriveAwayDigest } from "../session-away";
import { pickedProposal } from "../session-report";

const NOW = Date.parse("2026-11-01T08:30:00Z");

function report(id: string, kind: AgentInstanceReport["kind"], at: string): AgentInstanceReport {
  return { id, kind, state: "pending", created_at: at };
}

const AWAY: AgentInstanceAway = {
  last_seen_at: "2026-10-31T23:00:00Z",
  quiet_check_ins: 33,
  reports: [
    report("p1", "proposal", "2026-11-01T08:02:00Z"),
    report("f1", "failure", "2026-11-01T01:30:00Z"),
    report("e1", "approval_expired", "2026-11-01T03:40:00Z"),
  ],
};

describe("deriveAwayDigest", () => {
  it("Should not count hours for an absence under one", () => {
    const start = new Date(NOW - 20 * 60 * 1000).toISOString();
    const reports = AWAY.reports.map(report => ({ ...report, created_at: start }));
    const digest = deriveAwayDigest({ ...AWAY, last_seen_at: start, reports }, "Triage Lead", NOW);
    expect(digest?.headline).toBe("Under an hour, one decision for you.");
  });

  it("Should tell the night in one card and keep the decision as its own", () => {
    const digest = deriveAwayDigest(AWAY, "Triage Lead", NOW);
    expect(digest?.firstReportId).toBe("f1");
    expect(digest?.foldedIds).toEqual(new Set(["f1", "e1"]));
    expect(digest?.headline).toBe("Nine quiet hours, one decision for you.");
    expect(digest?.summary).toBe("Triage Lead checked in 36 times. Most had nothing to report.");
    expect(digest?.key.find(item => item.beat === "quiet")?.count).toBe(33);
    expect(digest?.beats.filter(beat => beat === "failed")).toHaveLength(2);
  });

  it("Should stay away after a short absence", () => {
    expect(
      deriveAwayDigest(
        {
          last_seen_at: "2026-11-01T08:00:00Z",
          quiet_check_ins: 1,
          reports: AWAY.reports.slice(0, 1),
        },
        "Triage Lead",
        NOW
      )
    ).toBeNull();
  });

  it("Should need a report to hang from", () => {
    expect(
      deriveAwayDigest({ last_seen_at: null, quiet_check_ins: 40, reports: [] }, "Triage Lead", NOW)
    ).toBeNull();
  });
});

describe("pickedProposal", () => {
  const proposals = [{ title: "Unblock the token migration", body: "" }];
  const user = (text: string, at: string) => ({
    role: "user",
    createdAt: new Date(at),
    content: [{ type: "text", text }],
  });

  it("Should find the prompt a choice sent", () => {
    expect(
      pickedProposal(
        [user("Unblock the token migration", "2026-11-01T09:00:00Z")],
        proposals,
        "2026-11-01T08:02:00Z"
      )
    ).toBe("Unblock the token migration");
  });

  it("Should ignore the same words said before the report", () => {
    expect(
      pickedProposal(
        [user("Unblock the token migration", "2026-11-01T07:00:00Z")],
        proposals,
        "2026-11-01T08:02:00Z"
      )
    ).toBeUndefined();
  });
});
