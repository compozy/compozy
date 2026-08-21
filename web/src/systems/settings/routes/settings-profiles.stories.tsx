import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";
import { storybookMswParameters } from "@/storybook/msw";
import { appRouteParameters } from "@/storybook/route-story-meta";
import { StorybookRouteCanvas, StorybookWorkspaceSetup } from "@/storybook/route-story";
import { defaultProfileFixture, profileFixtures } from "@/systems/profiles/mocks";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/SettingsProfiles",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Settings → Profiles. The default read is the active list plus one honest sentence about what a profile is not; the archived list and the project → profile map sit one disclosure deeper because neither is a daily question.",
      },
    },
  },
};

export default meta;

type Story = StoryObj<typeof StorybookRouteCanvas>;

/** Populated: active profiles, one needing setup, archives behind disclosure. */
export const Default: Story = {
  parameters: appRouteParameters("/settings/profiles"),
  render: () => <StorybookWorkspaceSetup />,
};

/** A fresh machine: `default` alone, with nothing demoted because nothing exists. */
export const SingleProfile: Story = {
  parameters: {
    ...appRouteParameters("/settings/profiles"),
    ...storybookMswParameters({
      profiles: [
        compozyApiMock.get("/api/profiles", () => HttpResponse.json([defaultProfileFixture])),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Every profile archived except the permanent one — the list still reads calmly. */
export const AllArchived: Story = {
  parameters: {
    ...appRouteParameters("/settings/profiles"),
    ...storybookMswParameters({
      profiles: [
        compozyApiMock.get("/api/profiles", () =>
          HttpResponse.json(
            profileFixtures.map(profile =>
              profile.name === "default"
                ? profile
                : { ...profile, state: "archived", archived_at: "2026-07-01T09:00:00Z" }
            )
          )
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
