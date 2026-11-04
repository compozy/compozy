/** The operator's always-on fleet, so the shell states read like a real Agents window (#1258). */
import type { ProfileData } from "../agent-profile/profile-fixtures";
import { PROFILES } from "../agent-profile/profile-fixtures";
import { LISTED_ON_DEMAND } from "../always-on-agents-shared/agents-fixtures";
import type { ListedAgent } from "../always-on-agents-shared/agents-page";
import type { DraftAgent } from "../always-on-agents-shared/fixtures";
import { primaryAgentFixture } from "@/systems/agent/mocks/fixtures";
import { scriptApprovalNoticesFixture } from "@/systems/agent/mocks/script-approval-fixtures";

function alwaysOn(name: string, label: string, description: string): DraftAgent {
  return {
    name,
    label,
    description,
    kind: "always_on",
    provider: "claude",
    model: "claude-opus-5-5",
    appearance: { avatar: "generated", seed: name },
    scopes: ["compozy"],
    active: 1,
  };
}

export const MERGE_TRAIN = alwaysOn(
  "merge-train",
  "Merge Train",
  "Lands green, approved pull requests in order and keeps main releasable"
);
export const PR_SHEPHERD = alwaysOn(
  "pr-shepherd",
  "PR Shepherd",
  "Walks open pull requests to green: CI, review threads and bot comments"
);
export const SESSION_SUPERVISOR = alwaysOn(
  "session-supervisor",
  "Session Supervisor",
  "Notices stalled or stuck sessions and nudges them along"
);
export const ISSUE_WATCH = alwaysOn(
  "issue-watch",
  "Issue Watch",
  "Reads new GitHub issues and files the ones that need you"
);

/** The Always-on group. Merge Train and Session Supervisor each have a script waiting. */
export function fleetAlwaysOn(flags: {
  mergeTrain?: ListedAgent["flag"];
  sessionSupervisor?: ListedAgent["flag"];
}): readonly ListedAgent[] {
  return [
    {
      ...MERGE_TRAIN,
      places: [{ scope: "compozy", state: "idle" }],
      activity: [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
      recap: "No reports yet",
      lastActive: "Added 8 min ago",
      flag: flags.mergeTrain,
    },
    {
      ...PR_SHEPHERD,
      places: [{ scope: "compozy", state: "working", now: "Waiting on CI for #1262" }],
      activity: [6, 9, 7, 12, 14, 4, 3, 8, 11, 13, 15, 12, 5, 9],
      recap: "96 reports in 2 weeks",
      lastActive: "Now",
    },
    {
      ...SESSION_SUPERVISOR,
      places: [{ scope: "compozy", state: "paused", pauseReason: "manual" }],
      activity: [4, 5, 4, 6, 5, 2, 2, 5, 4, 6, 5, 0, 0, 0],
      recap: "52 reports in 2 weeks",
      lastActive: "3 days ago",
      flag: flags.sessionSupervisor,
    },
    {
      ...ISSUE_WATCH,
      places: [{ scope: "compozy", state: "idle" }],
      activity: [2, 3, 1, 4, 2, 1, 0, 3, 2, 4, 3, 2, 1, 2],
      recap: "34 reports in 2 weeks",
      lastActive: "18 min ago",
    },
  ];
}

export const FLEET_ON_DEMAND = LISTED_ON_DEMAND;

/** Merge Train's home: added to compozy minutes ago, so it has nothing to show yet. */
export const MERGE_TRAIN_PROFILE: ProfileData = {
  ...PROFILES["always-on"],
  agent: MERGE_TRAIN,
  places: [
    {
      scope: "compozy",
      state: "idle",
      next: "in 22 min",
      activity: [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
    },
  ],
  findings: [],
  work: [],
  fresh: true,
};

/** Merge Train as the editors read it. */
export const MERGE_TRAIN_PAYLOAD = {
  ...primaryAgentFixture,
  name: "merge-train",
  label: "Merge Train",
  resolved_appearance: {
    ...primaryAgentFixture.resolved_appearance,
    version: 1,
    avatar: "generated",
    seed: "merge-train",
  },
};

/** The bell's count: the waiting scripts plus the question already in Needs you. */
export const SCRIPT_BELL_TOTAL = scriptApprovalNoticesFixture.length + 1;
