import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";
import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

import {
  activityTreeCallsFixture,
  buildLargeTreeFixture,
  createAgentCommsHandlers,
  nineStateCallsFixture,
} from "../mocks";
import type { CallPayload } from "../types";

/**
 * Deterministic Activity states for design-parity capture.
 *
 * Each story gets its **own** mock server over its **own** dataset. Storybook
 * can load several stories at once, and a handler set reading shared module
 * state would let one story's population leak into another's screenshot.
 *
 * The factory is the same one the product's tests use, so a story pages the way
 * the daemon pages: `total` describes the whole filtered set, `next_cursor` is
 * offered while rows remain and withheld on the last page. A handler that
 * returned the first 100 of 150 with no cursor would render a tree claiming 150
 * with no way to reach the rest — exactly the lie these stories exist to catch.
 */
function callsHandlers(calls: readonly CallPayload[]) {
  return createAgentCommsHandlers({ calls, messages: [] });
}

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/agent-comms/routes/AgentsActivity",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Delegation trees at /agents/activity — deterministic states for design-parity screenshots (live trees, the nine-state spectrum, scale, empty).",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Two live trees at depths 1–3 — the default view. */
export const Default: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/agents/activity"),
    ...storybookMswParameters({ "agent-comms": callsHandlers(activityTreeCallsFixture) }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Every one of the nine states a call can be in, in one tree. */
export const StateSpectrum: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/agents/activity"),
    ...storybookMswParameters({ "agent-comms": callsHandlers(nineStateCallsFixture) }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** The scale case: 150 calls under one root, one of them needing a look. */
export const LargeTree: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/agents/activity"),
    ...storybookMswParameters({ "agent-comms": callsHandlers(buildLargeTreeFixture(150)) }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** A fresh workspace: the empty state teaches the feature. */
export const Empty: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/agents/activity"),
    ...storybookMswParameters({ "agent-comms": callsHandlers([]) }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/agents/activity"),
    ...storybookMswParameters({
      "agent-comms": [
        compozyApiMock.get("/api/workspaces/{workspace_id}/calls", async () => {
          await delay("infinite");
          return HttpResponse.json({ items: [], total: 0 });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const LoadError: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/agents/activity"),
    ...storybookMswParameters({
      "agent-comms": [
        compozyApiMock.get("/api/workspaces/{workspace_id}/calls", ({ response }) =>
          response(503).json({ error: "calls service unavailable" })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
