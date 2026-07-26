import type {
  PermissionRequest,
  SessionApprovalResponse,
  SessionEventPayload,
  SessionPayload,
  SessionRepairPayload,
  TranscriptMessage,
  TurnHistoryPayload,
  UIMessage,
} from "@/systems/session/types";
import {
  storyAgentNames,
  storyChannels,
  storySessionIds,
  storySkillNames,
  storyWorkspaceIds,
  storyWorkspacePaths,
  storyWorkspaceSkillDir,
} from "@/storybook/fintech-scenario";
import { buildLiveNetworkParticipationFixture } from "@/test/network-participation-fixtures";

function liveParticipation(workspaceId: string, channelId: string) {
  return buildLiveNetworkParticipationFixture({ workspaceId, channelId });
}

export const sessionFixtures: SessionPayload[] = [
  {
    id: storySessionIds.frontend,
    name: "Landing page launch QA",
    agent_name: storyAgentNames.frontend,
    provider: "codex",
    workspace_id: storyWorkspaceIds.product,
    workspace_path: storyWorkspacePaths.product,
    state: "active",
    badge: "running",
    attachable: true,
    available_commands: [],
    resolved_network_participation: liveParticipation(
      storyWorkspaceIds.product,
      storyChannels.landingPage
    ),
    lineage: {
      parent_session_id: storySessionIds.product,
      root_session_id: storySessionIds.product,
      spawn_depth: 1,
      spawn_role: "worker",
      ttl_expires_at: "2026-04-17T20:00:00Z",
      auto_stop_on_parent: true,
      spawn_budget: {
        max_children: 4,
        max_depth: 1,
        ttl_seconds: 7200,
      },
      permission_policy: {
        tools: ["bash", "read", "edit", "grep"],
        skills: [storySkillNames.frontendQa],
        mcp_servers: [],
        workspace_paths: [storyWorkspacePaths.product, storyWorkspacePaths.hq],
        network_channels: [storyChannels.landingPage, storyChannels.launchWarRoom],
        sandbox_profiles: [],
      },
    },
    created_at: "2026-04-17T12:00:00Z",
    updated_at: "2026-04-17T18:09:00Z",
    acp_caps: {
      supports_load_session: true,
      supported_modes: ["chat"],
    },
  },
  {
    id: storySessionIds.cto,
    name: "Executive launch review",
    agent_name: storyAgentNames.cto,
    provider: "claude",
    workspace_id: storyWorkspaceIds.hq,
    workspace_path: storyWorkspacePaths.hq,
    state: "active",
    badge: "running",
    attachable: true,
    available_commands: [],
    resolved_network_participation: liveParticipation(
      storyWorkspaceIds.hq,
      storyChannels.execSignal
    ),
    created_at: "2026-04-17T10:10:00Z",
    updated_at: "2026-04-17T18:11:00Z",
  },
  {
    id: storySessionIds.cfo,
    name: "Launch revenue watch",
    agent_name: storyAgentNames.cfo,
    provider: "claude",
    workspace_id: storyWorkspaceIds.finance,
    workspace_path: storyWorkspacePaths.finance,
    state: "active",
    badge: "running",
    attachable: true,
    available_commands: [],
    resolved_network_participation: liveParticipation(
      storyWorkspaceIds.finance,
      storyChannels.financeWatch
    ),
    created_at: "2026-04-17T10:30:00Z",
    updated_at: "2026-04-17T18:13:00Z",
  },
  {
    id: storySessionIds.product,
    name: "Launch room command brief",
    agent_name: storyAgentNames.product,
    provider: "gemini",
    workspace_id: storyWorkspaceIds.hq,
    workspace_path: storyWorkspacePaths.hq,
    state: "active",
    badge: "running",
    attachable: true,
    available_commands: [],
    resolved_network_participation: liveParticipation(
      storyWorkspaceIds.hq,
      storyChannels.launchWarRoom
    ),
    created_at: "2026-04-17T11:20:00Z",
    updated_at: "2026-04-17T18:14:00Z",
  },
  {
    id: storySessionIds.marketing,
    name: "CRM launch timing",
    agent_name: storyAgentNames.marketing,
    provider: "gemini",
    workspace_id: storyWorkspaceIds.growth,
    workspace_path: storyWorkspacePaths.growth,
    state: "stopped",
    badge: "stopped",
    attachable: false,
    available_commands: [],
    resolved_network_participation: liveParticipation(
      storyWorkspaceIds.growth,
      storyChannels.growthLaunch
    ),
    created_at: "2026-04-17T09:45:00Z",
    updated_at: "2026-04-17T17:58:00Z",
  },
  {
    id: storySessionIds.copywriter,
    name: "Headline claim polish",
    agent_name: storyAgentNames.copywriter,
    provider: "claude",
    workspace_id: storyWorkspaceIds.growth,
    workspace_path: storyWorkspacePaths.growth,
    state: "active",
    badge: "running",
    attachable: true,
    available_commands: [],
    resolved_network_participation: liveParticipation(
      storyWorkspaceIds.growth,
      storyChannels.landingPage
    ),
    created_at: "2026-04-17T14:05:00Z",
    updated_at: "2026-04-17T18:06:00Z",
  },
  {
    id: storySessionIds.support,
    name: "Launch support swarm",
    agent_name: storyAgentNames.support,
    provider: "claude",
    workspace_id: storyWorkspaceIds.support,
    workspace_path: storyWorkspacePaths.support,
    state: "active",
    badge: "running",
    attachable: true,
    available_commands: [],
    resolved_network_participation: liveParticipation(
      storyWorkspaceIds.support,
      storyChannels.supportSwarm
    ),
    created_at: "2026-04-17T13:00:00Z",
    updated_at: "2026-04-17T18:08:00Z",
  },
  {
    id: storySessionIds.fraud,
    name: "Reserve spike monitor",
    agent_name: storyAgentNames.fraud,
    provider: "claude",
    workspace_id: storyWorkspaceIds.risk,
    workspace_path: storyWorkspacePaths.risk,
    state: "active",
    badge: "running",
    attachable: true,
    available_commands: [],
    resolved_network_participation: liveParticipation(
      storyWorkspaceIds.risk,
      storyChannels.riskOps
    ),
    created_at: "2026-04-17T10:45:00Z",
    updated_at: "2026-04-17T18:07:00Z",
  },
  {
    id: storySessionIds.compliance,
    name: "Claim compliance review",
    agent_name: storyAgentNames.compliance,
    provider: "qwen-code",
    workspace_id: storyWorkspaceIds.risk,
    workspace_path: storyWorkspacePaths.risk,
    state: "active",
    badge: "running",
    attachable: true,
    available_commands: [],
    resolved_network_participation: liveParticipation(
      storyWorkspaceIds.risk,
      storyChannels.launchWarRoom
    ),
    created_at: "2026-04-17T12:25:00Z",
    updated_at: "2026-04-17T18:04:00Z",
  },
  {
    id: storySessionIds.release,
    name: "Release control canary",
    agent_name: storyAgentNames.release,
    provider: "codex",
    workspace_id: storyWorkspaceIds.platform,
    workspace_path: storyWorkspacePaths.platform,
    state: "active",
    badge: "running",
    attachable: true,
    available_commands: [],
    resolved_network_participation: liveParticipation(
      storyWorkspaceIds.platform,
      storyChannels.releaseControl
    ),
    created_at: "2026-04-17T09:15:00Z",
    updated_at: "2026-04-17T18:03:00Z",
  },
  {
    id: storySessionIds.platform,
    name: "Partner webhook stability",
    agent_name: storyAgentNames.platform,
    provider: "codex",
    workspace_id: storyWorkspaceIds.platform,
    workspace_path: storyWorkspacePaths.platform,
    state: "active",
    badge: "running",
    attachable: true,
    available_commands: [],
    resolved_network_participation: liveParticipation(
      storyWorkspaceIds.platform,
      storyChannels.partnerSync
    ),
    created_at: "2026-04-17T09:05:00Z",
    updated_at: "2026-04-17T18:01:00Z",
  },
];

export const primarySessionFixture: SessionPayload = sessionFixtures[0]!;

export const sessionEventsFixture: SessionEventPayload[] = [
  {
    id: "event_001",
    agent_name: primarySessionFixture.agent_name,
    content: {
      text: "Checking hero CTA copy, launch banner fallback states, and mobile layout drift before 18:30 UTC.",
    },
    sequence: 1,
    session_id: primarySessionFixture.id,
    timestamp: "2026-04-17T16:01:00Z",
    turn_id: "turn_001",
    type: "message.created",
    workspace_id: primarySessionFixture.workspace_id,
    workspace_path: primarySessionFixture.workspace_path,
    parent_session_id: primarySessionFixture.lineage?.parent_session_id,
    root_session_id: primarySessionFixture.lineage?.root_session_id,
    spawn_depth: primarySessionFixture.lineage?.spawn_depth ?? 0,
  },
  {
    id: "event_002",
    agent_name: primarySessionFixture.agent_name,
    content: {
      tool_name: "Read",
      file_path: storyWorkspaceSkillDir(storySkillNames.frontendQa, storyWorkspacePaths.product),
    },
    sequence: 2,
    session_id: primarySessionFixture.id,
    timestamp: "2026-04-17T16:02:00Z",
    turn_id: "turn_001",
    type: "tool.called",
    workspace_id: primarySessionFixture.workspace_id,
    workspace_path: primarySessionFixture.workspace_path,
    parent_session_id: primarySessionFixture.lineage?.parent_session_id,
    root_session_id: primarySessionFixture.lineage?.root_session_id,
    spawn_depth: primarySessionFixture.lineage?.spawn_depth ?? 0,
  },
];

export const sessionHistoryFixture: TurnHistoryPayload[] = [
  {
    turn_id: "turn_001",
    events: sessionEventsFixture,
  },
];

export const sessionRepairFixture: SessionRepairPayload = {
  session_id: primarySessionFixture.id,
  issues: [],
  actions: [
    {
      code: "append_terminal_error",
      turn_id: "turn_001",
      event_id: "event_repair_001",
      persisted: true,
    },
  ],
  persisted: true,
};

export const bashToolMessageFixture: UIMessage = {
  id: "tool_bash",
  role: "tool_call",
  content: "",
  toolName: "Bash",
  toolInput: {
    command: "bun run --cwd apps/launch-site test -- --run hero-banner",
  },
  toolResult: {
    stdout: "Running launch-site tests\nhero-banner passed\npricing-banner passed\n",
  },
  timestamp: Date.parse("2026-04-17T16:04:00Z"),
};

export const runningBashToolMessageFixture: UIMessage = {
  ...bashToolMessageFixture,
  id: "tool_bash_running",
  toolResult: undefined,
};

export const longBashToolMessageFixture: UIMessage = {
  ...bashToolMessageFixture,
  id: "tool_bash_long",
  toolResult: {
    stdout: Array.from({ length: 240 }, (_, index) => `launch check line ${index + 1}`).join("\n"),
  },
};

export const errorToolMessageFixture: UIMessage = {
  ...bashToolMessageFixture,
  id: "tool_bash_error",
  toolError: true,
  toolResult: {
    stderr: "hero-banner visual diff exceeded threshold\nexit status 1\n",
    error: "Command failed with exit status 1",
  },
};

export const editToolMessageFixture: UIMessage = {
  id: "tool_edit",
  role: "tool_call",
  content: "",
  toolName: "Edit",
  toolInput: {
    file_path: "apps/launch-site/src/components/hero-banner.tsx",
    old_string: 'const heroHeadline = "Move money without enterprise drag";',
    new_string: 'const heroHeadline = "Launch checkout in days, not quarters";',
  },
  toolResult: {
    content: "Applied patch successfully.",
  },
  timestamp: Date.parse("2026-04-17T16:05:00Z"),
};

export const multiHunkEditToolMessageFixture: UIMessage = {
  ...editToolMessageFixture,
  id: "tool_edit_multi_hunk",
  toolInput: {
    file_path: "apps/launch-site/src/routes/home.tsx",
    old_string: [
      "@@ -18,7 +18,7 @@",
      '-const heroSubhead = "Accept cards with no surprise fees.";',
      "",
      "@@ -42,4 +42,6 @@",
      "-export const showLaunchFallbackBanner = false;",
    ].join("\n"),
    new_string: [
      "@@ -18,7 +18,7 @@",
      '+const heroSubhead = "Predictable processing for launch teams shipping across LATAM.";',
      "",
      "@@ -42,4 +42,6 @@",
      "+export const showLaunchFallbackBanner = true;",
    ].join("\n"),
  },
};

export const readToolMessageFixture: UIMessage = {
  id: "tool_read",
  role: "tool_call",
  content: "",
  toolName: "Read",
  toolInput: {
    file_path: storyWorkspaceSkillDir(storySkillNames.frontendQa, storyWorkspacePaths.product),
  },
  toolResult: {
    stdout:
      "# Frontend Launch QA\n\nVerify hero copy, fallback banners, pricing claims, and mobile checkout spacing before cutover.\n",
  },
  timestamp: Date.parse("2026-04-17T16:06:00Z"),
};

export const truncatedReadToolMessageFixture: UIMessage = {
  ...readToolMessageFixture,
  id: "tool_read_large",
  toolInput: {
    file_path: "apps/launch-site/src/generated/launch-copy.d.ts",
  },
  toolResult: {
    stdout: Array.from({ length: 180 }, (_, index) => `type LaunchLine${index + 1} = string;`).join(
      "\n"
    ),
  },
};

export const searchToolMessageFixture: UIMessage = {
  id: "tool_search",
  role: "tool_call",
  content: "",
  toolName: "Grep",
  toolInput: {
    pattern: "launchBanner",
    glob: "**/*.tsx",
  },
  toolResult: {
    stdout:
      "apps/launch-site/src/components/hero-banner.tsx\napps/launch-site/src/components/pricing-banner.tsx",
  },
  timestamp: Date.parse("2026-04-17T16:07:00Z"),
};

export const emptySearchToolMessageFixture: UIMessage = {
  ...searchToolMessageFixture,
  id: "tool_search_empty",
  toolResult: {
    stdout: "",
  },
};

export const writeToolMessageFixture: UIMessage = {
  id: "tool_write",
  role: "tool_call",
  content: "",
  toolName: "Write",
  toolInput: {
    file_path: "apps/launch-site/tmp/launch-qa-notes.md",
    content: "# Launch QA Notes\n\nCollected the remaining launch blockers.",
  },
  toolResult: {
    stdout: "Wrote apps/launch-site/tmp/launch-qa-notes.md",
  },
  timestamp: Date.parse("2026-04-17T16:08:00Z"),
};

export const overwriteWriteToolMessageFixture: UIMessage = {
  ...writeToolMessageFixture,
  id: "tool_write_overwrite",
  toolInput: {
    file_path: "apps/launch-site/tmp/launch-qa-notes.md",
    content: "# Launch QA Notes\n\nUpdated with the mobile fallback screenshot review.",
  },
};

export const genericToolMessageFixture: UIMessage = {
  id: "tool_generic",
  role: "tool_call",
  content: "",
  toolName: "Context7",
  toolInput: {
    library: "stripe",
    topic: "checkout launch fallback patterns",
  },
  toolResult: {
    content: "Fetched docs excerpt.",
  },
  timestamp: Date.parse("2026-04-17T16:09:00Z"),
};

export const markdownFixture = `# Launch readiness snapshot

- Hero headline now matches the approved pricing language.
- Mobile checkout spacing is clear at 360px and 390px widths.
- Remaining blocker: partner-bank timeout copy for BR merchants still needs compliance sign-off.

\`\`\`ts
const heroHeadline = "Launch checkout in days, not quarters";
\`\`\`

[Launch brief](https://ops.northstarpay.internal/launch/brief)
`;

export const userMessageFixture: UIMessage = {
  id: "msg_user_001",
  role: "user",
  content: "Summarize the launch blockers before the 18:30 UTC cutover.",
  timestamp: Date.parse("2026-04-17T16:00:00Z"),
};

export const assistantMessageFixture: UIMessage = {
  id: "msg_assistant_001",
  role: "assistant",
  content: "I am reviewing the hero banner, pricing claims, and fallback states now.",
  thinking:
    "Need the approved pricing language and the partner-bank fallback copy before closing the launch checklist.",
  thinkingComplete: true,
  timestamp: Date.parse("2026-04-17T16:01:00Z"),
};

export const streamingAssistantMessageFixture: UIMessage = {
  id: "msg_assistant_streaming",
  role: "assistant",
  content: "Drafting the launch readiness recap...",
  isStreaming: true,
  timestamp: Date.parse("2026-04-17T16:11:00Z"),
};

export const systemMessageFixture: UIMessage = {
  id: "msg_system_001",
  role: "system",
  content: "System notice: permission required to run the launch-site verification command.",
  timestamp: Date.parse("2026-04-17T16:02:00Z"),
};

export const diffMessageFixture: UIMessage = {
  id: "msg_diff_001",
  role: "diff",
  content: "",
  diff: {
    language: "diff",
    path: "apps/launch-site/src/components/pricing-banner.tsx",
    additions: 4,
    removals: 38,
    content: [
      "@@ pricing-banner.tsx @@",
      '-  return "No surprise fees.";',
      "-  if (showLaunchCredit) {",
      '-    return "Zero setup costs.";',
      "-  }",
      '+  return "Predictable processing for launch teams.";',
      "+  if (showLaunchCredit) {",
      '+    return "Launch-week credits applied at activation.";',
      "+  }",
    ].join("\n"),
  },
  timestamp: Date.parse("2026-04-17T16:12:00Z"),
};

export const uiMessageFixtures: UIMessage[] = [
  userMessageFixture,
  assistantMessageFixture,
  bashToolMessageFixture,
  {
    ...bashToolMessageFixture,
    id: "tool_bash_result",
    role: "tool_result",
  },
  {
    id: "msg_assistant_002",
    role: "assistant",
    content: markdownFixture,
    timestamp: Date.parse("2026-04-17T16:10:00Z"),
  },
];

export const sessionTranscriptFixture: TranscriptMessage[] = [
  {
    id: "transcript_user_001",
    role: "user",
    parts: [
      {
        type: "text",
        text: "Summarize the launch blockers before the 18:30 UTC cutover.",
        state: "done",
      },
    ],
  },
  {
    id: "transcript_assistant_001",
    role: "assistant",
    parts: [
      {
        type: "reasoning",
        text: "Need the approved pricing language and the partner-bank fallback copy before closing the launch checklist.",
        state: "done",
      },
      { type: "text", text: markdownFixture, state: "done" },
    ],
  },
  {
    id: "transcript_tool_001",
    role: "assistant",
    parts: [
      {
        type: "tool-Bash",
        toolCallId: "tool_bash_001",
        state: "output-available",
        input: bashToolMessageFixture.toolInput,
        output: {
          type: "tool_result",
          title: "Bash",
          raw: {
            stdout: "Running launch-site tests\nhero-banner passed\npricing-banner passed\n",
          },
        },
      },
    ],
  },
  {
    id: "transcript_tool_002",
    role: "assistant",
    parts: [
      {
        type: "tool-Read",
        toolCallId: "tool_read_001",
        state: "output-available",
        input: readToolMessageFixture.toolInput,
        output: {
          type: "tool_result",
          title: "Read",
          raw: {
            stdout:
              "# Frontend Launch QA\n\nVerify hero copy, fallback banners, pricing claims, and mobile checkout spacing before cutover.\n",
          },
        },
      },
    ],
  },
  {
    id: "transcript_assistant_002",
    role: "assistant",
    parts: [
      {
        type: "text",
        text: "Hero copy, fallback banner behavior, and mobile spacing are clean. The remaining blocker is the partner-bank timeout note for BR merchants.",
        state: "done",
      },
    ],
  },
];

export const sessionTranscriptPermissionFixture: TranscriptMessage[] = [
  ...sessionTranscriptFixture,
  {
    id: "transcript_permission_001",
    role: "assistant",
    parts: [
      {
        type: "data-agh-permission",
        data: {
          type: "permission.required",
          request_id: "perm_launch_001",
          turn_id: "turn_perm_001",
          tool_call_id: "tool_bash_perm_001",
          action: "execute",
          resource: "bun run --cwd apps/launch-site test -- --run hero-banner",
          title: "Bash",
          raw: {
            command: "bun run --cwd apps/launch-site test -- --run hero-banner",
          },
        },
      },
    ],
  },
];

export const sessionApprovalFixture: SessionApprovalResponse = {
  status: "approved",
};

export const permissionRequestFixture: PermissionRequest = {
  requestId: "perm_launch_001",
  toolName: "Bash",
  toolInput: {
    command: "bun run --cwd apps/launch-site test -- --run hero-banner",
  },
  action: "execute",
  resource: "bun run --cwd apps/launch-site test -- --run hero-banner",
};
