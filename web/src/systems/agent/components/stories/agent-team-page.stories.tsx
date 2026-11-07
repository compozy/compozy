import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";
import { fn } from "storybook/test";

import { storybookMswParameters } from "@/storybook/msw";
import { compozyApiMock } from "@/storybook/openapi-msw";

import { agentAlwaysOn, buildAgentTeam } from "../../lib/agent-team";
import {
  TEAM_WORKSPACES,
  activityFixture,
  crewInstances,
  crewItems,
  teamInstances,
  teamItems,
} from "../../mocks/team-fixtures";
import { AgentTeamPage } from "../agent-team/agent-team-page";

const members = buildAgentTeam({
  items: teamItems(),
  instances: teamInstances(),
  workspaces: TEAM_WORKSPACES,
});

const meta: Meta<typeof AgentTeamPage> = {
  title: "systems/agent/components/AgentTeamPage",
  component: AgentTeamPage,
  args: {
    actions: {
      add: {
        capacity: { limit: 10, active: 2, reached: false },
        onAdd: fn(),
        onSignIn: fn(),
        places: [{ id: "@user", name: "Global" }, ...TEAM_WORKSPACES],
        signedOut: new Map(),
      },
      newSessionDisabled: false,
      onNewSession: fn(),
      onOpen: fn(),
    },
    members,
    onClearFilters: fn(),
    onCreate: fn(),
    status: "ready",
    unfiltered: true,
    view: "cards",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Cards: Story = {};

export const Rows: Story = { args: { view: "rows" } };

const crew = buildAgentTeam({
  items: crewItems(),
  instances: crewInstances(),
  workspaces: TEAM_WORKSPACES,
});

/** Twenty always-on agents: the section has to stay scannable at this size. */
export const ManyAlwaysOn: Story = {
  args: { members: crew },
  parameters: {
    ...storybookMswParameters({
      agent: [
        compozyApiMock.post("/api/agents/activity", async ({ request }) => {
          const body = (await request.json()) as { items: unknown[] };
          return HttpResponse.json({ activities: body.items.map(() => activityFixture()) });
        }),
      ],
    }),
  },
};

export const ManyAlwaysOnRows: Story = { ...ManyAlwaysOn, args: { members: crew, view: "rows" } };

export const NobodyAlwaysOn: Story = {
  args: { members: members.filter(member => !agentAlwaysOn(member)) },
};

export const NothingMatches: Story = { args: { status: "filtered-empty", unfiltered: false } };

export const FirstRun: Story = { args: { members: [], status: "first-run-empty" } };
