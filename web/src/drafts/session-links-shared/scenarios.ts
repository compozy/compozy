/** The states every direction walks through, so the three compare one to one. */
import {
  SESSION_LINK_SETS,
  SESSION_LINKS_DOZEN,
  SESSION_LINKS_FIFTY,
  type SessionLink,
  type SessionRunsIn,
} from "@/systems/session/mocks/session-links-fixtures";

export interface LinkScenario {
  sessionId: "s1" | "s4";
  links: readonly SessionLink[];
  runsIn: SessionRunsIn;
  /** The list is showing, not at rest. */
  open: boolean;
  /** The row whose menu is pinned open. */
  openMenuFor?: string;
  /** The row whose details are pinned open. */
  openDetailFor?: string;
  narrow?: boolean;
  user: string;
  agent: string;
}

const PROJECT: SessionRunsIn = { kind: "project", name: "compozy" };

const SINGLE = {
  sessionId: "s4",
  runsIn: PROJECT,
  open: true,
  user: "Show the pull request in the session header, and open it when the checks pass.",
  agent:
    "I made a separate worktree for this so the main project stays clean, and opened a draft pull request from there.",
} as const;

const ORCHESTRATOR = {
  sessionId: "s1",
  runsIn: PROJECT,
  open: true,
  user: "Ship everything on the Sessions Kanban list today.",
  agent:
    "Each item has its own worktree and pull request. Two need you: #951 has failing checks, and #957 has changes requested.",
} as const;

const SCENARIOS: Record<string, LinkScenario> = {
  none: {
    ...SINGLE,
    links: SESSION_LINK_SETS.none,
    user: "Why does the session list jump when a new message arrives?",
    agent: "It re-sorts on every message. I can show you where, if you want to look first.",
  },
  "one-pr": { ...SINGLE, links: SESSION_LINK_SETS.onePr },
  "mixed-rest": {
    ...SINGLE,
    links: SESSION_LINK_SETS.mixed,
    open: false,
    agent: "Four pull requests from this session: one is merged, one closed, two still open.",
  },
  "mixed-open": {
    ...SINGLE,
    links: SESSION_LINK_SETS.mixed,
    agent: "Four pull requests from this session: one is merged, one closed, two still open.",
  },
  "worktree-no-pr": {
    ...SINGLE,
    links: SESSION_LINK_SETS.worktreeNoPr,
    agent: "I made a worktree for this and started there. No pull request yet.",
  },
  "worktree-removed": {
    ...SINGLE,
    links: SESSION_LINK_SETS.worktreeRemoved,
    agent: "#931 is ready to merge. I cleaned up its worktree, so only the pull request is left.",
  },
  "lookup-failed": {
    ...SINGLE,
    links: SESSION_LINK_SETS.lookupFailed,
    agent: "Both branches are pushed. CompozyOS can't reach GitHub right now.",
  },
  "added-by": {
    ...SINGLE,
    links: SESSION_LINK_SETS.addedBy,
    agent: "I linked the migration guide here too, since this change depends on it.",
  },
  "runs-here": {
    ...SINGLE,
    links: SESSION_LINK_SETS.runsHere,
    runsIn: { kind: "worktree", name: "session-pr-chip" },
    agent: "This session now runs in session-pr-chip. #948 is linked, but it lives elsewhere.",
  },
  unlink: {
    ...SINGLE,
    links: SESSION_LINK_SETS.mixed,
    openMenuFor: "pr-948",
    agent: "Four pull requests from this session: one is merged, one closed, two still open.",
  },
  "details-failing": {
    ...SINGLE,
    links: SESSION_LINK_SETS.mixed,
    openDetailFor: "pr-951",
    agent: "Four pull requests from this session: one is merged, one closed, two still open.",
  },
  "details-pending": {
    ...SINGLE,
    links: SESSION_LINK_SETS.mixed,
    openDetailFor: "pr-948",
    agent: "Four pull requests from this session: one is merged, one closed, two still open.",
  },
  "details-ready": {
    ...SINGLE,
    links: SESSION_LINK_SETS.worktreeRemoved,
    openDetailFor: "pr-931",
    agent: "#931 is ready to merge. I cleaned up its worktree, so only the pull request is left.",
  },
  "details-stale": {
    ...SINGLE,
    links: SESSION_LINK_SETS.lookupFailed,
    openDetailFor: "pr-942",
    agent: "Both branches are pushed. CompozyOS can't reach GitHub right now.",
  },
  "details-merged": {
    ...SINGLE,
    links: SESSION_LINK_SETS.mixed,
    openDetailFor: "pr-939",
    agent: "Four pull requests from this session: one is merged, one closed, two still open.",
  },
  "details-branch": {
    ...SINGLE,
    links: SESSION_LINK_SETS.lookupFailed,
    openDetailFor: "branch-fix/attachment-rebase",
    agent: "Both branches are pushed. CompozyOS can't reach GitHub right now.",
  },
  orchestrator: { ...ORCHESTRATOR, links: SESSION_LINKS_DOZEN },
  fifty: {
    ...ORCHESTRATOR,
    links: SESSION_LINKS_FIFTY,
    agent: "Fifty pull requests so far across the whole migration.",
  },
  narrow: { ...ORCHESTRATOR, links: SESSION_LINKS_DOZEN, narrow: true },
};

export function linkScenario(state: string): LinkScenario {
  return SCENARIOS[state] ?? SCENARIOS["mixed-rest"]!;
}
