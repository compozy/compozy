import type { Meta, StoryObj } from "@storybook/react-vite";
import { http, HttpResponse } from "msw";

import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";
import { accountStatusStoryFixture } from "../mocks/handlers";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/account/routes/SignIn",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Full-window mandatory sign-in screen (U0) rendered through the `/sign-in` route.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/sign-in"),
    ...storybookMswParameters({
      settings: [
        http.get("/api/account", () =>
          HttpResponse.json({
            ...accountStatusStoryFixture,
            gate: "enforced",
            state: "signed_out",
            signed_in: false,
            source: null,
            device: null,
          })
        ),
        http.get(
          "/api/account/sign-in/events",
          () =>
            new HttpResponse("", {
              status: 200,
              headers: { "Content-Type": "text/event-stream" },
            })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
