import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn, userEvent, within } from "storybook/test";

import { StorySurface } from "@/storybook/story-layout";

import {
  SESSION_PULL_REQUEST_CHIP_STATES as STATES,
  pullRequestFacts as pr,
} from "../../mocks/session-pull-request-fixtures";
import { SessionPullRequestChip } from "../session-pull-request-chip";

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
