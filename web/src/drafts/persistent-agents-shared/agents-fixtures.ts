import type { ListedAgent } from "./agents-page";
import {
  CODE_ENGINEER,
  EPHEMERAL_AGENTS,
  GENERAL,
  ORCHESTRATOR,
  RELEASE,
  REVIEWER,
  SCOUT,
  TRIAGE,
} from "./fixtures";

export const LISTED_PERSISTENT: readonly ListedAgent[] = [
  {
    ...ORCHESTRATOR,
    places: [
      { scope: "compozy", state: "working" },
      { scope: "reloaddev", state: "idle" },
    ],
  },
  {
    ...TRIAGE,
    places: [
      { scope: "compozy", state: "idle" },
      { scope: "advos", state: "idle" },
    ],
  },
  { ...RELEASE, places: [{ scope: "@user", state: "idle" }] },
  { ...SCOUT, places: [] },
];

export const LISTED_EPHEMERAL: readonly ListedAgent[] = EPHEMERAL_AGENTS;

export const LISTED_PAUSED: readonly ListedAgent[] = [
  LISTED_PERSISTENT[0]!,
  {
    ...TRIAGE,
    places: [
      { scope: "compozy", state: "paused" },
      { scope: "advos", state: "paused" },
    ],
  },
  { ...RELEASE, places: [{ scope: "@user", state: "failed" }] },
];

export const LISTED_NEEDS_YOU: readonly ListedAgent[] = [
  {
    ...TRIAGE,
    places: [
      { scope: "compozy", state: "needs-you" },
      { scope: "advos", state: "idle" },
    ],
  },
  LISTED_PERSISTENT[0]!,
  LISTED_PERSISTENT[2]!,
];

export const LISTED_SAME_SLUG: readonly ListedAgent[] = [
  {
    ...GENERAL,
    origin: "compozy",
    scopes: ["compozy"],
    active: 2,
  },
  {
    ...GENERAL,
    description: "Answers questions about the reloaddev deploy pipeline",
    origin: "reloaddev",
    provider: "codex",
    scopes: ["reloaddev"],
    active: 0,
  },
  CODE_ENGINEER,
  REVIEWER,
];

export const LISTED_INVALID: readonly ListedAgent[] = [
  CODE_ENGINEER,
  { ...REVIEWER, invalid: true, description: "AGENT.md has an unknown kind value" },
  GENERAL,
];

const NAMES = [
  "Migration Pilot",
  "Docs Keeper",
  "Bug Hunter",
  "Test Writer",
  "Spec Author",
  "Issue Sorter",
  "Schema Guard",
  "Copy Editor",
  "Pipeline Fixer",
];

/** One page of a 500-agent catalog: the groups keep their server totals past the cut. */
export const LISTED_MANY: readonly ListedAgent[] = NAMES.map((label, index) => {
  const name = label.toLowerCase().replace(/\s/g, "-");
  return {
    ...CODE_ENGINEER,
    name,
    label,
    appearance: { avatar: "generated", seed: name },
    provider: index % 2 === 0 ? "claude" : "codex",
    description: "Runs on demand across your projects",
  };
});
