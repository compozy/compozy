import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { agentStaysOn, buildAgentTeam } from "../../lib/agent-team";
import { TEAM_WORKSPACES, teamInstances, teamItems } from "../../mocks/team-fixtures";
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
        capped: false,
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

export const NobodyStaysOn: Story = {
  args: { members: members.filter(member => !agentStaysOn(member)) },
};

export const NothingMatches: Story = { args: { status: "filtered-empty", unfiltered: false } };

export const FirstRun: Story = { args: { members: [], status: "first-run-empty" } };
