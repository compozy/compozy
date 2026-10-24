import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { expect, userEvent, within } from "storybook/test";

import { compozyApiMock } from "@/storybook/openapi-msw";
import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";
import { settingsProviderUsageSectionFixture } from "@/systems/settings/mocks";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/SettingsProviderUsage",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Settings › Provider usage: the master switch, the background refresh cadence, one switch per configured provider with a usage collector, and — only when the daemon runs on macOS — Claude Keychain consent. One draft behind the save bar, applied through the settings lifecycle.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const PATH = "/settings/provider-usage";

/** On, every 5 minutes, OpenRouter turned off; the daemon is not on macOS, so no Keychain row. */
export const Default: Story = {
  parameters: appRouteParameters(PATH),
  render: () => <StorybookWorkspaceSetup />,
};

/** The daemon runs on macOS (`keychain_supported`): the Privacy row appears. */
export const MacHostKeychain: Story = {
  parameters: {
    ...appRouteParameters(PATH),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/settings/provider-usage", () =>
          HttpResponse.json({ ...settingsProviderUsageSectionFixture, keychain_supported: true })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Feature off: only the master switch remains; the provider list is hidden, not dimmed. */
export const TurnedOff: Story = {
  parameters: {
    ...appRouteParameters(PATH),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/settings/provider-usage", () =>
          HttpResponse.json({
            ...settingsProviderUsageSectionFixture,
            config: { ...settingsProviderUsageSectionFixture.config, enabled: false },
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** The apply is rejected: the daemon's validation message stays in the save bar, draft kept. */
export const ApplyValidationError: Story = {
  parameters: {
    ...appRouteParameters(PATH),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.patch("/api/settings/provider-usage", () =>
          HttpResponse.json(
            {
              error: "provider_usage.refresh_interval must be between 1m and 1h",
              code: "validation_error",
            },
            { status: 400 }
          )
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("settings-provider-usage-provider-codex"));
    await userEvent.click(await canvas.findByTestId("settings-page-provider-usage-save"));
    await expect(
      await canvas.findByText(/refresh_interval must be between 1m and 1h/)
    ).toBeInTheDocument();
  },
};

/** Loading state: the collection, providers and privacy groups hold their place. */
export const Loading: Story = {
  parameters: {
    ...appRouteParameters(PATH),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/settings/provider-usage", async () => {
          await delay("infinite");
          return HttpResponse.json(settingsProviderUsageSectionFixture);
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
