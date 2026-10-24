import type { Meta, StoryObj } from "@storybook/react-vite";
import { StorybookRouteCanvas, appRouteParameters } from "@/storybook/route-story-meta";
import { expect, within } from "storybook/test";
import { storybookMswParameters } from "@/storybook/msw";
import { compozyApiMock } from "@/storybook/openapi-msw";
import { delay, HttpResponse } from "msw";
import { agentFixtures } from "@/systems/agent/mocks";

import { fraudAgentRoute } from "./agent-detail-story-fixtures";
import { AgentWorkspaceSetup } from "./agent-detail-story-canvas";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/agent/routes/AgentDetailLoading",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Agent detail loading branches rendered through the real router: the agent, tab, heartbeat status and SOUL.md reads in flight.",
      },
    },
  },
};

export default meta;

type Story = StoryObj<typeof meta>;

/** The agent request never settles, so the page holds its loading state. */
function agentLoadingParameters(path: string) {
  return {
    ...appRouteParameters(path),
    ...storybookMswParameters({
      agent: [
        compozyApiMock.get("/api/agents/{name}", async () => {
          await delay(120_000);
          return HttpResponse.json({ agent: agentFixtures[0]! });
        }),
      ],
    }),
  };
}

/**
 * Agent detail loading branch: `/api/agents/:name` is in flight while the shell stays mounted.
 */
export const AgentLoading: Story = {
  args: {},
  parameters: agentLoadingParameters(fraudAgentRoute),
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await expect(await body.findByTestId("agent-detail-loading")).toBeInTheDocument();
    await expect(await body.findByTestId("agent-overview-skeleton")).toBeInTheDocument();
  },
};

/** The open tab decides the skeleton: AGENT.md is a prompt panel, SOUL.md the editor panel. */
export const AgentLoadingInstructions: Story = {
  args: {},
  parameters: agentLoadingParameters(`${fraudAgentRoute}?tab=instructions&file=soul`),
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await expect(await body.findByTestId("agent-instructions-skeleton")).toBeInTheDocument();
    await expect(await body.findByTestId("agent-soul-loading")).toBeInTheDocument();
  },
};

export const AgentLoadingConfiguration: Story = {
  args: {},
  parameters: agentLoadingParameters(`${fraudAgentRoute}?tab=configuration`),
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await expect(await body.findByTestId("agent-configuration-skeleton")).toBeInTheDocument();
  },
};

export const AgentLoadingSessions: Story = {
  args: {},
  parameters: agentLoadingParameters(`${fraudAgentRoute}?tab=sessions`),
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await expect(await body.findByTestId("agent-sessions-tab-skeleton")).toBeInTheDocument();
    await expect(await body.findByTestId("agent-sessions-loading")).toBeInTheDocument();
  },
};

/** The heartbeat status read is in flight: the status card holds its place above the file. */
export const HeartbeatStatusLoading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(`${fraudAgentRoute}?tab=instructions&file=heartbeat`),
    ...storybookMswParameters({
      agent: [
        compozyApiMock.get("/api/agents/{name}/heartbeat/status", async () => {
          await delay("infinite");
          return HttpResponse.json(null);
        }),
      ],
    }),
  },
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.findByTestId("agent-heartbeat-ops-skeleton")).resolves.toBeDefined();
  },
};

/** The agent is loaded and its SOUL.md read is in flight: the editor panel holds its place. */
export const SoulLoading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(`${fraudAgentRoute}?tab=instructions&file=soul`),
    ...storybookMswParameters({
      agent: [
        compozyApiMock.get("/api/agents/{name}/soul", async () => {
          await delay("infinite");
          return HttpResponse.json(null);
        }),
      ],
    }),
  },
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.findByTestId("agent-soul-loading")).resolves.toBeDefined();
  },
};
