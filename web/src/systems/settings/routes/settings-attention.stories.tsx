import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";

import { appRouteParameters } from "@/storybook/route-story-meta";
import { StorybookRouteCanvas, StorybookWorkspaceSetup } from "@/storybook/route-story";
import { compozyApiMock } from "@/storybook/openapi-msw";
import { storybookMswParameters } from "@/storybook/msw";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/SettingsAttention",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Attention delivery policy: three channels and a mute list. Everything applies live, so the section has no save bar. The system row renders its real platform state — it can never claim to be armed when permission was refused.",
      },
    },
  },
};

export default meta;

type Story = StoryObj<typeof StorybookRouteCanvas>;

/** Defaults: toasts on, sound on, system off, nothing muted. */
export const Default: Story = {
  parameters: appRouteParameters("/settings/attention"),
  render: () => <StorybookWorkspaceSetup />,
};

/** A muted workspace keeps its bell rows and counts — only delivery stops. */
export const MutedWorkspace: Story = {
  parameters: {
    ...appRouteParameters("/settings/attention"),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/settings/attention", () =>
          HttpResponse.json({
            section: "attention",
            scope: "global",
            available_scopes: ["global"],
            config: {
              toasts: true,
              sound: false,
              system: false,
              muted_workspaces: ["workspace-1"],
            },
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** The daemon is unreachable: the page states it and offers the one useful action. */
export const Error: Story = {
  parameters: {
    ...appRouteParameters("/settings/attention"),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/settings/attention", () =>
          HttpResponse.json({ error: "settings unavailable" }, { status: 500 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
