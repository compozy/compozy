import type { Meta, StoryObj } from "@storybook/react-vite";
import { StorybookRouteCanvas, appRouteParameters } from "@/storybook/route-story-meta";
import { expect, within } from "storybook/test";
import { storybookMswParameters } from "@/storybook/msw";
import { aghApiMock } from "@/storybook/openapi-msw";
import { delay, HttpResponse } from "msw";
import { agentFixtures } from "@/systems/agent/mocks";
import { storyAgentNames } from "@/storybook/fintech-scenario";

import {
  chooseAlternateRuntimeOption,
  complianceAgentRoute,
  denseFraudAgentHandlers,
  failedSessionPage,
  fraudAgentRoute,
  missingAgentRoute,
  openDetailRuntimeSelector,
  sessionPage,
} from "./agent-detail-story-fixtures";
import { AgentWorkspaceSetup } from "./agent-detail-story-canvas";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/agent/routes/AgentDetail",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Agent detail route stories rendered through the real router. Covers the four-tab cockpit, Overview/Sessions states, and loading/not-found branches.",
      },
    },
  },
};

export default meta;

type Story = StoryObj<typeof meta>;

/**
 * Default agent detail page — Overview cockpit with status pill and toolbar.
 */
export const Default: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(fraudAgentRoute),
    ...denseFraudAgentHandlers(),
  },
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.findByTestId("agent-detail-page")).resolves.toBeDefined();
    await expect(canvas.findByTestId("agent-overview-tab")).resolves.toBeDefined();
    await expect(canvas.findByTestId("agent-page-status")).resolves.toBeDefined();
    await expect(canvas.findByTestId("agent-page-toolbar")).resolves.toBeDefined();
    await expect(canvas.findByTestId("agent-overview-glance")).resolves.toBeDefined();
    expect(canvas.queryByTestId("agent-info-inspector")).toBeNull();
  },
};

export const InstructionsAgent: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(`${fraudAgentRoute}?tab=instructions&file=agent`),
    ...denseFraudAgentHandlers(),
  },
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.findByTestId("agent-file-agent")).resolves.toBeDefined();
    await expect(canvas.findByTestId("agent-file-meta")).resolves.toBeDefined();
    expect(canvas.getByTestId("agent-file-meta")).toHaveTextContent("Read-only here");
  },
};

export const SoulMissing: Story = {
  args: {},
  parameters: appRouteParameters(`${fraudAgentRoute}?tab=instructions&file=soul`),
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.findByTestId("agent-soul-missing")).resolves.toBeDefined();
  },
};

export const SoulEditor: Story = {
  args: {},
  parameters: appRouteParameters(`${fraudAgentRoute}?tab=instructions&file=soul`),
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await (await canvas.findByTestId("agent-soul-create")).click();
    await expect(canvas.findByTestId("agent-soul-editor")).resolves.toBeDefined();
  },
};

export const HeartbeatMissing: Story = {
  args: {},
  parameters: appRouteParameters(`${fraudAgentRoute}?tab=instructions&file=heartbeat`),
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.findByTestId("agent-heartbeat-missing")).resolves.toBeDefined();
  },
};

export const HeartbeatEditor: Story = {
  args: {},
  parameters: appRouteParameters(`${fraudAgentRoute}?tab=instructions&file=heartbeat`),
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await (await canvas.findByTestId("agent-heartbeat-create")).click();
    await expect(canvas.findByTestId("agent-heartbeat-editor")).resolves.toBeDefined();
  },
};

export const Configuration: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(`${fraudAgentRoute}?tab=configuration`),
    ...denseFraudAgentHandlers(),
  },
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.findByTestId("agent-configuration-tab")).resolves.toBeDefined();
    await expect(canvas.findByTestId("agent-config-deny-tools")).resolves.toBeDefined();
    await expect(canvas.findByTestId("agent-mcp-list")).resolves.toBeDefined();
  },
};

export const Sessions: Story = {
  args: {},
  parameters: appRouteParameters(`${fraudAgentRoute}?tab=sessions`),
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.findByTestId("agent-sessions-tab")).resolves.toBeDefined();
  },
};

export const Diagnostics: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(fraudAgentRoute),
    ...storybookMswParameters({
      agent: [
        aghApiMock.get("/api/agents/{name}", () =>
          HttpResponse.json({
            agent: {
              ...agentFixtures.find(agent => agent.name === storyAgentNames.fraud)!,
              diagnostics: [
                {
                  error_kind: "frontmatter.invalid",
                  message: "Unknown field in agent definition",
                  path: "AGENT.md:3",
                },
              ],
            },
          })
        ),
      ],
    }),
  },
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const banner = await canvas.findByTestId("agent-diagnostics-banner");
    const header = await canvas.findByTestId("agent-detail-header");
    expect(banner.compareDocumentPosition(header) & Node.DOCUMENT_POSITION_FOLLOWING).toBe(
      Node.DOCUMENT_POSITION_FOLLOWING
    );
    expect(canvas.queryByTestId("agent-detail-body")?.contains(banner)).toBe(false);
  },
};

export const SessionsError: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(fraudAgentRoute),
    ...storybookMswParameters({
      session: [
        aghApiMock.get("/api/sessions", () =>
          HttpResponse.json({ error: "sessions unavailable" }, { status: 500 })
        ),
      ],
    }),
  },
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.findByTestId("agent-overview-sessions-notice")).resolves.toBeDefined();
  },
};

/**
 * Agent that has no sessions yet, with an empty state inside the Sessions tab.
 */
export const NoSessions: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(`${complianceAgentRoute}?tab=sessions`),
    ...storybookMswParameters({
      session: [aghApiMock.get("/api/sessions", () => HttpResponse.json(sessionPage([])))],
    }),
  },
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.findByTestId("agent-sessions-empty")).resolves.toBeDefined();
  },
};

/**
 * Overview with exact catalog metrics at zero (not derived from the sessions page).
 */
export const OverviewZeroSessions: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(complianceAgentRoute),
    ...storybookMswParameters({
      agent: [
        aghApiMock.get("/api/agents/catalog", ({ request }) => {
          const url = new URL(request.url);
          const name = url.searchParams.get("name")?.trim() ?? "";
          const agent = agentFixtures.find(entry => entry.name === storyAgentNames.compliance)!;
          if (name && name !== agent.name) {
            return HttpResponse.json({
              agents: [],
              facets: { active: 0, categories: [], idle: 0, total: 0 },
              page: { has_more: false, limit: 1, total: 0 },
              sessions_available: true,
            });
          }
          return HttpResponse.json({
            agents: [
              {
                agent,
                sessions: {
                  active: 0,
                  failed: 0,
                  runtime_seconds: 0,
                  total: 0,
                },
              },
            ],
            facets: { active: 0, categories: [], idle: 1, total: 1 },
            page: { has_more: false, limit: 1, total: 1 },
            sessions_available: true,
          });
        }),
      ],
      session: [aghApiMock.get("/api/sessions", () => HttpResponse.json(sessionPage([])))],
    }),
  },
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.findByTestId("agent-overview-tab")).resolves.toBeDefined();
    await expect(canvas.findByTestId("agent-stats-grid")).resolves.toBeDefined();
  },
};

/** Runtime selector open on the detail header. */
export const RuntimeSelectorOpen: Story = {
  args: {},
  parameters: appRouteParameters(fraudAgentRoute),
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    await openDetailRuntimeSelector(canvasElement);
  },
};

/** Runtime mutation stays pending while the header keeps the closed trigger. */
export const RuntimeMutationPending: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(fraudAgentRoute),
    ...storybookMswParameters({
      agent: [
        aghApiMock.put("/api/agents/{name}", async () => {
          await delay(120_000);
          return HttpResponse.json({ error: "unreachable" }, { status: 500 });
        }),
      ],
    }),
  },
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const body = await openDetailRuntimeSelector(canvasElement);
    await chooseAlternateRuntimeOption(await body.findByTestId("runtime-selector-popup"));
    await expect(await canvas.findByTestId("agent-detail-runtime-pending")).toBeInTheDocument();
  },
};

/** Runtime CAS conflict surfaces server-refresh guidance without poisoning cache. */
export const RuntimeMutationConflict: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(fraudAgentRoute),
    ...storybookMswParameters({
      agent: [
        aghApiMock.put("/api/agents/{name}", () =>
          HttpResponse.json({ error: "definition digest conflict" }, { status: 409 })
        ),
      ],
    }),
  },
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const body = await openDetailRuntimeSelector(canvasElement);
    await chooseAlternateRuntimeOption(await body.findByTestId("runtime-selector-popup"));
    await expect(await canvas.findByTestId("agent-detail-runtime-conflict")).toBeInTheDocument();
  },
};

/** Provider-source failures keep the runtime selector inert and expose a retry action. */
export const RuntimeProvidersError: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(fraudAgentRoute),
    ...storybookMswParameters({
      settings: [
        aghApiMock.get("/api/settings/providers", () =>
          HttpResponse.json({ error: "Global providers unavailable" }, { status: 503 })
        ),
      ],
    }),
  },
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(
      canvas.findByTestId("agent-detail-runtime-providers-error")
    ).resolves.toHaveTextContent("Global providers unavailable");
    await expect(
      canvas.findByTestId("agent-detail-runtime-providers-retry")
    ).resolves.toBeDefined();
  },
};

/**
 * Catalog metrics loading while session rows remain independent.
 */
export const MetricsLoading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(fraudAgentRoute),
    ...storybookMswParameters({
      agent: [
        aghApiMock.get("/api/agents/catalog", async () => {
          await delay("infinite");
          return HttpResponse.json({
            agents: [],
            facets: { active: 0, categories: [], idle: 0, total: 0 },
            page: { has_more: false, limit: 1, total: 0 },
            sessions_available: true,
          });
        }),
      ],
    }),
  },
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.findByTestId("agent-overview-metrics-skeleton")).resolves.toBeDefined();
  },
};

/**
 * Sessions list loading branch while catalog metrics stay independent.
 */
export const SessionsLoading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(fraudAgentRoute),
    ...storybookMswParameters({
      session: [
        aghApiMock.get("/api/sessions", async () => {
          await delay("infinite");
          return HttpResponse.json(sessionPage([]));
        }),
      ],
    }),
  },
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.findByTestId("agent-stats-grid")).resolves.toBeDefined();
    await expect(canvas.findByTestId("agent-overview-live-sessions")).resolves.toBeDefined();
  },
};

/**
 * Agent detail loading branch: `/api/agents/:name` is in flight while the shell stays mounted.
 */
export const AgentLoading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(fraudAgentRoute),
    ...storybookMswParameters({
      agent: [
        aghApiMock.get("/api/agents/{name}", async () => {
          await delay(120_000);
          return HttpResponse.json({ agent: agentFixtures[0]! });
        }),
      ],
    }),
  },
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await expect(await body.findByTestId("agent-detail-loading")).toBeInTheDocument();
  },
};

/**
 * Not-found branch: the agent name does not match anything in the workspace.
 */
export const NotFound: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(missingAgentRoute),
    ...storybookMswParameters({
      agent: [
        aghApiMock.get("/api/agents/{name}", ({ params }) =>
          HttpResponse.json({ error: `Agent not found: ${String(params.name)}` }, { status: 404 })
        ),
      ],
    }),
  },
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.findByTestId("agent-detail-not-found")).resolves.toBeDefined();
  },
};

/**
 * Failed-session branch: at least one session has a populated failure payload, surfacing the FAILED chip.
 */
export const WithFailedSession: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(`${fraudAgentRoute}?tab=sessions`),
    ...storybookMswParameters({
      session: [aghApiMock.get("/api/sessions", () => HttpResponse.json(failedSessionPage()))],
    }),
  },
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(
      canvas.findByTestId("agent-session-status-sess_fraud_failed")
    ).resolves.toHaveTextContent("FAILED");
  },
};

/**
 * Live agents list returning many agents confirms the sidebar still resolves the active row when the
 * detail route's agent is deeper in the list.
 */
export const ManyAgents: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(fraudAgentRoute),
    ...storybookMswParameters({
      agent: [aghApiMock.get("/api/agents", () => HttpResponse.json({ agents: agentFixtures }))],
    }),
  },
  render: () => <AgentWorkspaceSetup />,
};
