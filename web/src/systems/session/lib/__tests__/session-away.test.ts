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

  it("Should exclude a picked proposal from pending decisions and keep its card visible", () => {
    const title = "Unblock the token migration";
    const reports = AWAY.reports.map(report =>
      report.kind === "proposal" ? { ...report, proposals: [{ title, body: "" }] } : report
    );
    const messages = [
      { role: "user", createdAt: new Date(NOW), content: [{ type: "text", text: title }] },
    ];
    const digest = deriveAwayDigest({ ...AWAY, reports }, "Triage Lead", NOW, messages);
    expect(digest?.headline).toBe("Nine quiet hours, nothing for you to decide.");
    expect(digest?.key.find(item => item.beat === "needs-you")?.count).toBe(0);
    expect(digest?.foldedIds.has("p1")).toBe(false);
  });

  it("Should say it paused, when, and what's waiting [UT-080]", () => {
    const pause = {
      ...report("u1", "failure", "2026-11-01T04:12:00Z"),
      reason: "unattended",
    };
    const digest = deriveAwayDigest(
      { ...AWAY, reports: [...AWAY.reports, pause] },
      "Triage Lead",
      NOW,
      [],
      { held: 2, pausedNow: true }
    );
    expect(digest?.paused).toBe(true);
    expect(digest?.headline).toBe("Nine hours of work, then it paused.");
    expect(digest?.summary).toMatch(
      /^Triage Lead reached its limit at \d\d:\d\d\. 2 things are waiting\.$/
    );
    expect(digest?.events.find(event => event.id === "u1")?.title).toBe(
      "It paused until you're back"
    );
  });

  it("Should tell a resumed pause in the past tense, with no Resume [FE-5]", () => {
    const pause = {
      ...report("u1", "failure", "2026-11-01T04:12:00Z"),
      reason: "unattended",
    };
    const digest = deriveAwayDigest(
      { ...AWAY, reports: [...AWAY.reports, pause] },
      "Triage Lead",
      NOW,
      [],
      { held: 0, pausedNow: false }
    );
    expect(digest?.paused).toBe(false);
    expect(digest?.headline).not.toMatch(/then it paused/);
    expect(digest?.summary).toMatch(/It paused at \d\d:\d\d, then you resumed it\.$/);
  });

  it("Should tell a lost wake apart from an interrupted approval", () => {
    const lost = { ...report("w1", "interrupted", "2026-11-01T03:02:00Z"), cause: "undelivered" };
    const restart = {
      ...report("r1", "interrupted", "2026-11-01T03:10:00Z"),
      cause: "daemon_restart",
    };
    const digest = deriveAwayDigest(
      { ...AWAY, reports: [...AWAY.reports, lost, restart] },
      "Triage Lead",
      NOW
    );
    const title = (id: string) => digest?.events.find(event => event.id === id)?.title;
    expect(title("w1")).toBe("A wake-up didn't arrive");
    expect(title("r1")).toBe("Some work stopped");
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
