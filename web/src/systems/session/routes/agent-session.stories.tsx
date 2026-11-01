import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { compozyApiMock } from "@/storybook/openapi-msw";

import { storyAgentNames, storySessionIds, storyWorkspaceIds } from "@/storybook/fintech-scenario";
import { primarySessionFixture, sessionTranscriptPermissionFixture } from "@/systems/session/mocks";
import { SESSIONS_BOARD_SESSIONS } from "@/systems/session/mocks/sessions-board-fixtures";
import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/session/routes/AgentSession",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Nested session chat route under an agent. Mirrors the previous routes/app/session stories — transcript hydration, stopped session, permission prompt, loading and not-found behaviors — through the canonical /agents/$name/sessions/$id URL.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const fraudSessionRoute = `/agents/${storyAgentNames.fraud}/sessions/${storySessionIds.fraud}`;
const marketingStoppedSessionRoute = `/agents/${storyAgentNames.marketing}/sessions/${storySessionIds.marketing}`;
const missingSessionRoute = `/agents/${storyAgentNames.fraud}/sessions/sess-missing`;

function transcriptEntries<T>(messages: T[]) {
  return messages.map((message, index) => ({
    message,
    sequence: index + 1,
    start_sequence: index + 1,
  }));
}

function transcriptPayload<T>(messages: T[]) {
  return {
    entries: transcriptEntries(messages),
    epoch: 1,
    generation: 1,
    has_older: false,
    limit: 200,
    max_sequence: messages.length,
  };
}

/**
 * Active session transcript route for the primary payout-operations session.
 */
export const Default: Story = {
  args: {},
  parameters: appRouteParameters(fraudSessionRoute),
  render: () => <StorybookWorkspaceSetup workspaceId={storyWorkspaceIds.risk} />,
};

const LONG_SIDEBAR_SESSIONS = Object.values(SESSIONS_BOARD_SESSIONS).flat();
const LONG_SIDEBAR_OUTLINE = Array.from({ length: 12 }, (_, index) => ({
  at: new Date(Date.UTC(2026, 8, 6, 14, index)).toISOString(),
  preview: `Check launch blocker ${index + 1} before the cutover`,
  reply_preview: "Checked. The partner-bank timeout copy still needs compliance sign-off.",
  sequence: index + 1,
  turn_id: `turn-${index + 1}`,
}));

/**
 * The window with a long session list and the message trail on the transcript's
 * right edge: the tonal hierarchy and the trail read together, as they ship.
 */
export const LongSidebarWithTrail: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(fraudSessionRoute),
    ...storybookMswParameters({
      session: [
        compozyApiMock.get("/api/sessions", () =>
          HttpResponse.json({
            sessions: [primarySessionFixture, ...LONG_SIDEBAR_SESSIONS],
            page: {
              has_more: false,
              limit: 200,
              total: LONG_SIDEBAR_SESSIONS.length + 1,
            },
          })
        ),
        compozyApiMock.get(
          "/api/workspaces/{workspace_id}/sessions/{session_id}/transcript/outline",
          () => HttpResponse.json({ entries: LONG_SIDEBAR_OUTLINE })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup workspaceId={storyWorkspaceIds.risk} />,
};

/**
 * Loading branch before the selected session resource resolves.
 */
export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(fraudSessionRoute),
    ...storybookMswParameters({
      session: [
        compozyApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}", async () => {
          await delay("infinite");
          return HttpResponse.json({ session: primarySessionFixture });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup workspaceId={storyWorkspaceIds.risk} />,
};

/**
 * Stopped-session branch that keeps the composer available while attach remains disabled.
 */
export const Stopped: Story = {
  args: {},
  parameters: appRouteParameters(marketingStoppedSessionRoute),
  render: () => <StorybookWorkspaceSetup workspaceId={storyWorkspaceIds.growth} />,
};

/**
 * Pending permission prompt hydrated from the transcript replay payload.
 */
export const PendingPermission: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(fraudSessionRoute),
    ...storybookMswParameters({
      session: [
        compozyApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}/transcript", () =>
          HttpResponse.json(transcriptPayload(sessionTranscriptPermissionFixture))
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup workspaceId={storyWorkspaceIds.risk} />,
};

/**
 * Not-found session behavior — toast fires and navigation falls back to the parent agent route.
 */
export const NotFoundRedirect: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(missingSessionRoute),
    ...storybookMswParameters({
      session: [
        compozyApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}", ({ params }) =>
          HttpResponse.json(
            { error: `Session not found: ${String(params.session_id)}` },
            { status: 404 }
          )
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup workspaceId={storyWorkspaceIds.risk} />,
};
