import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";
import { expect, fn, userEvent, within } from "storybook/test";

import { storybookMswParameters } from "@/storybook/msw";
import { PanelSurface } from "@/storybook/story-layout";
import { useAutomationJob, useAutomationJobRuns } from "@/systems/automation";
import {
  automationRunFixtures,
  primaryAutomationJobFixture,
  primaryAutomationTriggerFixture,
} from "@/systems/automation/mocks";

import { AutomationDetailPanel } from "../automation-detail-panel";

const meta: Meta<typeof AutomationDetailPanel> = {
  title: "systems/automation/components/AutomationDetailPanel",
  component: AutomationDetailPanel,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function AutomationDetailPanelFromPage() {
  const jobId = primaryAutomationJobFixture.id;
  const job = useAutomationJob(jobId);
  const runs = useAutomationJobRuns(jobId, { limit: 10 }, { enabled: Boolean(job.data) });

  return (
    <PanelSurface>
      <AutomationDetailPanel
        error={job.error}
        item={job.data}
        kind="jobs"
        onDelete={fn()}
        onEdit={fn()}
        onToggleEnabled={fn()}
        onTriggerNow={fn()}
        runs={runs.data ?? []}
        runsError={runs.error}
        runsLoading={runs.isLoading}
        state={{
          isDeleting: false,
          isLoading: job.isLoading,
          isTogglePending: false,
          isTriggerPending: false,
        }}
      />
    </PanelSurface>
  );
}

export const Default: Story = {
  args: {},
  render: () => <AutomationDetailPanelFromPage />,
};

export const Error: Story = {
  args: {},
  parameters: {
    ...storybookMswParameters({
      automation: [
        aghApiMock.get("/api/automation/jobs/{id}", ({ params }) =>
          HttpResponse.json(
            { error: `Failed to load automation job ${String(params.id)}` },
            { status: 500 }
          )
        ),
      ],
    }),
  },
  render: () => <AutomationDetailPanelFromPage />,
};

export const TriggerDefault: Story = {
  args: {},
  render: () => (
    <PanelSurface>
      <AutomationDetailPanel
        error={null}
        state={{
          isDeleting: false,
          isLoading: false,
          isTogglePending: false,
          isTriggerPending: false,
        }}
        item={primaryAutomationTriggerFixture}
        kind="triggers"
        onDelete={fn()}
        onEdit={fn()}
        onToggleEnabled={fn()}
        onTriggerNow={fn()}
        runs={automationRunFixtures.filter(
          run => run.trigger_id === primaryAutomationTriggerFixture.id
        )}
        runsError={null}
        runsLoading={false}
      />
    </PanelSurface>
  ),
};

export const PackageManaged: Story = {
  args: {},
  render: () => (
    <PanelSurface>
      <AutomationDetailPanel
        error={null}
        state={{
          isDeleting: false,
          isLoading: false,
          isTogglePending: false,
          isTriggerPending: false,
        }}
        item={{ ...primaryAutomationTriggerFixture, source: "package" }}
        kind="triggers"
        onDelete={fn()}
        onEdit={fn()}
        onToggleEnabled={fn()}
        onTriggerNow={fn()}
        runs={[]}
        runsError={null}
        runsLoading={false}
      />
    </PanelSurface>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(
      canvas.getByText(
        "This automation is provided by an installed package. Only its enabled state can be changed here."
      )
    ).toBeInTheDocument();
  },
};

export const TriggerHook: Story = {
  args: {},
  tags: ["play-fn"],
  render: () => <AutomationDetailPanelFromPage />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.tab();
    await expect(canvas.findByTestId("automation-detail-panel")).resolves.toBeDefined();
  },
};

export const Loading: Story = {
  args: {},
  render: () => (
    <PanelSurface>
      <AutomationDetailPanel
        error={null}
        state={{
          isDeleting: false,
          isLoading: true,
          isTogglePending: false,
          isTriggerPending: false,
        }}
        item={undefined}
        kind="jobs"
        onDelete={fn()}
        onEdit={fn()}
        onToggleEnabled={fn()}
        onTriggerNow={fn()}
        runs={[]}
        runsError={null}
        runsLoading={false}
      />
    </PanelSurface>
  ),
};

export const LoopJob: Story = {
  args: {},
  render: () => (
    <PanelSurface>
      <AutomationDetailPanel
        error={null}
        state={{
          isDeleting: false,
          isLoading: false,
          isTogglePending: false,
          isTriggerPending: false,
        }}
        item={{
          ...primaryAutomationJobFixture,
          agent_name: "",
          prompt: "",
          target_kind: "loop",
          loop_target: {
            workspace_id: primaryAutomationJobFixture.workspace_id ?? "ws_hq",
            loop_name: "software-delivery",
            inputs: { slug: "helix-v1-launch", dry_run: false },
            input_mapping: {},
          },
        }}
        kind="jobs"
        onDelete={fn()}
        onEdit={fn()}
        onToggleEnabled={fn()}
        onTriggerNow={fn()}
        runs={[
          {
            ...automationRunFixtures[0],
            id: "run-f4489762ac431856",
            job_id: primaryAutomationJobFixture.id,
            status: "delegated",
            session_id: undefined,
            loop_run_id: "looprun-aeb24d4f17cf1feb",
          },
        ]}
        runsError={null}
        runsLoading={false}
      />
    </PanelSurface>
  ),
};
