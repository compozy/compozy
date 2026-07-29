import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";
import { expect, userEvent, within } from "storybook/test";

import { compozyApiMock } from "@/storybook/openapi-msw";
import {
  storySessionIds,
  storyWorkspaceIds,
  storyWorkspaceNames,
} from "@/storybook/fintech-scenario";
import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

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
        compozyApiMock.get("/api/sessions/{session_id}", ({ params }) =>
          HttpResponse.json(
            { error: `Session not found: ${String(params.session_id)}` },
            { status: 404 }
          )
        ),
      ],
    }),
  },
};

/**
 * The linked session misses in the active workspace and the owner projection names another one:
 * the route canonicalizes into `?workspaceSwitch=confirm` and asks before switching (ADR-004).
 */
export const ForeignWorkspaceConfirm: Story = {
  render: () => <StorybookWorkspaceSetup workspaceId={storyWorkspaceIds.risk} />,
  parameters: {
    ...appRouteParameters(frontendPermalinkRoute),
    ...storybookMswParameters({
      session: [
        compozyApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}", ({ params }) =>
          HttpResponse.json(
            { error: `Session not found: ${String(params.session_id)}` },
            { status: 404 }
          )
        ),
        compozyApiMock.get("/api/sessions/{session_id}/owner", ({ params }) =>
          HttpResponse.json({
            session_id: String(params.session_id),
            workspace_id: storyWorkspaceIds.product,
            workspace_name: storyWorkspaceNames.product,
          })
        ),
      ],
    }),
  },
};

/** Cancelled confirmation: the workspace stays put and the route keeps today's not-found result. */
export const ForeignWorkspaceCancel: Story = {
  render: () => <StorybookWorkspaceSetup workspaceId={storyWorkspaceIds.risk} />,
  tags: ["play-fn"],
  parameters: ForeignWorkspaceConfirm.parameters,
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    const dialog = await body.findByTestId("session-workspace-switch-dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Stay here" }));
    await expect(body.findByText("Session not found")).resolves.toBeVisible();
    await expect(body.queryByTestId("session-workspace-switch-dialog")).not.toBeInTheDocument();
  },
};
