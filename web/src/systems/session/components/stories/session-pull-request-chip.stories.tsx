import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn, userEvent, within } from "storybook/test";

import { StorySurface } from "@/storybook/story-layout";

import type {
  SessionPullRequest,
  SessionPullRequestFacts,
  SessionPullRequestLookup,
} from "../../lib/session-pull-request";
import { SessionPullRequestChip } from "../session-pull-request-chip";

const OK: SessionPullRequestLookup = {
  state: "ok",
  cause: "",
  observed_at: "2026-10-23T18:01:40Z",
  stale: false,
};
const OPEN: SessionPullRequest = {
  provider: "github",
  number: 931,
  url: "https://github.com/compozy/compozy/pull/931",
  state: "open",
  draft: false,
  head_sha: "c1d9e07b2a",
  checks: { state: "passing", passed: 14, failed: 0, pending: 0, failed_names: [] },
  review_decision: "approved",
  mergeable: "mergeable",
  ready_to_merge: false,
};
const BRANCH = "feat/session-pr-chip";

const branch = (lookup: SessionPullRequestLookup): SessionPullRequestFacts => ({
  branch: BRANCH,
  pull_request: null,
  pull_request_lookup: lookup,
});
const pr = (
  patch: Partial<SessionPullRequest>,
  lookup: SessionPullRequestLookup = OK
): SessionPullRequestFacts => ({
  branch: BRANCH,
  pull_request: { ...OPEN, ...patch },
  pull_request_lookup: lookup,
});

/** Board §S1 VC-01…VC-14, in board order. */
const STATES: {
  vc: string;
  label: string;
  facts: SessionPullRequestFacts;
  refreshing?: boolean;
}[] = [
  { vc: "VC-02", label: "Lookup pending", facts: branch({ ...OK, state: "pending", stale: true }) },
  { vc: "VC-03", label: "Branch, no PR", facts: branch(OK) },
  {
    vc: "VC-04",
    label: "Unavailable",
    facts: branch({ state: "error", cause: "credential_missing", observed_at: null, stale: true }),
  },
  {
    vc: "VC-05",
    label: "Incomplete",
    facts: branch({
      state: "incomplete",
      cause: "selection_incomplete",
      observed_at: null,
      stale: true,
    }),
  },
  {
    vc: "VC-06",
    label: "Checks pending",
    facts: pr({
      checks: { state: "pending", passed: 11, failed: 0, pending: 3, failed_names: [] },
      review_decision: "review_required",
    }),
  },
  {
    vc: "VC-07",
    label: "Checks failing",
    facts: pr({
      number: 932,
      checks: {
        state: "failing",
        passed: 12,
        failed: 2,
        pending: 0,
        failed_names: ["lint", "test-web"],
      },
      review_decision: "review_required",
    }),
  },
  { vc: "VC-08", label: "Changes requested", facts: pr({ review_decision: "changes_requested" }) },
  { vc: "VC-09", label: "Draft", facts: pr({ draft: true, review_decision: "review_required" }) },
  { vc: "VC-10", label: "Ready to merge", facts: pr({ ready_to_merge: true }) },
  { vc: "VC-11", label: "Conflicts", facts: pr({ mergeable: "conflicting" }) },
  { vc: "VC-12", label: "Merged", facts: pr({ state: "merged" }) },
  { vc: "VC-12", label: "Closed", facts: pr({ state: "closed" }) },
  {
    vc: "VC-06",
    label: "No checks",
    facts: pr({ checks: { state: "none", passed: 0, failed: 0, pending: 0, failed_names: [] } }),
  },
  {
    vc: "VC-13",
    label: "Stale facts",
    facts: pr(
      { ready_to_merge: true },
      { ...OK, state: "error", cause: "rate_limited", stale: true }
    ),
  },
  { vc: "VC-14", label: "Refreshing", facts: pr({}), refreshing: true },
];

function ChipGallery() {
  return (
    <StorySurface className="grid grid-cols-[8rem_auto] items-center gap-x-6 gap-y-3 p-8">
      {STATES.map(state => (
        <div className="contents" key={`${state.vc}-${state.label}`}>
          <span className="text-eyebrow text-subtle">
            {state.vc} · {state.label}
          </span>
          <span className="flex">
            <SessionPullRequestChip
              controllable
              facts={state.facts}
              onPrompt={fn()}
              onRefresh={fn()}
              refreshing={state.refreshing ?? false}
            />
          </span>
        </div>
      ))}
    </StorySurface>
  );
}

const meta: Meta<typeof SessionPullRequestChip> = {
  title: "systems/session/components/SessionPullRequestChip",
  component: SessionPullRequestChip,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "The session's branch or pull request in the window topbar status zone, with the next delivery action first in its menu. No branch renders nothing; unavailable actions are absent.",
      },
    },
  },
  args: { controllable: true, onPrompt: fn(), onRefresh: fn(), onExitAction: fn() },
  render: args => (
    <StorySurface className="flex h-80 w-96 items-start justify-start p-6">
      <SessionPullRequestChip {...args} />
    </StorySurface>
  ),
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-02…VC-14 — every branch and PR chip state. */
export const States: Story = {
  args: { facts: pr({}) },
  render: () => <ChipGallery />,
};

const openMenu: Story["play"] = async ({ canvasElement }) => {
  await userEvent.click(within(canvasElement).getByTestId("session-pull-request-chip"));
};

/** VC-15 — a managed worktree leads with the exit plan's action. */
export const MenuExitFirst: Story = {
  args: {
    facts: pr({ draft: true, review_decision: "review_required" }),
    exitPlan: {
      primary: { action: "commit_push", label: "Commit & push", enabled: true },
      blocked: false,
    },
  },
  play: openMenu,
};

/** VC-16 — failing checks lead with the fix prompt. */
export const MenuFixFirst: Story = {
  args: {
    facts: pr({
      number: 932,
      checks: {
        state: "failing",
        passed: 12,
        failed: 2,
        pending: 0,
        failed_names: ["lint", "test-web"],
      },
      review_decision: "changes_requested",
    }),
  },
  play: openMenu,
};

/** VC-17 — a non-controllable session: Open PR and Refresh only. */
export const MenuReadOnly: Story = {
  args: { ...MenuFixFirst.args, controllable: false },
  play: openMenu,
};

/** VC-18 — the exit action runs: spinner and progressive label. */
export const MenuExitRunning: Story = {
  args: { ...MenuExitFirst.args, exitRunning: true },
  play: openMenu,
};
