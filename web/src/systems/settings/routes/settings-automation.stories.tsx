import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";

import { storybookMswParameters } from "@/storybook/msw";
import { StorybookFieldDirtySetup } from "@/storybook/settings-state-helpers";
import {
  StorybookRestartNoticeSetup,
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";
import { settingsAutomationSectionFixture } from "@/systems/settings/mocks";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/SettingsAutomation",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Automation settings route stories covering the default configuration surface plus loading, error, and restart-required states.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default automation settings page with runtime summary and scheduler controls.
 */
export const Default: Story = {
  args: {},
  parameters: appRouteParameters("/settings/automation"),
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Dirty shell state -- the timezone field has been edited so the save-bar
 * reads Unsaved changes + the Save button enables.
 */
export const Dirty: Story = {
  args: {},
  parameters: appRouteParameters("/settings/automation"),
  render: () => (
    <>
      <StorybookWorkspaceSetup />
      <StorybookFieldDirtySetup
        testId="settings-page-automation-timezone-input"
        value="America/Sao_Paulo"
      />
    </>
  ),
};

/**
 * Restart-required banner after changing automation config that affects the scheduler runtime.
 */
export const RestartNotice: Story = {
  args: {},
  parameters: appRouteParameters("/settings/automation"),
  render: () => (
    <>
      <StorybookWorkspaceSetup />
      <StorybookRestartNoticeSetup section="automation" />
    </>
  ),
};

/**
 * Loading state while the automation section envelope is still fetching.
 */
export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/automation"),
    ...storybookMswParameters({
      settings: [
        aghApiMock.get("/api/settings/automation", async () => {
          await delay("infinite");
          return HttpResponse.json(settingsAutomationSectionFixture);
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Error branch shown when the automation settings request fails.
 */
export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/automation"),
    ...storybookMswParameters({
      settings: [
        aghApiMock.get("/api/settings/automation", () =>
          HttpResponse.json({ error: "Failed to load automation settings" }, { status: 500 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
