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
import { settingsNetworkSectionFixture } from "@/systems/settings/mocks";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/SettingsNetwork",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Network settings route stories covering the runtime summary, restart notice, and request boundary states.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default network settings page with truthful availability and finite Live bounds.
 */
export const Default: Story = {
  args: {},
  parameters: appRouteParameters("/settings/network"),
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Dirty shell state -- the Live wake default has been edited so the save-bar reads
 * Unsaved changes + the Save button enables.
 */
export const Dirty: Story = {
  args: {},
  parameters: appRouteParameters("/settings/network"),
  render: () => (
    <>
      <StorybookWorkspaceSetup />
      <StorybookFieldDirtySetup testId="settings-page-network-live-default-max-wakes" value="12" />
    </>
  ),
};

/**
 * Restart-required banner after a Network settings update requires daemon reconciliation.
 */
export const RestartNotice: Story = {
  args: {},
  parameters: appRouteParameters("/settings/network"),
  render: () => (
    <>
      <StorybookWorkspaceSetup />
      <StorybookRestartNoticeSetup section="network" />
    </>
  ),
};

/**
 * Loading state while the network section is still resolving.
 */
export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/network"),
    ...storybookMswParameters({
      settings: [
        aghApiMock.get("/api/settings/network", async () => {
          await delay("infinite");
          return HttpResponse.json(settingsNetworkSectionFixture);
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Error branch shown when the network settings request fails.
 */
export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/network"),
    ...storybookMswParameters({
      settings: [
        aghApiMock.get("/api/settings/network", () =>
          HttpResponse.json({ error: "Failed to load network settings" }, { status: 500 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
