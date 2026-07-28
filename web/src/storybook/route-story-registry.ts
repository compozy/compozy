import type { FileRouteTypes } from "@/routeTree.gen";

import {
  storyAgentNames,
  storyDefaultWorkspaceId,
  storyHeroNetworkChannel,
  storySessionIds,
} from "./fintech-scenario";

export type GeneratedRoutePath = FileRouteTypes["fullPaths"];

export interface RouteStoryRegistryEntry {
  system: string;
  routePath: GeneratedRoutePath;
  storybookPath: string;
  title: `systems/${string}/routes/${string}`;
  storyName: string;
}

export interface RouteStoryExclusion {
  routePath: GeneratedRoutePath;
  reason: string;
}

const storyTaskId = "task_001";
const storyTaskRunId = "run_001";
const storyLoopName = "software-delivery";
const storyLoopRunId = "looprun_running";
const storyThreadId = "thread_launch_command";
const storyDirectId = "direct_story_launch_corridor";
const storyBridgeId = "brg_launch_room";

const storyNetworkBasePath = `/network/${storyDefaultWorkspaceId}/${storyHeroNetworkChannel}`;

export const routeStoryExclusions: RouteStoryExclusion[] = [];

export const routeStoryRegistry = [
  {
    system: "design-system",
    routePath: "/design-system",
    storybookPath: "/design-system",
    title: "systems/design-system/routes/DesignSystem",
    storyName: "Default",
  },
  {
    system: "runtime",
    routePath: "/",
    storybookPath: "/",
    title: "systems/runtime/routes/Home",
    storyName: "Default",
  },
  {
    system: "vault",
    routePath: "/vault",
    storybookPath: "/vault",
    title: "systems/vault/routes/Vault",
    storyName: "Default",
  },
  {
    system: "marketplace",
    routePath: "/marketplace",
    storybookPath: "/marketplace",
    title: "systems/marketplace/routes/Marketplace",
    storyName: "SkillsMarketplace",
  },
  {
    system: "marketplace",
    routePath: "/marketplace/",
    storybookPath: "/marketplace/",
    title: "systems/marketplace/routes/Marketplace",
    storyName: "SkillsMarketplace",
  },
  {
    system: "marketplace",
    routePath: "/marketplace/$kind/$entryId",
    storybookPath: "/marketplace/skill/git-flow",
    title: "systems/marketplace/routes/Marketplace",
    storyName: "DetailSkillInstalled",
  },
  {
    system: "automation",
    routePath: "/triggers",
    storybookPath: "/triggers",
    title: "systems/automation/routes/Triggers",
    storyName: "Default",
  },
  {
    system: "tasks",
    routePath: "/tasks",
    storybookPath: "/tasks",
    title: "systems/tasks/routes/Tasks",
    storyName: "DefaultList",
  },
  {
    system: "marketplace",
    routePath: "/marketplace/skills",
    storybookPath: "/marketplace/skills?tab=installed",
    title: "systems/marketplace/routes/Marketplace",
    storyName: "SkillsInstalled",
  },
  {
    system: "marketplace",
    routePath: "/marketplace/mcps",
    storybookPath: "/marketplace/mcps",
    title: "systems/marketplace/routes/Marketplace",
    storyName: "McpsMarketplace",
  },
  {
    system: "marketplace",
    routePath: "/marketplace/bundles",
    storybookPath: "/marketplace/bundles",
    title: "systems/marketplace/routes/Marketplace",
    storyName: "BundlesMarketplace",
  },
  {
    system: "settings",
    routePath: "/settings",
    storybookPath: "/settings",
    title: "systems/settings/routes/SettingsShell",
    storyName: "Shell",
  },
  {
    system: "settings",
    routePath: "/settings/",
    storybookPath: "/settings/",
    title: "systems/settings/routes/SettingsShell",
    storyName: "IndexRedirect",
  },
  {
    system: "settings",
    routePath: "/sandbox",
    storybookPath: "/sandbox",
    title: "systems/settings/routes/Sandbox",
    storyName: "Default",
  },
  {
    system: "network",
    routePath: "/network",
    storybookPath: "/network",
    title: "systems/network/routes/Network",
    storyName: "Overview",
  },
  {
    system: "loops",
    routePath: "/loops",
    storybookPath: "/loops",
    title: "systems/loops/routes/Loops",
    storyName: "Catalog",
  },
  {
    system: "loops",
    routePath: "/loop-runs",
    storybookPath: "/loop-runs",
    title: "systems/loops/routes/LoopRuns",
    storyName: "RunsList",
  },
  {
    system: "knowledge",
    routePath: "/knowledge",
    storybookPath: "/knowledge",
    title: "systems/knowledge/routes/Knowledge",
    storyName: "Default",
  },
  {
    system: "automation",
    routePath: "/jobs",
    storybookPath: "/jobs",
    title: "systems/automation/routes/Jobs",
    storyName: "Default",
  },
  {
    system: "automation",
    routePath: "/jobs/$jobId",
    storybookPath: "/jobs/job_launch_command_digest",
    title: "systems/automation/routes/Jobs",
    storyName: "JobDetail",
  },
  {
    system: "automation",
    routePath: "/triggers/$triggerId",
    storybookPath: "/triggers/trg_support_sla_breach",
    title: "systems/automation/routes/Triggers",
    storyName: "TriggerDetail",
  },
  {
    system: "bridges",
    routePath: "/bridges",
    storybookPath: "/bridges",
    title: "systems/bridges/routes/Bridges",
    storyName: "Default",
  },
  {
    system: "bridges",
    routePath: "/bridges/$id",
    storybookPath: `/bridges/${storyBridgeId}`,
    title: "systems/bridges/routes/Bridges",
    storyName: "TestDelivery",
  },
  {
    system: "tasks",
    routePath: "/tasks/new",
    storybookPath: "/tasks/new",
    title: "systems/tasks/routes/TaskCreate",
    storyName: "Default",
  },
  {
    system: "tasks",
    routePath: "/tasks/$id",
    storybookPath: `/tasks/${storyTaskId}`,
    title: "systems/tasks/routes/TaskDetail",
    storyName: "Overview",
  },
  {
    system: "settings",
    routePath: "/settings/skills",
    storybookPath: "/settings/skills",
    title: "systems/settings/routes/SettingsSkills",
    storyName: "Default",
  },
  {
    system: "settings",
    routePath: "/settings/providers",
    storybookPath: "/settings/providers",
    title: "systems/settings/routes/SettingsProviders",
    storyName: "Default",
  },
  {
    system: "settings",
    routePath: "/settings/observability",
    storybookPath: "/settings/observability",
    title: "systems/settings/routes/SettingsObservability",
    storyName: "Default",
  },
  {
    system: "settings",
    routePath: "/settings/network",
    storybookPath: "/settings/network",
    title: "systems/settings/routes/SettingsNetwork",
    storyName: "Default",
  },
  {
    system: "settings",
    routePath: "/settings/memory",
    storybookPath: "/settings/memory",
    title: "systems/settings/routes/SettingsMemory",
    storyName: "Default",
  },
  {
    system: "settings",
    routePath: "/settings/roles",
    storybookPath: "/settings/roles",
    title: "systems/settings/routes/SettingsRoles",
    storyName: "Populated",
  },
  {
    system: "settings",
    routePath: "/settings/hooks",
    storybookPath: "/settings/hooks",
    title: "systems/settings/routes/SettingsHooks",
    storyName: "Default",
  },
  {
    system: "settings",
    routePath: "/settings/extensions",
    storybookPath: "/settings/extensions",
    title: "systems/settings/routes/SettingsExtensions",
    storyName: "Default",
  },
  {
    system: "marketplace",
    routePath: "/marketplace/extensions",
    storybookPath: "/marketplace/extensions?tab=installed",
    title: "systems/marketplace/routes/Marketplace",
    storyName: "ExtensionsInstalled",
  },
  {
    system: "marketplace",
    routePath: "/marketplace/bundles/activations/$id",
    storybookPath: "/marketplace/bundles/activations/activation-ops-starter",
    title: "systems/marketplace/routes/BundleActivationDetail",
    storyName: "Current",
  },
  {
    system: "settings",
    routePath: "/settings/general",
    storybookPath: "/settings/general",
    title: "systems/settings/routes/SettingsGeneral",
    storyName: "Default",
  },
  {
    system: "settings",
    routePath: "/settings/appearance",
    storybookPath: "/settings/appearance",
    title: "systems/settings/routes/SettingsAppearance",
    storyName: "Default",
  },
  {
    system: "settings",
    routePath: "/settings/layouts",
    storybookPath: "/settings/layouts",
    title: "systems/settings/routes/SettingsLayouts",
    storyName: "Default",
  },
  {
    system: "settings",
    routePath: "/settings/automation",
    storybookPath: "/settings/automation",
    title: "systems/settings/routes/SettingsAutomation",
    storyName: "Default",
  },
  {
    system: "session",
    routePath: "/session/$id",
    storybookPath: `/session/${storySessionIds.frontend}`,
    title: "systems/session/routes/SessionPermalink",
    storyName: "RedirectsToCanonicalSession",
  },
  {
    system: "loops",
    routePath: "/loops/$name",
    storybookPath: `/loops/${storyLoopName}`,
    title: "systems/loops/routes/Loops",
    storyName: "Detail",
  },
  {
    system: "loops",
    routePath: "/loop-runs/$runId",
    storybookPath: `/loop-runs/${storyLoopRunId}`,
    title: "systems/loops/routes/LoopRuns",
    storyName: "RunDetail",
  },
  {
    system: "agent",
    routePath: "/agents",
    storybookPath: "/agents",
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
    routePath: "/agents/$name/settings",
    storybookPath: `/agents/${storyAgentNames.fraud}/settings`,
    title: "systems/agent/routes/AgentSettings",
    storyName: "Basics",
  },
  {
    system: "tasks",
    routePath: "/tasks/$id/edit",
    storybookPath: `/tasks/${storyTaskId}/edit`,
    title: "systems/tasks/routes/TaskEdit",
    storyName: "Default",
  },
  {
    system: "loops",
    routePath: "/loops/$name/run",
    storybookPath: `/loops/${storyLoopName}/run`,
    title: "systems/loops/routes/Loops",
    storyName: "RunForm",
  },
  {
    system: "loops",
    routePath: "/loops/$name/editor",
    storybookPath: `/loops/${storyLoopName}/editor`,
    title: "systems/loops/routes/Loops",
    storyName: "Editor",
  },
  {
    system: "loops",
    routePath: "/loops/$name/configure",
    storybookPath: `/loops/${storyLoopName}/configure`,
    title: "systems/loops/routes/Loops",
    storyName: "Configure",
  },
  {
    system: "tasks",
    routePath: "/tasks/$id/runs/$runId",
    storybookPath: `/tasks/${storyTaskId}/runs/${storyTaskRunId}`,
    title: "systems/tasks/routes/TaskRunDetail",
    storyName: "Running",
  },
  {
    system: "network",
    routePath: "/network/$workspaceId/$channel/threads",
    storybookPath: `${storyNetworkBasePath}/threads`,
    title: "systems/network/routes/Network",
    storyName: "ThreadsTab",
  },
  {
    system: "network",
    routePath: "/network/$workspaceId/$channel/directs",
    storybookPath: `${storyNetworkBasePath}/directs`,
    title: "systems/network/routes/Network",
    storyName: "DirectsTab",
  },
  {
    system: "network",
    routePath: "/network/$workspaceId/$channel/activity",
    storybookPath: `${storyNetworkBasePath}/activity`,
    title: "systems/network/routes/Network",
    storyName: "ActivityTab",
  },
  {
    system: "session",
    routePath: "/agents/$name/sessions/$id",
    storybookPath: `/agents/${storyAgentNames.fraud}/sessions/${storySessionIds.fraud}`,
    title: "systems/session/routes/AgentSession",
    storyName: "Default",
  },
  {
    system: "network",
    routePath: "/network/$workspaceId/$channel/threads/$threadId",
    storybookPath: `${storyNetworkBasePath}/threads/${storyThreadId}`,
    title: "systems/network/routes/NetworkDetail",
    storyName: "ThreadDetail",
  },
  {
    system: "network",
    routePath: "/network/$workspaceId/$channel/directs/$directId",
    storybookPath: `${storyNetworkBasePath}/directs/${storyDirectId}`,
    title: "systems/network/routes/NetworkDetail",
    storyName: "DirectDetail",
  },
] satisfies RouteStoryRegistryEntry[];

export const routeStoryRegistryByRoutePath = new Map<GeneratedRoutePath, RouteStoryRegistryEntry>(
  routeStoryRegistry.map(entry => [entry.routePath, entry])
);

export const routeStorySystems = Array.from(
  new Set(routeStoryRegistry.map(entry => entry.system))
).sort();
