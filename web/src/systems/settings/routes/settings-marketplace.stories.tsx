import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { expect, userEvent, within } from "storybook/test";

import { compozyApiMock } from "@/storybook/openapi-msw";
import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRestartNoticeSetup,
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";
import { marketplaceSourceHandlers, marketplaceSourceFixtures } from "@/systems/marketplace/mocks";
import { settingsMarketplaceSectionFixture } from "@/systems/settings/mocks";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/SettingsMarketplace",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Settings › Marketplace: the catalog keys behind the save bar and the plugin marketplaces as collapsible rows — Compozy catalog always on, presets with a switch, custom rows with Remove in the fold, a degraded row with its reason and diagnostics.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Feed · preset on · preset off · custom degraded, exactly as `GET /api/marketplace/sources` lists them. */
export const Default: Story = {
  parameters: appRouteParameters("/settings/marketplace"),
  render: () => <StorybookWorkspaceSetup />,
};

/** The degraded custom row opened: sentence with the last-read age, mono reason, diagnostics, Try again. */
export const DegradedRowOpen: Story = {
  parameters: appRouteParameters("/settings/marketplace"),
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(
      await canvas.findByTestId("settings-page-marketplace-source-team-plugins-disclosure")
    );
    await expect(
      canvas.findByTestId("settings-page-marketplace-source-team-plugins-reason")
    ).resolves.toBeDefined();
    await expect(
      canvas.findByTestId("settings-page-marketplace-source-team-plugins-diagnostics")
    ).resolves.toBeDefined();
  },
};

/** Every source healthy: the preset that is off still names its repository inside the fold. */
export const AllHealthy: Story = {
  parameters: {
    ...appRouteParameters("/settings/marketplace"),
    ...storybookMswParameters({
      marketplace: marketplaceSourceHandlers([
        marketplaceSourceFixtures.feed,
        marketplaceSourceFixtures.presetOn,
        marketplaceSourceFixtures.presetOff,
        {
          ...marketplaceSourceFixtures.customDegraded,
          diagnostics: [],
          error: undefined,
          error_class: undefined,
          installable: 4,
          state: "ok",
        },
      ]),
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Add plugin marketplace… from Settings opens the same dialog as Add ▾ in Browse. */
export const AddDialogFromSettings: Story = {
  parameters: appRouteParameters("/settings/marketplace"),
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("settings-page-marketplace-sources-add"));
    await expect(
      within(document.body).findByTestId("add-marketplace-dialog")
    ).resolves.toBeDefined();
  },
};

/** Restart notice after a catalog settings save the daemon must reconcile. */
export const RestartNotice: Story = {
  parameters: appRouteParameters("/settings/marketplace"),
  render: () => (
    <>
      <StorybookWorkspaceSetup />
      <StorybookRestartNoticeSetup section="marketplace" />
    </>
  ),
};

/** The catalog section is still resolving. */
export const Loading: Story = {
  parameters: {
    ...appRouteParameters("/settings/marketplace"),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/settings/marketplace", async () => {
          await delay("infinite");
          return HttpResponse.json(settingsMarketplaceSectionFixture);
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** The catalog section request failed; the sources group is not reached. */
export const Error: Story = {
  parameters: {
    ...appRouteParameters("/settings/marketplace"),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/settings/marketplace", () =>
          HttpResponse.json({ error: "Failed to load marketplace settings" }, { status: 500 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** The sources list could not be read while the catalog keys loaded: the group says so with Retry. */
export const SourcesUnavailable: Story = {
  parameters: {
    ...appRouteParameters("/settings/marketplace"),
    ...storybookMswParameters({
      marketplace: [
        compozyApiMock.get("/api/marketplace/sources", () =>
          HttpResponse.json({ error: "sources: daemon store unavailable" }, { status: 500 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
