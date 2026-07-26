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
import { createAutomationTriggerDraft } from "@/systems/automation";
import type { CreateAutomationTriggerRequest } from "@/systems/automation";
import { loopCatalogFixtures } from "@/systems/loops/mocks/fixtures";

import { AutomationRequestPayload } from "../automation-request-payload";
import { AutomationTriggerForm } from "../automation-trigger-form";

const meta: Meta<typeof AutomationTriggerForm> = {
  title: "systems/automation/components/AutomationTriggerForm",
  component: AutomationTriggerForm,
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

const STOP_PROMPT =
  "Session {{ .Data.session_id }} (agent {{ .Data.agent_name }}) stopped with reason " +
  '"{{ .Data.stop_reason }}".\n\nRead the transcript, summarize what went wrong, the likely ' +
  "root cause, and one suggested next step.";

function baseDraft(
  overrides: Partial<CreateAutomationTriggerRequest> = {}
): CreateAutomationTriggerRequest {
  return {
    ...createAutomationTriggerDraft(storyWorkspaceIds.hq),
    event: "session.stopped",
    scope: "workspace",
    workspace_id: storyWorkspaceIds.hq,
    name: "summarize-failures",
    agent_name: storyAgentNames.compliance,
    filter: { "data.stop_reason": "error" },
    prompt: STOP_PROMPT,
    ...overrides,
  };
}

function TriggerFormHarness({
  initialDraft,
  mode = "create",
  isPending = false,
}: {
  initialDraft: CreateAutomationTriggerRequest;
  mode?: "create" | "edit";
  isPending?: boolean;
}) {
  const [draft, setDraft] = useState(initialDraft);

  return (
    <CenteredSurface className="items-start justify-center p-6">
      <div className="grid h-(--height-modal-xl) max-h-[90vh] w-full max-w-(--width-modal-xl) grid-cols-[minmax(0,1fr)] grid-rows-[minmax(0,1fr)] overflow-hidden rounded-xl border border-line bg-canvas-soft">
        <AutomationTriggerForm
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
  render: () => <TriggerFormHarness initialDraft={baseDraft()} />,
};

export const WebhookSelected: Story = {
  args: {},
  render: () => (
    <TriggerFormHarness
      initialDraft={baseDraft({
        name: "deploy-watch",
        event: "webhook",
        scope: "global",
        workspace_id: undefined,
        filter: {},
        endpoint_slug: "ci-deploys",
        webhook_id: "wbh_a1b2c3",
        webhook_secret_value: "whsec_live_8f3a9c2e",
        agent_name: storyAgentNames.release,
        prompt: "A deploy webhook arrived: {{ .Data.payload }}. Post a launch-room status update.",
      })}
    />
  ),
};

export const WebhookRequestRedacted: Story = {
  render: () => (
    <CenteredSurface className="p-8">
      <div className="w-full max-w-3xl">
        <AutomationRequestPayload
          request={{
            method: "POST",
            path: "/api/automation/triggers",
            payload: baseDraft({
              event: "webhook",
              scope: "global",
              workspace_id: undefined,
              endpoint_slug: "ci-deploys",
              webhook_id: "wbh_a1b2c3",
              webhook_secret_value: "whsec_live_8f3a9c2e",
            }),
          }}
        />
      </div>
    </CenteredSurface>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByTestId("automation-request-payload")).toHaveTextContent("[redacted]");
    await expect(canvas.queryByText("whsec_live_8f3a9c2e")).not.toBeInTheDocument();
  },
};

export const ExtensionEvent: Story = {
  args: {},
  render: () => (
    <TriggerFormHarness
      initialDraft={baseDraft({
        name: "release-deploys",
        event: "ext.release.deploy-started",
        filter: { "data.status": "started" },
        agent_name: storyAgentNames.release,
        prompt: "Release {{ .Data.version }} started deploying to {{ .Data.repo }}.",
      })}
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
          start: [{ kind: "trigger" as const }],
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
    <TriggerFormHarness
      initialDraft={baseDraft({
        agent_name: "",
        filter: {},
        loop_target: {
          input_mapping: {},
          inputs: { pr: 2 },
          loop_name: "reviews-watch",
          network_participation: {
            mode: "live",
            channel_strategy: "named",
            channel_id: "release-room",
            bounds: { max_wakes: 3 },
          },
          workspace_id: storyWorkspaceIds.hq,
        },
        name: "review-on-stop",
        prompt: "",
        target_kind: "loop",
      })}
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
    await expect(canvas.getByTestId("submit-trigger-form")).toBeEnabled();
  },
};

export const ReliabilityExpanded: Story = {
  args: {},
  render: () => (
    <TriggerFormHarness
      initialDraft={baseDraft({
        retry: { strategy: "backoff", max_retries: 2, base_delay: "5s" },
        fire_limit: { max: 4, window: "1h" },
        enabled: false,
      })}
    />
  ),
};

export const EditMode: Story = {
  args: {},
  render: () => (
    <TriggerFormHarness
      initialDraft={baseDraft({
        name: "hook-failures",
        event: "hook.transform.completed",
        filter: { "data.hook_outcome": "failed" },
        agent_name: storyAgentNames.support,
        prompt:
          "Hook {{ .Data.hook_name }} failed: {{ .Data.error }}. Investigate and report back.",
      })}
      mode="edit"
    />
  ),
};

/** A saved Trigger keeps a schedule-only Loop visible while blocking trigger dispatch. */
export const EditIncompatibleLoop: Story = {
  args: {},
  render: () => (
    <TriggerFormHarness
      initialDraft={baseDraft({
        name: "delivery-on-stop",
        agent_name: "",
        prompt: "",
        target_kind: "loop",
        loop_target: {
          workspace_id: storyWorkspaceIds.hq,
          loop_name: "software-delivery",
          inputs: { goal: "Ship the queued delivery" },
          input_mapping: {},
        },
      })}
      mode="edit"
    />
  ),
  play: async ({ canvasElement }) => {
    const targetFields = await within(canvasElement).findByTestId("loop-target-fields");
    targetFields.scrollIntoView({ block: "center" });
    await expect(targetFields).toBeVisible();
  },
};
