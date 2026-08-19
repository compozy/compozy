import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";

import { appRouteParameters } from "@/storybook/route-story-meta";
import { StorybookRouteCanvas, StorybookWorkspaceSetup } from "@/storybook/route-story";
import { compozyApiMock } from "@/storybook/openapi-msw";
import { storybookMswParameters } from "@/storybook/msw";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/SettingsPalette",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Command palette personalization. `[cmd_palette]` applies live, so the switch is the commit and the section carries no save bar. Turning it off stops recording; what the palette already learned is kept until it is reset.",
      },
    },
  },
};

export default meta;

type Story = StoryObj<typeof StorybookRouteCanvas>;

/** Personalization on — the shipped default. */
export const Default: Story = {
  parameters: appRouteParameters("/settings/palette"),
  render: () => <StorybookWorkspaceSetup />,
};

/** Off: the palette stops learning, and the ranking it shows is the shipped order. */
export const PersonalizationOff: Story = {
  parameters: {
    ...appRouteParameters("/settings/palette"),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/settings/cmd-palette", () =>
          HttpResponse.json({
            section: "cmd-palette",
            scope: "global",
            available_scopes: ["global", "workspace"],
            personalization: false,
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
