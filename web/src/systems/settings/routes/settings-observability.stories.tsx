import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";

import { storybookMswParameters } from "@/storybook/msw";
import { StorybookFieldDirtySetup } from "@/storybook/settings-state-helpers";
import { settingsObservabilitySectionFixture } from "@/systems/settings/mocks";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/SettingsObservability",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Observability settings route stories covering storage metrics, log-tail capability variations, and request boundary states.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default observability page with usage metrics and live log-tail metadata.
 */
export const Default: Story = {
  args: {},
  parameters: appRouteParameters("/settings/observability"),
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Dirty shell state -- the retention-days input has been edited so the save-bar
 * reads Unsaved changes + the Save button enables.
 */
export const Dirty: Story = {
  args: {},
  parameters: appRouteParameters("/settings/observability"),
  render: () => (
    <>
      <StorybookWorkspaceSetup />
      <StorybookFieldDirtySetup testId="settings-page-observability-retention-days" value="14" />
    </>
  ),
};

/**
 * Capability variation where log-tail streaming is unavailable.
 */
export const LogTailUnavailable: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/observability"),
    ...storybookMswParameters({
      settings: [
        aghApiMock.get("/api/settings/observability", () =>
          HttpResponse.json({
            ...settingsObservabilitySectionFixture,
            log_tail: {
              available: false,
              stream_url: undefined,
              transport: undefined,
            },
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Loading state while the observability section envelope is still resolving.
 */
export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/observability"),
    ...storybookMswParameters({
      settings: [
        aghApiMock.get("/api/settings/observability", async () => {
          await delay("infinite");
          return HttpResponse.json(settingsObservabilitySectionFixture);
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Error branch shown when the observability request fails.
 */
export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/observability"),
    ...storybookMswParameters({
      settings: [
        aghApiMock.get("/api/settings/observability", () =>
          HttpResponse.json({ error: "Failed to load observability settings" }, { status: 500 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
