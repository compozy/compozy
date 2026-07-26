import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";

import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";
import { statusFixture } from "@/systems/status/mocks/fixtures";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/runtime/routes/Home",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Full app-shell route stories for the home dashboard , daemon status + key metrics , and the onboarding branch.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Healthy daemon , green StatusDot, connected pill, populated metric grid.
 */
export const Default: Story = {
  args: {},
  parameters: appRouteParameters("/"),
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Daemon responded but reported a non-healthy status , warning StatusDot, still
 * connected pill, metrics still populated from the partial payload.
 */
export const Degraded: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/"),
    ...storybookMswParameters({
      daemon: [
        aghApiMock.get("/api/status", () =>
          HttpResponse.json({
            ...statusFixture,
            health: { ...statusFixture.health, status: "degraded" },
            memory: {
              ...statusFixture.memory,
              dream_enabled: false,
              global_files: 0,
              workspace_files: 0,
            },
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Daemon unreachable , danger StatusDot, disconnected pill, recovery hint card
 * via Empty + ConnectionIndicator composition.
 */
export const Disconnected: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/"),
    ...storybookMswParameters({
      daemon: [
        aghApiMock.get("/api/status", () =>
          HttpResponse.json({ error: "daemon offline" }, { status: 503 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * First paint with slow network , skeletons in the daemon and metric sections.
 */
export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/"),
    ...storybookMswParameters({
      daemon: [
        aghApiMock.get("/api/status", async () => {
          await delay("infinite");
          return HttpResponse.json(statusFixture);
        }),
      ],
      workspace: [
        aghApiMock.get("/api/workspaces", async () => {
          await delay("infinite");
          return HttpResponse.json({ workspaces: [] });
        }),
      ],
      agent: [
        aghApiMock.get("/api/agents", async () => {
          await delay("infinite");
          return HttpResponse.json({ agents: [] });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Workspaces fetch fails , Empty error card replaces the dashboard body.
 */
export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/"),
    ...storybookMswParameters({
      workspace: [
        aghApiMock.get("/api/workspaces", () =>
          HttpResponse.json({ error: "workspaces unavailable" }, { status: 500 })
        ),
      ],
    }),
  },
};

/**
 * Onboarding branch shown when the daemon has no configured workspaces yet.
 * The shell handles this above the route, so the route component never renders.
 */
export const Onboarding: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/"),
    ...storybookMswParameters({
      onboarding: [
        aghApiMock.get("/api/onboarding", () =>
          HttpResponse.json({ onboarding: { completed: false } })
        ),
      ],
      workspace: [aghApiMock.get("/api/workspaces", () => HttpResponse.json({ workspaces: [] }))],
    }),
  },
};
