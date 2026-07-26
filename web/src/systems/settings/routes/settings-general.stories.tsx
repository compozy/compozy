import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";

import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookGeneralDraftDirtySetup,
  StorybookGeneralSavingSetup,
  StorybookRestartPhaseSetup,
} from "@/storybook/settings-state-helpers";
import {
  StorybookRestartNoticeSetup,
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";
import {
  settingsAppliedMutationFixture,
  settingsGeneralSectionFixture,
  settingsRestartStatusFixture,
} from "@/systems/settings/mocks";
import { toolApprovalGrantsResponseFixture } from "@/systems/tool-approvals/mocks";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/SettingsGeneral",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "General settings route stories rendered through the real app shell, including loading, error, dirty, saving, and all restart notice tones.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default general settings page with runtime status and editable defaults.
 * Represents the idle shell state for the route story.
 */
export const Default: Story = {
  args: {},
  parameters: appRouteParameters("/settings/general"),
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Runtime memory reporting disabled -- the daemon interval is 0s, so the general
 * page renders the disabled sampling value for the field.
 */
export const MemoryReportingDisabled: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/general"),
    ...storybookMswParameters({
      settings: [
        aghApiMock.get("/api/settings/general", () =>
          HttpResponse.json({
            ...settingsGeneralSectionFixture,
            config: {
              ...settingsGeneralSectionFixture.config,
              daemon: {
                ...settingsGeneralSectionFixture.config.daemon,
                memory_report_interval: "0s",
              },
            },
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Initial loading state while the section envelope is still resolving.
 */
export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/general"),
    ...storybookMswParameters({
      settings: [
        aghApiMock.get("/api/settings/general", async () => {
          await delay("infinite");
          return HttpResponse.json(settingsGeneralSectionFixture);
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Error branch shown when the general settings request fails.
 */
export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/general"),
    ...storybookMswParameters({
      settings: [
        aghApiMock.get("/api/settings/general", () =>
          HttpResponse.json({ error: "Failed to load general settings" }, { status: 500 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Dirty shell state -- the default-agent field has been edited so the save-bar
 * reads Unsaved changes + the Save button enables.
 */
export const Dirty: Story = {
  args: {},
  parameters: appRouteParameters("/settings/general"),
  render: () => (
    <>
      <StorybookWorkspaceSetup />
      <StorybookGeneralDraftDirtySetup />
    </>
  ),
};

/**
 * Saving shell state -- the PATCH endpoint hangs so the Save button shows the
 * spinner + Saving... label for the story.
 */
export const Saving: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/general"),
    ...storybookMswParameters({
      settings: [
        aghApiMock.patch("/api/settings/general", async () => {
          await delay("infinite");
          return HttpResponse.json(settingsAppliedMutationFixture);
        }),
      ],
    }),
  },
  render: () => (
    <>
      <StorybookWorkspaceSetup />
      <StorybookGeneralSavingSetup />
    </>
  ),
};

/**
 * Restart-warning notice -- mutation recorded as restart-required.
 */
export const RestartWarning: Story = {
  args: {},
  parameters: appRouteParameters("/settings/general"),
  render: () => (
    <>
      <StorybookWorkspaceSetup />
      <StorybookRestartNoticeSetup section="general" />
    </>
  ),
};

/**
 * Restart-polling notice -- operation started, status still pending, spinner
 * visible in the notice.
 */
export const RestartPolling: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/general"),
    ...storybookMswParameters({
      settings: [
        aghApiMock.get("/api/settings/actions/restart/{operation_id}", async () => {
          await delay("infinite");
          return HttpResponse.json({
            ...settingsRestartStatusFixture,
            operation_id: "op_polling",
            status: "pending",
            active_session_count: 2,
          });
        }),
      ],
    }),
  },
  render: () => (
    <>
      <StorybookWorkspaceSetup />
      <StorybookRestartPhaseSetup
        section="general"
        overrides={{
          mutationRestartRequired: true,
          operationId: "op_polling",
          status: "pending",
          activeSessionCount: 2,
        }}
      />
    </>
  ),
};

/**
 * Apply-history state -- config apply records include applied, blocked, and
 * failed rows with next-action and diagnostics surfaced in the general route.
 */
export const ApplyHistory: Story = {
  args: {},
  parameters: appRouteParameters("/settings/general"),
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Restart-success notice -- operation completed, Dismiss button visible.
 */
export const RestartSuccess: Story = {
  args: {},
  parameters: appRouteParameters("/settings/general"),
  render: () => (
    <>
      <StorybookWorkspaceSetup />
      <StorybookRestartPhaseSetup
        section="general"
        overrides={{
          mutationRestartRequired: true,
          operationId: "op_success",
          status: "ready",
        }}
      />
    </>
  ),
};

/**
 * Restart-failure notice -- operation failed with a reason suffix + Dismiss.
 */
export const RestartFailure: Story = {
  args: {},
  parameters: appRouteParameters("/settings/general"),
  render: () => (
    <>
      <StorybookWorkspaceSetup />
      <StorybookRestartPhaseSetup
        section="general"
        overrides={{
          mutationRestartRequired: true,
          operationId: "op_failure",
          status: "failed",
          failureReason: "helper exited non-zero",
        }}
      />
    </>
  ),
};

/**
 * Remembered decisions in context: the Permissions area shows the active workspace's
 * remembered native-tool approval decisions (allow + reject exact rows) inside the real
 * general settings page, proving the section's placement.
 */
export const RememberedDecisions: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/general"),
    ...storybookMswParameters({
      "tool-approvals": [
        aghApiMock.get("/api/tool-approval-grants", () =>
          HttpResponse.json(toolApprovalGrantsResponseFixture)
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
