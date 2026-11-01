import type { AgentInstanceAway } from "@/systems/agent";

import type { ReportAgent } from "../lib/session-report-identity";

export const REPORT_TRIAGE: ReportAgent = {
  label: "Triage Lead",
  appearance: { avatar: "generated", seed: "triage" },
  provider: "claude",
};

export const REPORT_ENGINEER: ReportAgent = {
  label: "Code Engineer",
  appearance: { avatar: "generated", seed: "code-engineer" },
  provider: "codex",
};

export const REPORT_CHOICES = [
  {
    title: "Unblock the token migration",
    body: "Run 12 has been waiting on a review since yesterday.",
  },
  { title: "Close the stale design issue", body: "#918 was fixed in #931 and nobody closed it." },
];

export const AWAY_NOW = Date.parse("2026-11-01T08:30:00Z");

export const AWAY_FIXTURE: AgentInstanceAway = {
  last_seen_at: "2026-10-31T23:00:00Z",
  quiet_check_ins: 33,
  reports: [
    {
      id: "r1",
      kind: "failure",
      state: "pending",
      created_at: "2026-11-01T01:30:00Z",
      detail: "Both stopped after 10 minutes. The next one finished fine.",
    },
    {
      id: "r2",
      kind: "failure",
      state: "pending",
      created_at: "2026-11-01T02:30:00Z",
      detail: "It ran out of time.",
    },
    {
      id: "r3",
      kind: "approval_expired",
      state: "pending",
      created_at: "2026-11-01T03:40:00Z",
      tool: "make migrate",
    },
    { id: "r4", kind: "rotation_deferred", state: "pending", created_at: "2026-11-01T04:12:00Z" },
    { id: "r5", kind: "proposal", state: "pending", created_at: "2026-11-01T08:02:00Z" },
  ],
};
