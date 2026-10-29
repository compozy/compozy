import { HttpResponse } from "msw";
import { expect, fn, userEvent, within } from "storybook/test";
import { storybookMswParameters } from "@/storybook/msw";
import { compozyApiMock } from "@/storybook/openapi-msw";
import { primarySessionFixture } from "../../../mocks/fixtures";

import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import type { SessionGoalSnapshot } from "@/systems/session/types";

import { SessionGoalStrip } from "../session-goal-strip";
import { SessionGoalHeader } from "../session-goal-header";
import { SessionPrimaryAction } from "../../session-primary-action";

function snapshot(overrides: Partial<SessionGoalSnapshot> = {}): SessionGoalSnapshot {
  return {
    bound_session_id: "session_1",
    cause: null,
    context: {
      nudge_ratio: 0.7,
      ratio: 0.41,
      reported_at: "2026-07-10T12:00:00Z",
      size: 200_000,
      state: "known",
      used: 84_120,
    },
    contract_summary: "All 16 transcript components on session.css tokens; make verify green.",
    last_verdict: {
      blocking_issues: [],
      evaluated_at: "2026-07-10T12:00:00Z",
      evidence_ref: "verdict_012",
      outcome: "approved",
    },
    live: true,
    node_id: "plan-4",
    objective: "Migrate session transcript to the calm-surface vocabulary",
    origin_session_id: "session_1",
    run_id: "run_1",
    run_status: "running",
    status: "active",
    turn_limit: 20,
    turns_used: 3,
    ...overrides,
  };
}

const meta: Meta<typeof SessionGoalStrip> = {
  title: "systems/session/components/SessionGoalStrip",
  component: SessionGoalStrip,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Goal state and controls share one surface above the transcript. Pause interrupts goal work, Resume is explicit, and Stop ends the goal while keeping its history. Expanded details retain the objective, run and context audit.",
      },
    },
  },
  args: { onPause: fn(), onResume: fn(), onApprove: fn(), onStop: fn(), onClear: fn() },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="w-full max-w-2xl">
          <Story />
        </div>
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Active: Story = {
  args: { snapshot: snapshot() },
};

export const Paused: Story = {
  args: { snapshot: snapshot({ status: "paused", run_status: "paused" }) },
};

export const SessionStopped: Story = {
  args: {
    snapshot: snapshot({ status: "paused", run_status: "paused" }),
    continuationDisabledReason: "Send a message to reopen this session, then resume the goal.",
  },
  play: async ({ canvasElement }) => {
    await userEvent.hover(within(canvasElement).getByRole("button", { name: "Resume goal" }));
    await expect(within(document.body).findByRole("tooltip")).resolves.toHaveTextContent(
      "Send a message to reopen this session, then resume the goal."
    );
  },
};

export const Pausing: Story = {
  args: { snapshot: snapshot({ pause_requested: true }) },
};

export const Stopping: Story = {
  args: { snapshot: snapshot(), pendingAction: "stop" },
};

export const Stopped: Story = {
  args: { snapshot: snapshot({ live: false, status: "paused", run_status: "canceled" }) },
};

export const ContextUnavailable: Story = {
  args: {
    snapshot: snapshot({
      context: {
        state: "unknown",
        used: null,
        size: null,
        ratio: null,
        nudge_ratio: 0.7,
        reported_at: null,
        diagnostic: "goal_context_unavailable",
      },
    }),
  },
};

export const StopConfirmation: Story = {
  args: { snapshot: snapshot() },
  play: async ({ canvasElement }) => {
    await userEvent.click(within(canvasElement).getByRole("button", { name: "Stop goal" }));
    await expect(
      within(document.body).findByRole("dialog", { name: "Stop this goal?" })
    ).resolves.toBeVisible();
  },
};

export const SessionStopConfirmation: Story = {
  args: { snapshot: snapshot() },
  render: args => (
    <>
      <div className="flex justify-end border-b border-line pb-2">
        <SessionPrimaryAction
          showStopAction
          hasLiveGoal
          showUnarchiveAction={false}
          controlsBusy={false}
          isStopping={false}
          isUnarchiving={false}
          onStop={fn()}
          onUnarchive={fn()}
        />
      </div>
      <SessionGoalStrip {...args} />
    </>
  ),
  play: async ({ canvasElement }) => {
    await userEvent.click(within(canvasElement).getByRole("button", { name: "Stop session" }));
    await expect(
      within(document.body).findByRole("dialog", { name: "Stop this session?" })
    ).resolves.toBeVisible();
  },
};

export const ControlFailed: Story = {
  args: { snapshot: snapshot() },
  render: args => (
    <SessionGoalHeader
      {...args}
      actionError={new Error("Couldn't reach the session. Try again.")}
    />
  ),
};

export const RefreshFailed: Story = {
  args: { snapshot: snapshot() },
  render: args => (
    <SessionGoalHeader
      {...args}
      error={new Error("The connection was interrupted.")}
      onRefresh={fn()}
    />
  ),
};

export const Blocked: Story = {
  args: {
    snapshot: snapshot({ status: "blocked", run_status: "blocked", cause: "goal_agent_refused" }),
  },
};

export const Done: Story = {
  args: { snapshot: snapshot({ live: false, status: "complete", run_status: "done" }) },
};

export const Moved: Story = {
  args: {
    snapshot: snapshot({ bound_session_id: "session_2", origin_session_id: "session_1" }),
  },
};

export const WithDraftAction: Story = {
  args: {
    snapshot: snapshot(),
    composerAffordance: { kind: "draft", expandedObjective: "Ship the calm transcript" },
    onPrefillComposer: () => {},
  },
};

export const VerificationAudit: Story = {
  args: { snapshot: snapshot({ status: "complete", run_status: "done", live: false }) },
  parameters: storybookMswParameters({
    session: [
      compozyApiMock.get("/api/sessions", ({ request }) => {
        const query = new URL(request.url).searchParams;
        const sessions =
          query.get("type") === "system" && query.get("parent") === "session_1"
            ? [
                {
                  ...primarySessionFixture,
                  id: "sess-judge",
                  name: "loop gate general objective_satisfied",
                  type: "system" as const,
                  state: "stopped" as const,
                  badge: "stopped" as const,
                  stop_reason: "completed" as const,
                },
              ]
            : [];
        return HttpResponse.json({
          sessions,
          page: { total: sessions.length, limit: 50, has_more: false },
        });
      }),
    ],
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByTestId("goal-strip-line"));
    await userEvent.click(canvas.getByRole("button", { name: "Verification sessions" }));
    await expect(
      canvas.findByRole("link", { name: /loop gate general objective_satisfied/ })
    ).resolves.toBeVisible();
  },
};
