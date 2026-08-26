import { storyAgentNames } from "./fintech-scenario";
import type { RouteStoryRegistryEntry } from "./route-story-registry-types";

export const agentRouteStories = [
  {
    system: "agent",
    routePath: "/agents",
    storybookPath: "/agents",
    title: "systems/agent/routes/AgentsFleet",
    storyName: "Loaded",
  },
  {
    system: "agent",
    routePath: "/agents/",
    storybookPath: "/agents/",
    title: "systems/agent/routes/AgentsFleet",
    storyName: "Loaded",
  },
  {
    system: "agent",
    routePath: "/agents/$name",
    storybookPath: `/agents/${storyAgentNames.fraud}`,
    title: "systems/agent/routes/AgentDetail",
    storyName: "Default",
  },
  {
    system: "agent",
    routePath: "/agents/$name/",
    storybookPath: `/agents/${storyAgentNames.fraud}/`,
    title: "systems/agent/routes/AgentDetail",
    storyName: "Default",
  },
  {
    system: "agent",
    routePath: "/agents/$name/settings",
    storybookPath: `/agents/${storyAgentNames.fraud}/settings`,
    title: "systems/agent/routes/AgentSettings",
    storyName: "Basics",
  },
  {
    system: "agent-comms",
    routePath: "/agents/activity",
    storybookPath: "/agents/activity",
    title: "systems/agent-comms/routes/AgentsActivity",
    storyName: "Default",
  },
  {
    system: "agent-comms",
    routePath: "/agents/calls/$callId",
    storybookPath: "/agents/calls/call_01JBD8G2K7Q9",
    title: "systems/agent-comms/routes/AgentCallDetail",
    storyName: "Completed",
  },
] satisfies RouteStoryRegistryEntry[];
