import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";

import { aghApiMock } from "@/storybook/openapi-msw";
import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";
import { settingsHooksExtensionsSectionFixture } from "@/systems/settings/mocks";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/SettingsHooks",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component: "Hook declarations and notification presets on the dedicated Hooks route.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  parameters: appRouteParameters("/settings/hooks"),
  render: () => <StorybookWorkspaceSetup />,
};

export const Loading: Story = {
  parameters: {
    ...appRouteParameters("/settings/hooks"),
    ...storybookMswParameters({
      settings: [
        aghApiMock.get("/api/settings/hooks-extensions", async () => {
          await delay("infinite");
          return HttpResponse.json(settingsHooksExtensionsSectionFixture);
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
