import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";
import { expect, userEvent, within } from "storybook/test";

import { storybookMswParameters } from "@/storybook/msw";
import { StorybookFieldDirtySetup } from "@/storybook/settings-state-helpers";
import {
  StorybookRestartNoticeSetup,
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";
import { settingsMemorySectionFixture } from "@/systems/settings/mocks";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/SettingsMemory",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Memory settings route stories covering the save surface, consolidation action, restart notice, and failure states.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default memory page with persistence controls and dream thresholds.
 */
export const Default: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/memory"),
    ...storybookMswParameters({
      daemon: [
        aghApiMock.get("/api/onboarding", () =>
          HttpResponse.json({ onboarding: { completed: true } })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Dirty shell state -- the global memory directory has been edited so the save-bar
 * reads Unsaved changes + the Save button enables.
 */
export const Dirty: Story = {
  args: {},
  parameters: appRouteParameters("/settings/memory"),
  render: () => (
    <>
      <StorybookWorkspaceSetup />
      <StorybookFieldDirtySetup
        testId="settings-page-memory-global-dir-input"
        value="~/.agh/memory-dirty"
      />
    </>
  ),
};

/**
 * Dream action triggered from the dream section header.
 */
export const DreamTriggered: Story = {
  args: {},
  parameters: appRouteParameters("/settings/memory"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("settings-page-memory-dream-trigger"));
    await expect(
      canvas.findByTestId("settings-page-memory-action-message")
    ).resolves.toHaveTextContent("Dream triggered");
  },
};

/**
 * Restart-required banner after a memory configuration mutation that affects daemon behavior.
 */
export const RestartNotice: Story = {
  args: {},
  parameters: appRouteParameters("/settings/memory"),
  render: () => (
    <>
      <StorybookWorkspaceSetup />
      <StorybookRestartNoticeSetup section="memory" />
    </>
  ),
};

/**
 * Initial loading state while the memory section envelope is still resolving.
 */
export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/memory"),
    ...storybookMswParameters({
      settings: [
        aghApiMock.get("/api/settings/memory", async () => {
          await delay("infinite");
          return HttpResponse.json(settingsMemorySectionFixture);
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Error branch shown when the memory settings request fails.
 */
export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/memory"),
    ...storybookMswParameters({
      settings: [
        aghApiMock.get("/api/settings/memory", () =>
          HttpResponse.json({ error: "Failed to load memory settings" }, { status: 500 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
