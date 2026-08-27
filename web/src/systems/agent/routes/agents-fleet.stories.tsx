import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";
import { storybookMswParameters } from "@/storybook/msw";
import { storyAgentNames } from "@/storybook/fintech-scenario";
import { agentCatalogMockResponse, agentFixtures } from "@/systems/agent/mocks";
import { buildCallFixture, createAgentCommsHandlers } from "@/systems/agent-comms/mocks";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

const fleetRunningCallHandlers = createAgentCommsHandlers({
  calls: [
    buildCallFixture({
      call_id: "call_fleet_cto_1",
      agent: storyAgentNames.cto,
      state: "running",
    }),
    buildCallFixture({
      call_id: "call_fleet_cto_2",
      agent: storyAgentNames.cto,
      state: "running",
    }),
    buildCallFixture({
      call_id: "call_fleet_frontend_1",
      agent: storyAgentNames.frontend,
      state: "running",
    }),
  ],
  messages: [],
});

function fleetCategoryPath(index: number): string[] | undefined {
  if (index % 3 === 0) return ["Engineering", "Platform", "Release"];
  if (index % 3 === 1) return ["Operations"];
  return undefined;
}

const fleetStoryAgents = agentFixtures.map((agent, index) => ({
  ...agent,
  category_path: fleetCategoryPath(index),
  ...(index === 0
    ? {
        layer: "project_profile",
        origin: "workspace" as const,
        shadows: [
          { layer: "project", path: "/workspace/.compozy/agents/reviewer/AGENT.md" },
          { layer: "profile", path: "/profiles/default/agents/reviewer/AGENT.md" },
        ],
        workspace_id: "ws_launch_hq",
      }
    : {}),
  ...(index === 1
    ? {
        diagnostics: [
          {
            error_kind: "validation",
            message: "Provider configuration is incomplete",
            path: "AGENT.md",
          },
        ],
      }
    : {}),
}));

const loadedAgentHandlers = [
  compozyApiMock.get("/api/agents/catalog", ({ request }) =>
    HttpResponse.json(
      agentCatalogMockResponse(request, fleetStoryAgents, {
        activeAgents: [fleetStoryAgents[0]!.name],
      })
    )
  ),
];

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/agent/routes/AgentsFleet",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Fleet listing at /agents — deterministic states for design-parity screenshots (loading, loaded rows/cards, first-run empty, filtered empty, agents error, sessions partial).",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Catalog rows with workspace-scoped running-call pills and Catalog | Activity tabs. */
export const Loaded: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/agents"),
    ...storybookMswParameters({
      agent: loadedAgentHandlers,
      "agent-comms": fleetRunningCallHandlers,
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Card view of the same catalog population. */
export const LoadedCards: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/agents?view=cards"),
    ...storybookMswParameters({
      agent: loadedAgentHandlers,
      "agent-comms": fleetRunningCallHandlers,
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/agents"),
    ...storybookMswParameters({
      agent: [
        compozyApiMock.get("/api/agents/catalog", async ({ request }) => {
          await delay("infinite");
          return HttpResponse.json(agentCatalogMockResponse(request, []));
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const FirstRunEmpty: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/agents"),
    ...storybookMswParameters({
      agent: [
        compozyApiMock.get("/api/agents/catalog", ({ request }) =>
          HttpResponse.json(agentCatalogMockResponse(request, []))
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const FilteredEmpty: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/agents?q=no-such-agent"),
    ...storybookMswParameters({}),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const AgentsError: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/agents"),
    ...storybookMswParameters({
      agent: [
        compozyApiMock.get("/api/agents/catalog", () =>
          HttpResponse.json({ error: "agents unavailable" }, { status: 500 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const SessionsPartial: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/agents"),
    ...storybookMswParameters({
      agent: [
        compozyApiMock.get("/api/agents/catalog", ({ request }) =>
          HttpResponse.json(
            agentCatalogMockResponse(request, fleetStoryAgents, { sessionsAvailable: false })
          )
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Long agent name + deep category path for truncation/wrap visual contract. */
export const LongNameCategory: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/agents"),
    ...storybookMswParameters({
      agent: [
        compozyApiMock.get("/api/agents/catalog", ({ request }) =>
          HttpResponse.json(
            agentCatalogMockResponse(request, [
              {
                ...fleetStoryAgents[0]!,
                name: "launch-room-merchant-risk-settlement-compliance-orchestrator-v2",
                category_path: [
                  "Operations",
                  "Risk",
                  "Settlement",
                  "Launch Room",
                  "Compliance Gates",
                ],
              },
            ])
          )
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Catalog row with agent diagnostics (invalid definition signal). */
export const InvalidAgent: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/agents"),
    ...storybookMswParameters({
      agent: [
        compozyApiMock.get("/api/agents/catalog", ({ request }) =>
          HttpResponse.json(
            agentCatalogMockResponse(request, [
              {
                ...fleetStoryAgents[1]!,
                diagnostics: [
                  {
                    error_kind: "validation",
                    message: "Provider configuration is incomplete",
                    path: "AGENT.md",
                  },
                ],
              },
            ])
          )
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
