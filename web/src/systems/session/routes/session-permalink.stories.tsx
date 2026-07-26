import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";

import { aghApiMock } from "@/storybook/openapi-msw";
import { storySessionIds } from "@/storybook/fintech-scenario";
import { storybookMswParameters } from "@/storybook/msw";
import { StorybookRouteCanvas, appRouteParameters } from "@/storybook/route-story-meta";

const frontendPermalinkRoute = `/session/${storySessionIds.frontend}`;

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/session/routes/SessionPermalink",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Permalink route that resolves a session id and redirects to the canonical /agents/$name/sessions/$id route.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const RedirectsToCanonicalSession: Story = {
  args: {},
  parameters: appRouteParameters(frontendPermalinkRoute),
};

export const NotFound: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/session/sess_missing"),
    ...storybookMswParameters({
      session: [
        aghApiMock.get("/api/sessions/{session_id}", ({ params }) =>
          HttpResponse.json(
            { error: `Session not found: ${String(params.session_id)}` },
            { status: 404 }
          )
        ),
      ],
    }),
  },
};
