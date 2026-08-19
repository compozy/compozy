import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";
import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";
import { settingsHooksExtensionsSectionFixture } from "@/systems/settings/mocks";

const unhealthyFixture = structuredClone(settingsHooksExtensionsSectionFixture);
const notes = unhealthyFixture.installed?.find(extension => extension.name === "notes");
if (notes?.palette) {
  notes.health = "unhealthy";
  notes.health_message = "crash loop";
  for (const command of notes.palette.commands) {
    command.available = false;
    command.reason = "extension notes is unhealthy (crash loop)";
  }
  for (const view of notes.palette.views) {
    view.available = false;
    view.reason = "extension notes is unhealthy (crash loop)";
  }
}

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/SettingsExtensions",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Extension install sources and trust policy on the dedicated Extensions settings route.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  parameters: appRouteParameters("/settings/extensions"),
  render: () => <StorybookWorkspaceSetup />,
};

export const Loading: Story = {
  parameters: {
    ...appRouteParameters("/settings/extensions"),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/settings/hooks-extensions", async () => {
          await delay("infinite");
          return HttpResponse.json(settingsHooksExtensionsSectionFixture);
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const DormantDefault: Story = {
  parameters: {
    ...appRouteParameters("/settings/extensions"),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/settings/hooks-extensions", () =>
          HttpResponse.json(settingsHooksExtensionsSectionFixture)
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const UnhealthyContribution: Story = {
  parameters: {
    ...appRouteParameters("/settings/extensions"),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/settings/hooks-extensions", () =>
          HttpResponse.json(unhealthyFixture)
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
