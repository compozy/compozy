import { agentFixtures } from "@/systems/agent/mocks";
import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, within } from "storybook/test";
import { HttpResponse } from "msw";

import {
  storyAgentNames,
  storyWorkspaceIds,
  storyWorkspaceNames,
} from "@/storybook/fintech-scenario";
import { storybookMswParameters } from "@/storybook/msw";
import { aghApiMock } from "@/storybook/openapi-msw";
import { CenteredSurface } from "@/storybook/story-layout";
import type { CreateAutomationJobRequest } from "@/systems/automation";
import { loopCatalogFixtures } from "@/systems/loops/mocks/fixtures";

import { createAutomationJobDraft, setJobOutputMode } from "../../lib/automation-drafts";

import { AutomationJobForm } from "../automation-job-form";

const meta: Meta<typeof AutomationJobForm> = {
  title: "systems/automation/components/AutomationJobForm",
  component: AutomationJobForm,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const storyWorkspaces = [
  { id: storyWorkspaceIds.hq, name: storyWorkspaceNames.hq },
  { id: storyWorkspaceIds.risk, name: storyWorkspaceNames.risk },
  { id: storyWorkspaceIds.growth, name: storyWorkspaceNames.growth },
];

// The target selector reads the real agent catalog shape, not bare names.
const storyAgents = agentFixtures;

const REVIEW_PROMPT =
  "Review the diffs merged in the last 24 hours. Flag risky changes, summarize what shipped, " +
  "and post a launch-room digest with one follow-up per concern.";

function baseDraft(
  overrides: Partial<CreateAutomationJobRequest> = {}
): CreateAutomationJobRequest {
  return {
    ...createAutomationJobDraft(storyWorkspaceIds.hq),
    name: "daily-code-review",
    agent_name: storyAgentNames.release,
    prompt: REVIEW_PROMPT,
    schedule: { mode: "cron", expr: "0 9 * * 1-5" },
    ...overrides,
  };
}

function JobFormHarness({
  initialDraft,
  mode = "create",
  isPending = false,
}: {
  initialDraft: CreateAutomationJobRequest;
  mode?: "create" | "edit";
  isPending?: boolean;
}) {
  const [draft, setDraft] = useState(initialDraft);

  return (
    <CenteredSurface className="items-start justify-center p-6">
      <div className="grid h-(--height-modal-xl) max-h-[90vh] w-full max-w-(--width-modal-xl) grid-cols-[minmax(0,1fr)] grid-rows-[minmax(0,1fr)] overflow-hidden rounded-xl border border-line bg-canvas-soft">
        <AutomationJobForm
          activeWorkspaceId={storyWorkspaceIds.hq}
          agents={storyAgents}
          draft={draft}
          isPending={isPending}
          mode={mode}
          onCancel={() => undefined}
          onChange={setDraft}
          onSubmit={() => undefined}
          workspaces={storyWorkspaces}
        />
      </div>
    </CenteredSurface>
  );
}

export const Default: Story = {
  args: {},
  render: () => <JobFormHarness initialDraft={baseDraft()} />,
};

export const TaskDelegation: Story = {
  args: {},
  render: () => (
    <JobFormHarness
      initialDraft={setJobOutputMode(
        baseDraft({
          name: "nightly-reconcile",
          task: {
            title: "Reconcile settlement ledger",
            description: "Diff the settlement ledger against the gateway and open a task per gap.",
            owner: { kind: "pool", ref: "finance-ops" },
            network_participation: {
              mode: "live",
              channel_strategy: "named",
              channel_id: "release-room",
            },
          },
        }),
        "task"
      )}
    />
  ),
};

export const EveryInterval: Story = {
  args: {},
  render: () => (
    <JobFormHarness
      initialDraft={baseDraft({
        name: "fraud-sweep",
        agent_name: storyAgentNames.fraud,
        prompt: "Sweep open fraud signals and escalate anything above the launch-week threshold.",
        schedule: { mode: "every", interval: "30m" },
      })}
    />
  ),
};

export const RunOnce: Story = {
  args: {},
  render: () => (
    <JobFormHarness
      initialDraft={baseDraft({
        name: "launch-kickoff",
        agent_name: storyAgentNames.compliance,
        prompt: "Post the launch-window go/no-go checklist to the war room.",
        schedule: { mode: "at", time: "2027-01-04T09:00" },
      })}
    />
  ),
};

export const ReliabilityExpanded: Story = {
  args: {},
  render: () => (
    <JobFormHarness
      initialDraft={baseDraft({
        retry: { strategy: "backoff", max_retries: 4, base_delay: "5s" },
        fire_limit: { max: 6, window: "1h" },
        enabled: false,
      })}
    />
  ),
};

/** A recurring Job with a configured catch-up policy and misfire grace, reliability section open. */
export const CatchUpPolicy: Story = {
  args: {},
  render: () => (
    <JobFormHarness
      initialDraft={baseDraft({
        name: "hourly-reconcile",
        agent_name: storyAgentNames.fraud,
        prompt: "Reconcile balances every hour and catch up cleanly after any downtime.",
        schedule: {
          mode: "cron",
          expr: "0 * * * *",
          catch_up_policy: "replay",
          misfire_grace_seconds: 45,
        },
      })}
      mode="edit"
    />
  ),
};

export const WorkspaceLoopTarget: Story = {
  args: {},
  parameters: storybookMswParameters({
    loops: [
      aghApiMock.get("/api/workspaces/{workspace_id}/loops", () => {
        const reviewsWatch = loopCatalogFixtures.find(loop => loop.name === "reviews-watch");
        if (!reviewsWatch) {
          return HttpResponse.json(
            { error: "reviews-watch fixture is unavailable" },
            { status: 500 }
          );
        }

        const loop = {
          ...reviewsWatch,
          inputs: { pr: { type: "number" as const, required: true } },
          start: [{ kind: "schedule" as const }],
        };
        return HttpResponse.json({
          facets: { categories: { watch: 1 }, kinds: {}, statuses: {} },
          loops: [loop],
          page: { has_more: false, limit: 50, total: 1 },
        });
      }),
    ],
  }),
  render: () => (
    <JobFormHarness
      initialDraft={baseDraft({
        name: "reviews-watch-daily",
        agent_name: "",
        prompt: "",
        schedule: { mode: "cron", expr: "0 8 * * 1" },
        target_kind: "loop",
        loop_target: {
          workspace_id: storyWorkspaceIds.hq,
          loop_name: "reviews-watch",
          inputs: { pr: 312 },
          input_mapping: {},
          network_participation: {
            mode: "live",
            channel_strategy: "named",
            channel_id: "release-room",
            bounds: { max_wakes: 3 },
          },
        },
      })}
      mode="edit"
    />
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const participation = await canvas.findByTestId("loop-target-participation");
    participation.scrollIntoView({ block: "center" });
    await expect(canvas.getByTestId("loop-target-participation-mode")).toHaveValue("live");
    await expect(canvas.getByTestId("loop-target-participation-channel")).toHaveValue(
      "release-room"
    );
    await expect(canvas.getByTestId("submit-job-form")).toBeEnabled();
  },
};

/** A saved Job keeps an incompatible Loop visible while blocking the request and save action. */
export const EditIncompatibleLoop: Story = {
  args: {},
  render: () => (
    <JobFormHarness
      initialDraft={baseDraft({
        name: "reviews-watch-daily",
        agent_name: "",
        prompt: "",
        target_kind: "loop",
        loop_target: {
          workspace_id: storyWorkspaceIds.hq,
          loop_name: "reviews-watch",
          inputs: {},
          input_mapping: {},
        },
      })}
      mode="edit"
    />
  ),
};
