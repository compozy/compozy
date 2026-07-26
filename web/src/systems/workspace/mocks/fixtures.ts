import { FIXTURE_AGENT_DEFINITION_DIGEST } from "@/systems/agent/mocks";
import type { WorkspaceDetailPayload, WorkspacePayload } from "@/systems/workspace/types";
import {
  storyAgentNames,
  storyChannels,
  storyDefaultWorkspaceId,
  storySessionIds,
  storySkillNames,
  storyWorkspaceIds,
  storyWorkspaceNames,
  storyWorkspacePaths,
  storyWorkspaceSkillDir,
} from "@/storybook/fintech-scenario";
import { buildLiveNetworkParticipationFixture } from "@/test/network-participation-fixtures";

function liveParticipation(workspaceId: string, channelId: string) {
  return buildLiveNetworkParticipationFixture({ workspaceId, channelId });
}

export const workspaceFixtures: WorkspacePayload[] = [
  {
    id: storyWorkspaceIds.hq,
    root_dir: storyWorkspacePaths.hq,
    add_dirs: [
      storyWorkspacePaths.sharedLaunch,
      storyWorkspacePaths.sharedAnalytics,
      storyWorkspacePaths.sharedPolicies,
    ],
    name: storyWorkspaceNames.hq,
    created_at: "2026-04-14T09:00:00Z",
    updated_at: "2026-04-17T18:16:00Z",
  },
  {
    id: storyWorkspaceIds.product,
    root_dir: storyWorkspacePaths.product,
    add_dirs: [storyWorkspacePaths.sharedLaunch, storyWorkspacePaths.sharedCampaigns],
    name: storyWorkspaceNames.product,
    created_at: "2026-04-12T09:00:00Z",
    updated_at: "2026-04-17T17:58:00Z",
  },
  {
    id: storyWorkspaceIds.growth,
    root_dir: storyWorkspacePaths.growth,
    add_dirs: [storyWorkspacePaths.sharedCampaigns, storyWorkspacePaths.sharedAnalytics],
    name: storyWorkspaceNames.growth,
    created_at: "2026-04-11T09:00:00Z",
    updated_at: "2026-04-17T17:54:00Z",
  },
  {
    id: storyWorkspaceIds.platform,
    root_dir: storyWorkspacePaths.platform,
    add_dirs: [storyWorkspacePaths.sharedLaunch, storyWorkspacePaths.sharedPolicies],
    name: storyWorkspaceNames.platform,
    created_at: "2026-04-10T09:00:00Z",
    updated_at: "2026-04-17T18:05:00Z",
  },
  {
    id: storyWorkspaceIds.finance,
    root_dir: storyWorkspacePaths.finance,
    add_dirs: [storyWorkspacePaths.sharedAnalytics, storyWorkspacePaths.sharedLaunch],
    name: storyWorkspaceNames.finance,
    created_at: "2026-04-10T12:00:00Z",
    updated_at: "2026-04-17T18:02:00Z",
  },
  {
    id: storyWorkspaceIds.support,
    root_dir: storyWorkspacePaths.support,
    add_dirs: [storyWorkspacePaths.sharedPolicies, storyWorkspacePaths.sharedLaunch],
    name: storyWorkspaceNames.support,
    created_at: "2026-04-09T09:00:00Z",
    updated_at: "2026-04-17T17:49:00Z",
  },
  {
    id: storyWorkspaceIds.risk,
    root_dir: storyWorkspacePaths.risk,
    add_dirs: [storyWorkspacePaths.sharedPolicies, storyWorkspacePaths.sharedAnalytics],
    name: storyWorkspaceNames.risk,
    created_at: "2026-04-08T09:00:00Z",
    updated_at: "2026-04-17T17:46:00Z",
  },
];

export const primaryWorkspaceFixture: WorkspacePayload =
  workspaceFixtures.find(workspace => workspace.id === storyDefaultWorkspaceId) ??
  workspaceFixtures[0]!;

export const workspaceDetailFixture: WorkspaceDetailPayload = {
  workspace: primaryWorkspaceFixture,
  agents: [
    {
      name: storyAgentNames.cto,
      provider: "claude",
      prompt:
        "Own launch command and consolidate cross-functional launch risk into operator briefings.",
      origin: "workspace",
      workspace_id: primaryWorkspaceFixture.id,
      definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
    },
    {
      name: storyAgentNames.cfo,
      provider: "claude",
      prompt: "Track launch GMV, burn, reserve exposure, and finance approvals in real time.",
      origin: "workspace",
      workspace_id: primaryWorkspaceFixture.id,
      definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
    },
    {
      name: storyAgentNames.product,
      provider: "gemini",
      prompt:
        "Coordinate the launch checklist, unblock decision-makers, and manage the final go-live sequence.",
      origin: "workspace",
      workspace_id: primaryWorkspaceFixture.id,
      definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
    },
    {
      name: storyAgentNames.frontend,
      provider: "codex",
      prompt:
        "QA launch UI surfaces, patch visual regressions, and protect conversion-critical flows.",
      origin: "workspace",
      workspace_id: primaryWorkspaceFixture.id,
      definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
    },
    {
      name: storyAgentNames.marketing,
      provider: "gemini",
      prompt:
        "Sequence launch messaging, CRM sends, ads, and campaign timing across every merchant audience.",
      origin: "workspace",
      workspace_id: primaryWorkspaceFixture.id,
      definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
    },
    {
      name: storyAgentNames.copywriter,
      provider: "claude",
      prompt:
        "Polish launch headlines, pricing claims, lifecycle copy, and support-safe fallback language.",
      origin: "workspace",
      workspace_id: primaryWorkspaceFixture.id,
      definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
    },
    {
      name: storyAgentNames.support,
      provider: "claude",
      prompt:
        "Handle launch-day merchant questions, cluster escalations, and prepare operator-safe responses.",
      origin: "workspace",
      workspace_id: primaryWorkspaceFixture.id,
      definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
    },
    {
      name: storyAgentNames.fraud,
      provider: "claude",
      prompt:
        "Review payout holds, reserve anomalies, and launch-day fraud spikes before merchants are unblocked.",
      origin: "workspace",
      workspace_id: primaryWorkspaceFixture.id,
      definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
    },
    {
      name: storyAgentNames.compliance,
      provider: "qwen-code",
      prompt:
        "Verify claims, policy exceptions, sanctions screens, and KYB evidence before approval.",
      origin: "workspace",
      workspace_id: primaryWorkspaceFixture.id,
      definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
    },
    {
      name: storyAgentNames.release,
      provider: "codex",
      prompt:
        "Run canary verification, rollback guardrails, and launch-readiness checks across payment services.",
      origin: "workspace",
      workspace_id: primaryWorkspaceFixture.id,
      definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
    },
    {
      name: storyAgentNames.platform,
      provider: "codex",
      prompt:
        "Investigate webhook drift, partner integrations, and platform stability during launch.",
      origin: "workspace",
      workspace_id: primaryWorkspaceFixture.id,
      definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
    },
  ],
  providers: [
    {
      name: "claude",
      display_name: "Claude Code",
      harness: "acp",
      runtime_provider: "claude",
    },
    {
      name: "codex",
      display_name: "Codex",
      harness: "acp",
      runtime_provider: "codex",
    },
    {
      name: "gemini",
      display_name: "Gemini CLI",
      harness: "acp",
      runtime_provider: "gemini",
    },
    {
      name: "qwen-code",
      display_name: "Qwen Code",
      harness: "acp",
      runtime_provider: "qwen-code",
    },
    {
      name: "openrouter",
      display_name: "OpenRouter",
      harness: "pi_acp",
      runtime_provider: "openrouter",
    },
    { name: "cline", display_name: "Cline", harness: "acp", runtime_provider: "cline" },
    { name: "hermes", display_name: "Hermes", harness: "acp", runtime_provider: "hermes" },
    { name: "goose", display_name: "Goose", harness: "acp", runtime_provider: "goose" },
    {
      name: "openhands",
      display_name: "OpenHands",
      harness: "acp",
      runtime_provider: "openhands",
    },
    {
      name: "junie",
      display_name: "Junie",
      harness: "acp",
      runtime_provider: "junie",
    },
  ],
  sessions: [
    {
      id: storySessionIds.product,
      name: "Launch room command brief",
      agent_name: storyAgentNames.product,
      resolved_network_participation: liveParticipation(
        primaryWorkspaceFixture.id,
        storyChannels.launchWarRoom
      ),
      provider: "gemini",
      workspace_id: primaryWorkspaceFixture.id,
      workspace_path: primaryWorkspaceFixture.root_dir,
      state: "active",
      badge: "idle",
      attachable: true,
      available_commands: [],
      created_at: "2026-04-17T11:20:00Z",
      updated_at: "2026-04-17T18:14:00Z",
    },
    {
      id: storySessionIds.frontend,
      name: "Landing page launch QA",
      agent_name: storyAgentNames.frontend,
      resolved_network_participation: liveParticipation(
        primaryWorkspaceFixture.id,
        storyChannels.landingPage
      ),
      provider: "codex",
      workspace_id: primaryWorkspaceFixture.id,
      workspace_path: primaryWorkspaceFixture.root_dir,
      state: "active",
      badge: "idle",
      attachable: true,
      available_commands: [],
      created_at: "2026-04-17T12:00:00Z",
      updated_at: "2026-04-17T18:09:00Z",
    },
    {
      id: storySessionIds.cto,
      name: "Executive launch review",
      agent_name: storyAgentNames.cto,
      resolved_network_participation: liveParticipation(
        primaryWorkspaceFixture.id,
        storyChannels.execSignal
      ),
      provider: "claude",
      workspace_id: primaryWorkspaceFixture.id,
      workspace_path: primaryWorkspaceFixture.root_dir,
      state: "active",
      badge: "idle",
      attachable: true,
      available_commands: [],
      created_at: "2026-04-17T10:10:00Z",
      updated_at: "2026-04-17T18:11:00Z",
    },
    {
      id: storySessionIds.cfo,
      name: "Launch revenue watch",
      agent_name: storyAgentNames.cfo,
      resolved_network_participation: liveParticipation(
        primaryWorkspaceFixture.id,
        storyChannels.financeWatch
      ),
      provider: "claude",
      workspace_id: primaryWorkspaceFixture.id,
      workspace_path: primaryWorkspaceFixture.root_dir,
      state: "active",
      badge: "idle",
      attachable: true,
      available_commands: [],
      created_at: "2026-04-17T10:30:00Z",
      updated_at: "2026-04-17T18:13:00Z",
    },
    {
      id: storySessionIds.marketing,
      name: "CRM launch timing",
      agent_name: storyAgentNames.marketing,
      resolved_network_participation: liveParticipation(
        primaryWorkspaceFixture.id,
        storyChannels.growthLaunch
      ),
      provider: "gemini",
      workspace_id: primaryWorkspaceFixture.id,
      workspace_path: primaryWorkspaceFixture.root_dir,
      state: "stopped",
      badge: "stopped",
      attachable: false,
      available_commands: [],
      created_at: "2026-04-17T09:45:00Z",
      updated_at: "2026-04-17T17:58:00Z",
    },
    {
      id: storySessionIds.copywriter,
      name: "Headline claim polish",
      agent_name: storyAgentNames.copywriter,
      resolved_network_participation: liveParticipation(
        primaryWorkspaceFixture.id,
        storyChannels.landingPage
      ),
      provider: "claude",
      workspace_id: primaryWorkspaceFixture.id,
      workspace_path: primaryWorkspaceFixture.root_dir,
      state: "active",
      badge: "idle",
      attachable: true,
      available_commands: [],
      created_at: "2026-04-17T14:05:00Z",
      updated_at: "2026-04-17T18:06:00Z",
    },
    {
      id: storySessionIds.release,
      name: "Release control canary",
      agent_name: storyAgentNames.release,
      resolved_network_participation: liveParticipation(
        primaryWorkspaceFixture.id,
        storyChannels.releaseControl
      ),
      provider: "codex",
      workspace_id: primaryWorkspaceFixture.id,
      workspace_path: primaryWorkspaceFixture.root_dir,
      state: "active",
      badge: "idle",
      attachable: true,
      available_commands: [],
      created_at: "2026-04-17T09:15:00Z",
      updated_at: "2026-04-17T18:03:00Z",
    },
  ],
  skills: [
    {
      name: storySkillNames.executiveBrief,
      source: "workspace",
      dir: storyWorkspaceSkillDir(storySkillNames.executiveBrief),
    },
    {
      name: storySkillNames.frontendQa,
      source: "workspace",
      dir: storyWorkspaceSkillDir(storySkillNames.frontendQa, storyWorkspacePaths.product),
    },
    {
      name: storySkillNames.launchCopy,
      source: "workspace",
      dir: storyWorkspaceSkillDir(storySkillNames.launchCopy, storyWorkspacePaths.growth),
    },
    {
      name: storySkillNames.financePrep,
      source: "workspace",
      dir: storyWorkspaceSkillDir(storySkillNames.financePrep, storyWorkspacePaths.finance),
    },
    {
      name: storySkillNames.merchantEscalation,
      source: "workspace",
      dir: storyWorkspaceSkillDir(storySkillNames.merchantEscalation, storyWorkspacePaths.support),
    },
  ],
};
