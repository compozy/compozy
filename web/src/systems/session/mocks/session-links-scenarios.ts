/** The states the linked work prototypes walk through. */
import type { SessionLinkedWorkItem, SessionRunsIn } from "../lib/session-linked-work";
import { isRolledUp, linkKey } from "../lib/session-linked-work";
import {
  SESSION_LINK_SETS,
  SESSION_LINKS_DOZEN,
  SESSION_LINKS_FIFTY,
  sessionLinkFixtureKey,
} from "./session-links-fixtures";

export interface LinkScenario {
  sessionId: "s1" | "s4";
  items: readonly SessionLinkedWorkItem[];
  runsIn: SessionRunsIn;
  /** The list is showing, not at rest. */
  open: boolean;
  /** The row whose menu is pinned open, by `linkKey`. */
  openMenuFor?: string;
  /** The row whose details are pinned open, by `linkKey`. */
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

const FOUR_PRS = "Four pull requests from this session: one is merged, one closed, two still open.";
const NO_FORGE = "Both branches are pushed. CompozyOS can't reach GitHub right now.";
const CLEANED_UP =
  "#931 is ready to merge. I cleaned up its worktree, so only the pull request is left.";

const SCENARIOS: Record<string, LinkScenario> = {
  none: {
    ...SINGLE,
    items: SESSION_LINK_SETS.none,
    user: "Why does the session list jump when a new message arrives?",
    agent: "It re-sorts on every message. I can show you where, if you want to look first.",
  },
  "one-pr": { ...SINGLE, items: SESSION_LINK_SETS.onePr },
  "mixed-rest": { ...SINGLE, items: SESSION_LINK_SETS.mixed, open: false, agent: FOUR_PRS },
  "mixed-open": { ...SINGLE, items: SESSION_LINK_SETS.mixed, agent: FOUR_PRS },
  "worktree-no-pr": {
    ...SINGLE,
    items: SESSION_LINK_SETS.worktreeNoPr,
    agent: "I made a worktree for this and started there. No pull request yet.",
  },
  "worktree-removed": {
    ...SINGLE,
    items: SESSION_LINK_SETS.worktreeRemoved,
    agent: CLEANED_UP,
  },
  "lookup-failed": { ...SINGLE, items: SESSION_LINK_SETS.lookupFailed, agent: NO_FORGE },
  "added-by": {
    ...SINGLE,
    items: SESSION_LINK_SETS.addedBy,
    agent: "I linked the migration guide here too, since this change depends on it.",
  },
  "runs-here": {
    ...SINGLE,
    items: SESSION_LINK_SETS.runsHere,
    runsIn: { kind: "worktree", name: "session-pr-chip" },
    agent: "This session now runs in session-pr-chip. #948 is linked, but it lives elsewhere.",
  },
  unlink: {
    ...SINGLE,
    items: SESSION_LINK_SETS.mixed,
    openMenuFor: sessionLinkFixtureKey("feat/forge-retry"),
    agent: FOUR_PRS,
  },
  "rolled-up": {
    ...ORCHESTRATOR,
    items: SESSION_LINK_SETS.rolledUp,
    openMenuFor: sessionLinkFixtureKey("fix/login-redirect"),
    agent: "Two child sessions are working on this. Their pull requests show up here too.",
  },
  "rolled-up-single": {
    ...ORCHESTRATOR,
    items: SESSION_LINK_SETS.rolledUp.filter(isRolledUp).slice(0, 1),
    agent: "A child session is working on this. Its pull request shows up here too.",
  },
  "details-failing": {
    ...SINGLE,
    items: SESSION_LINK_SETS.mixed,
    openDetailFor: sessionLinkFixtureKey("fix/login-redirect"),
    agent: FOUR_PRS,
  },
  "details-pending": {
    ...SINGLE,
    items: SESSION_LINK_SETS.mixed,
    openDetailFor: sessionLinkFixtureKey("feat/forge-retry"),
    agent: FOUR_PRS,
  },
  "details-ready": {
    ...SINGLE,
    items: SESSION_LINK_SETS.worktreeRemoved,
    openDetailFor: linkKey(SESSION_LINK_SETS.worktreeRemoved[0]!),
    agent: CLEANED_UP,
  },
  "details-stale": {
    ...SINGLE,
    items: SESSION_LINK_SETS.lookupFailed,
    openDetailFor: linkKey(SESSION_LINK_SETS.lookupFailed[1]!),
    agent: NO_FORGE,
  },
  "details-merged": {
    ...SINGLE,
    items: SESSION_LINK_SETS.mixed,
    openDetailFor: sessionLinkFixtureKey("refactor/session-cache"),
    agent: FOUR_PRS,
  },
  "details-branch": {
    ...SINGLE,
    items: SESSION_LINK_SETS.lookupFailed,
    openDetailFor: linkKey(SESSION_LINK_SETS.lookupFailed[0]!),
    agent: NO_FORGE,
  },
  orchestrator: { ...ORCHESTRATOR, items: SESSION_LINKS_DOZEN },
  fifty: {
    ...ORCHESTRATOR,
    items: SESSION_LINKS_FIFTY,
    agent: "Fifty pull requests so far across the whole migration.",
  },
  narrow: { ...ORCHESTRATOR, items: SESSION_LINKS_DOZEN, narrow: true },
};

export function linkScenario(state: string): LinkScenario {
  return SCENARIOS[state] ?? SCENARIOS["mixed-rest"]!;
}
