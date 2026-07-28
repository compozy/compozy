import type {
  CreateNetworkChannelResponse,
  NetworkCapabilityCatalog,
  NetworkChannel,
  NetworkChannelsResponse,
  NetworkConversationMessage,
  NetworkDirectRoomDetail,
  NetworkDirectRoomMessage,
  NetworkDirectRoomSummary,
  NetworkPeerDetail,
  NetworkPeerSummary,
  NetworkStatus,
  NetworkThreadDetail,
  NetworkThreadMessage,
  NetworkThreadSummary,
  NetworkWorkDetail,
} from "../types";
import {
  storyAgentNames,
  storyChannels,
  storyHeroNetworkChannel,
  storyPeerIds,
  storySessionIds,
  storyWorkspaceIds,
  storyWorkspacePaths,
} from "@/storybook/fintech-scenario";

function makePeerCard({
  peerId,
  displayName,
  capabilities,
  artifactsSupported = ["capability"],
}: {
  peerId: string;
  displayName: string;
  capabilities: Array<{ id: string; summary: string }>;
  artifactsSupported?: string[];
}) {
  return {
    peer_id: peerId,
    display_name: displayName,
    artifacts_supported: artifactsSupported,
    capabilities,
    profiles_supported: ["default"],
    trust_modes_supported: ["local-first", "relay"],
  };
}

const primaryPeerCard = makePeerCard({
  peerId: storyPeerIds.local,
  displayName: "Northstar Launch Control",
  capabilities: [
    {
      id: "chat",
      summary:
        "Coordinates launch-room decisions across product, finance, engineering, and support.",
    },
    {
      id: "tools",
      summary:
        "Runs verification and evidence-gathering passes before go-live changes are approved.",
    },
  ],
});

const ctoPeerCard = makePeerCard({
  peerId: storyPeerIds.cto,
  displayName: "Northstar CTO Office",
  capabilities: [
    {
      id: "chat",
      summary: "Publishes technical risk calls, rollout gates, and fallback policy guidance.",
    },
  ],
});

const financePeerCard = makePeerCard({
  peerId: storyPeerIds.finance,
  displayName: "Northstar Finance Desk",
  capabilities: [
    {
      id: "chat",
      summary: "Tracks launch GMV, burn, reserve exposure, and finance sign-offs in real time.",
    },
  ],
});

const growthPeerCard = makePeerCard({
  peerId: storyPeerIds.growth,
  displayName: "Northstar Growth Desk",
  capabilities: [
    {
      id: "chat",
      summary: "Coordinates CRM sends, ad pacing, landing-page copy, and launch timing changes.",
    },
  ],
});

const supportPeerCard = makePeerCard({
  peerId: storyPeerIds.support,
  displayName: "Northstar Support Desk",
  capabilities: [
    {
      id: "chat",
      summary: "Handles launch-day merchant escalations and summarizes support queue pressure.",
    },
  ],
});

const frontendPeerCard = makePeerCard({
  peerId: storyPeerIds.frontend,
  displayName: "Northstar Frontend Desk",
  capabilities: [
    {
      id: "tools",
      summary:
        "Runs launch-page QA, diff review, and pre-flight checks for customer-facing surfaces.",
    },
  ],
});

const settlementPeerCard = makePeerCard({
  peerId: storyPeerIds.partner,
  displayName: "Partner Settlement Desk",
  capabilities: [
    {
      id: "chat",
      summary: "Confirms settlement ETA, replay state, and partner-bank evidence during launch.",
    },
  ],
});

const creativePeerCard = makePeerCard({
  peerId: storyPeerIds.creative,
  displayName: "Creative Studio Desk",
  capabilities: [
    {
      id: "chat",
      summary: "Stages paid-media creative and waits on final pricing or compliance sign-off.",
    },
  ],
});

const settlementCapabilityCatalog: NetworkCapabilityCatalog = {
  capabilities: [
    {
      id: "chat",
      summary: "Confirms settlement ETA, replay state, and partner-bank evidence during launch.",
      outcome: "Returns the next settlement milestone without leaving the launch room.",
      version: "1.2.0",
      digest: "sha256:partner-settlement-chat",
      context_needed: ["channel-history", "merchant-case-id"],
      artifacts_expected: ["message.text", "settlement.eta"],
      execution_outline: [
        "Read the launch-room request.",
        "Confirm the partner-side replay or hold status.",
        "Reply with the next ETA or blocking evidence.",
      ],
      constraints: ["Network-visible only"],
      examples: ["Settlement replay ETA", "Partner incident ack"],
      requirements: [],
    },
  ],
};

const primaryCapabilityCatalog: NetworkCapabilityCatalog = {
  capabilities: [
    {
      id: "chat",
      summary:
        "Coordinates launch-room decisions across product, finance, engineering, and support.",
      outcome: "Operators get one shared launch narrative with accountable next steps.",
      version: "1.1.0",
      digest: "sha256:launch-room-chat",
      context_needed: ["channel-history", "active-workspace"],
      artifacts_expected: ["message.text"],
      execution_outline: [
        "Summarize the current blocker set.",
        "Assign the next owner and expected ETA.",
        "Report back in operator-safe language.",
      ],
      constraints: ["Latency-sensitive"],
      examples: ["Launch blocker review", "Cross-functional checkpoint"],
      requirements: [],
    },
    {
      id: "tools",
      summary:
        "Runs verification and evidence-gathering passes before go-live changes are approved.",
      outcome: "Delegated checks return concrete evidence the launch room can act on.",
      version: "0.4.0",
      digest: "sha256:launch-room-tools",
      context_needed: ["workspace-root", "launch-checklist"],
      artifacts_expected: ["tool-call.summary"],
      execution_outline: [
        "Pick the next risky surface.",
        "Run the command or read the evidence.",
        "Report the result back to the room.",
      ],
      constraints: [],
      examples: ["Hero QA pass", "Canary verification", "Pricing claim review"],
      requirements: ["chat"],
    },
  ],
};

function sayMessage(
  messageId: string,
  displayName: string,
  sessionId: string,
  peerFrom: string,
  timestamp: string,
  text: string
): NetworkConversationMessage {
  return {
    body: { text },
    channel: storyHeroNetworkChannel,
    direction: "sent",
    display_name: displayName,
    kind: "say",
    local: true,
    message_id: messageId,
    peer_from: peerFrom,
    preview_text: text,
    session_id: sessionId,
    text,
    timestamp,
  };
}

function directMessage(
  messageId: string,
  displayName: string,
  timestamp: string,
  text: string,
  options: {
    direction: "sent" | "received";
    local: boolean;
    peerFrom: string;
    peerTo: string;
    sessionId?: string;
  }
): NetworkConversationMessage {
  return {
    body: { text },
    channel: storyHeroNetworkChannel,
    direct_id: "direct_story_launch_corridor",
    direction: options.direction,
    display_name: displayName,
    kind: "say",
    local: options.local,
    message_id: messageId,
    peer_from: options.peerFrom,
    peer_to: options.peerTo,
    preview_text: text,
    session_id: options.sessionId,
    surface: "direct",
    text,
    timestamp,
    work_id: "work_story_launch_corridor",
  };
}

export const networkStatusFixture: NetworkStatus = {
  enabled: true,
  status: "active",
  channels: 10,
  local_peers: 6,
  messages_sent: 418,
};

export const networkChannelsFixture: NetworkChannelsResponse = {
  channels: [
    {
      channel: storyChannels.launchWarRoom,
      created_at: "2026-04-17T14:00:00Z",
      created_by: storyAgentNames.product,
      last_activity_at: "2026-04-17T18:16:00Z",
      last_message_preview:
        "Open both corridors at 18:30 UTC. Keep the fallback banner armed for the first 15 minutes.",
      local_peer_count: 8,
      message_count: 34,
      peer_count: 8,
      fanout_policy: "capability_match",
      coordinator_peer_id: ctoPeerCard.peer_id,
      purpose:
        "Coordinate launch command, pricing approvals, engineering sign-off, and merchant-risk decisions for Northstar Pay Checkout.",
      session_count: 6,
      workspace_id: storyWorkspaceIds.hq,
    },
    {
      channel: storyChannels.landingPage,
      created_at: "2026-04-17T12:00:00Z",
      created_by: storyAgentNames.frontend,
      last_activity_at: "2026-04-17T18:09:00Z",
      last_message_preview:
        "Hero headline and pricing banner now match the approved launch-week claims.",
      local_peer_count: 3,
      message_count: 12,
      peer_count: 3,
      purpose: "Align landing-page QA, pricing claims, and launch-ready marketing surfaces.",
      session_count: 3,
      workspace_id: storyWorkspaceIds.product,
    },
    {
      channel: storyChannels.releaseControl,
      created_at: "2026-04-17T09:15:00Z",
      created_by: storyAgentNames.release,
      last_activity_at: "2026-04-17T18:03:00Z",
      last_message_preview: "Canary is healthy at 25%. Rollback path remains warm.",
      local_peer_count: 2,
      message_count: 11,
      peer_count: 2,
      purpose: "Coordinate canary promotion, rollback guardrails, and cutover evidence.",
      session_count: 2,
      workspace_id: storyWorkspaceIds.platform,
    },
    {
      channel: storyChannels.growthLaunch,
      created_at: "2026-04-17T09:45:00Z",
      created_by: storyAgentNames.marketing,
      last_activity_at: "2026-04-17T17:58:00Z",
      last_message_preview:
        "CRM batch is staged and paid spend stays paused until the launch-room release.",
      local_peer_count: 3,
      message_count: 9,
      peer_count: 3,
      purpose: "Launch timing for CRM, ads, pricing claims, and merchant acquisition campaigns.",
      session_count: 2,
      workspace_id: storyWorkspaceIds.growth,
    },
    {
      channel: storyChannels.financeWatch,
      created_at: "2026-04-17T10:30:00Z",
      created_by: storyAgentNames.cfo,
      last_activity_at: "2026-04-17T18:13:00Z",
      last_message_preview: "GMV forecast remains above $2.2M with the current launch sequence.",
      local_peer_count: 2,
      message_count: 7,
      peer_count: 2,
      purpose: "Track GMV, burn, reserves, and finance approvals tied to launch-day decisions.",
      session_count: 2,
      workspace_id: storyWorkspaceIds.finance,
    },
    {
      channel: storyChannels.supportSwarm,
      created_at: "2026-04-17T13:00:00Z",
      created_by: storyAgentNames.support,
      last_activity_at: "2026-04-17T18:08:00Z",
      last_message_preview:
        "VIP queue is down to four and pricing questions now dominate the inbox.",
      local_peer_count: 2,
      message_count: 14,
      peer_count: 2,
      purpose: "Coordinate merchant communications, queue pressure, and launch-day support macros.",
      session_count: 2,
      workspace_id: storyWorkspaceIds.support,
    },
    {
      channel: storyChannels.riskOps,
      created_at: "2026-04-17T10:45:00Z",
      created_by: storyAgentNames.fraud,
      last_activity_at: "2026-04-17T18:07:00Z",
      last_message_preview: "Reserve exposure is stable after the last merchant batch review.",
      local_peer_count: 2,
      message_count: 8,
      peer_count: 2,
      purpose: "Monitor payout risk, reserves, and fraud anomalies during the launch window.",
      session_count: 2,
      workspace_id: storyWorkspaceIds.risk,
    },
    {
      channel: storyChannels.partnerSync,
      created_at: "2026-04-17T09:05:00Z",
      created_by: storyAgentNames.platform,
      last_activity_at: "2026-04-17T18:01:00Z",
      last_message_preview:
        "Webhook retries are within budget and partner replay visibility is back.",
      local_peer_count: 2,
      message_count: 6,
      peer_count: 2,
      purpose: "Track partner APIs, replay status, and integration-level launch dependencies.",
      session_count: 1,
      workspace_id: storyWorkspaceIds.platform,
    },
    {
      channel: storyChannels.merchantEscalations,
      created_at: "2026-04-17T12:50:00Z",
      created_by: storyAgentNames.support,
      last_activity_at: "2026-04-17T17:56:00Z",
      last_message_preview:
        "Three pilot merchants need callbacks once BR settlement replay closes.",
      local_peer_count: 2,
      message_count: 10,
      peer_count: 2,
      purpose: "High-touch merchant escalations that need a named owner before launch completes.",
      session_count: 2,
      workspace_id: storyWorkspaceIds.support,
    },
    {
      channel: storyChannels.execSignal,
      created_at: "2026-04-17T10:10:00Z",
      created_by: storyAgentNames.cto,
      last_activity_at: "2026-04-17T17:52:00Z",
      last_message_preview:
        "Board-facing summary updated with the fallback plan and current burn impact.",
      local_peer_count: 2,
      message_count: 5,
      peer_count: 2,
      purpose: "Executive-only launch signals, fallback thresholds, and external-update prep.",
      session_count: 2,
      workspace_id: storyWorkspaceIds.hq,
    },
  ],
};

export const networkChannelFixture: NetworkChannel = {
  channel: storyHeroNetworkChannel,
  created_at: "2026-04-17T14:00:00Z",
  created_by: storyAgentNames.product,
  kind_counts: [
    { kind: "say", count: 15 },
    { kind: "receipt", count: 3 },
    { kind: "capability", count: 1 },
    { kind: "whois", count: 2 },
    { kind: "trace", count: 3 },
    { kind: "greet", count: 2 },
  ],
  last_activity_at: "2026-04-17T18:16:00Z",
  last_message_preview:
    "Open both corridors at 18:30 UTC. Keep the fallback banner armed for the first 15 minutes.",
  local_peer_count: 8,
  message_count: 34,
  peer_count: 8,
  fanout_policy: "capability_match",
  coordinator_peer_id: ctoPeerCard.peer_id,
  peers: [
    {
      workspace_id: storyWorkspaceIds.hq,
      channel: storyHeroNetworkChannel,
      display_name: "Northstar Launch Control",
      joined_at: "2026-04-17T14:00:00Z",
      local: true,
      peer_card: primaryPeerCard,
      peer_id: primaryPeerCard.peer_id,
      presence_state: "local",
      session_id: storySessionIds.product,
    },
    {
      channel: storyHeroNetworkChannel,
      display_name: "Northstar CTO Office",
      joined_at: "2026-04-17T14:02:00Z",
      local: true,
      peer_card: ctoPeerCard,
      peer_id: ctoPeerCard.peer_id,
      presence_state: "local",
      session_id: storySessionIds.cto,
    },
    {
      channel: storyHeroNetworkChannel,
      display_name: "Northstar Finance Desk",
      joined_at: "2026-04-17T14:04:00Z",
      local: true,
      peer_card: financePeerCard,
      peer_id: financePeerCard.peer_id,
      presence_state: "local",
      session_id: storySessionIds.cfo,
    },
    {
      channel: storyHeroNetworkChannel,
      display_name: "Northstar Growth Desk",
      joined_at: "2026-04-17T14:05:00Z",
      local: true,
      peer_card: growthPeerCard,
      peer_id: growthPeerCard.peer_id,
      presence_state: "local",
      session_id: storySessionIds.marketing,
    },
    {
      channel: storyHeroNetworkChannel,
      display_name: "Northstar Support Desk",
      joined_at: "2026-04-17T14:06:00Z",
      local: true,
      peer_card: supportPeerCard,
      peer_id: supportPeerCard.peer_id,
      presence_state: "local",
      session_id: storySessionIds.support,
    },
    {
      channel: storyHeroNetworkChannel,
      display_name: "Northstar Frontend Desk",
      joined_at: "2026-04-17T14:07:00Z",
      local: true,
      peer_card: frontendPeerCard,
      peer_id: frontendPeerCard.peer_id,
      presence_state: "local",
      session_id: storySessionIds.frontend,
    },
    {
      channel: storyHeroNetworkChannel,
      display_name: "Partner Settlement Desk",
      joined_at: "2026-04-17T14:08:00Z",
      local: true,
      peer_card: settlementPeerCard,
      peer_id: settlementPeerCard.peer_id,
      presence_state: "local",
      session_id: "sess_partner_bank",
    },
    {
      channel: storyHeroNetworkChannel,
      display_name: "Creative Studio Desk",
      joined_at: "2026-04-17T14:10:00Z",
      local: true,
      peer_card: creativePeerCard,
      peer_id: creativePeerCard.peer_id,
      presence_state: "local",
      session_id: "sess_creative_studio",
    },
  ],
  purpose:
    "Coordinate launch command, pricing approvals, engineering sign-off, and merchant-risk decisions for Northstar Pay Checkout.",
  session_count: 6,
  sessions: [
    {
      id: storySessionIds.product,
      name: "Launch room command brief",
      agent_name: storyAgentNames.product,
      provider: "gemini",
      state: "active",
      badge: "idle",
      attachable: true,
      available_commands: [],
      created_at: "2026-04-17T11:20:00Z",
      updated_at: "2026-04-17T18:14:00Z",
      workspace_id: storyWorkspaceIds.hq,
      workspace_path: storyWorkspacePaths.hq,
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
      state: "active",
      badge: "idle",
      attachable: true,
      available_commands: [],
      created_at: "2026-04-17T10:10:00Z",
      updated_at: "2026-04-17T18:11:00Z",
      workspace_id: storyWorkspaceIds.hq,
      workspace_path: storyWorkspacePaths.hq,
      acp_caps: {
        supports_load_session: true,
        supported_modes: ["chat"],
      },
    },
    {
      id: storySessionIds.cfo,
      name: "Launch revenue watch",
      agent_name: storyAgentNames.cfo,
      provider: "claude",
      state: "active",
      badge: "idle",
      attachable: true,
      available_commands: [],
      created_at: "2026-04-17T10:30:00Z",
      updated_at: "2026-04-17T18:13:00Z",
      workspace_id: storyWorkspaceIds.finance,
      workspace_path: storyWorkspacePaths.finance,
      acp_caps: {
        supports_load_session: true,
        supported_modes: ["chat"],
      },
    },
    {
      id: storySessionIds.compliance,
      name: "Claim compliance review",
      agent_name: storyAgentNames.compliance,
      provider: "qwen-code",
      state: "active",
      badge: "idle",
      attachable: true,
      available_commands: [],
      created_at: "2026-04-17T12:25:00Z",
      updated_at: "2026-04-17T18:04:00Z",
      workspace_id: storyWorkspaceIds.risk,
      workspace_path: storyWorkspacePaths.risk,
      acp_caps: {
        supports_load_session: true,
        supported_modes: ["chat"],
      },
    },
    {
      id: storySessionIds.frontend,
      name: "Landing page launch QA",
      agent_name: storyAgentNames.frontend,
      provider: "codex",
      state: "active",
      badge: "idle",
      attachable: true,
      available_commands: [],
      created_at: "2026-04-17T12:00:00Z",
      updated_at: "2026-04-17T18:09:00Z",
      workspace_id: storyWorkspaceIds.product,
      workspace_path: storyWorkspacePaths.product,
      acp_caps: {
        supports_load_session: true,
        supported_modes: ["chat"],
      },
    },
    {
      id: storySessionIds.support,
      name: "Launch support swarm",
      agent_name: storyAgentNames.support,
      provider: "claude",
      state: "active",
      badge: "idle",
      attachable: true,
      available_commands: [],
      created_at: "2026-04-17T13:00:00Z",
      updated_at: "2026-04-17T18:08:00Z",
      workspace_id: storyWorkspaceIds.support,
      workspace_path: storyWorkspacePaths.support,
      acp_caps: {
        supports_load_session: true,
        supported_modes: ["chat"],
      },
    },
  ],
  workspace_id: storyWorkspaceIds.hq,
};

export const networkThreadMessagesFixture: NetworkThreadMessage[] = [
  {
    body: {
      summary:
        "Launch Control heartbeat with product, finance, engineering, support, and partner desks active.",
      peer_card: {
        display_name: "Northstar Launch Control",
        capabilities: ["chat", "tools"],
      },
    },
    channel: storyHeroNetworkChannel,
    direction: "sent",
    display_name: "Northstar Launch Control",
    kind: "greet",
    local: true,
    message_id: "msg_launch_001",
    peer_from: primaryPeerCard.peer_id,
    preview_text:
      "Launch Control heartbeat with product, finance, engineering, support, and partner desks active.",
    session_id: storySessionIds.product,
    timestamp: "2026-04-17T17:50:00Z",
  },
  {
    body: {
      summary: "Partner desks are connected and waiting on the final pricing and replay decision.",
      peer_card: {
        display_name: "Partner Settlement Desk",
        capabilities: ["chat"],
      },
    },
    channel: storyHeroNetworkChannel,
    direction: "received",
    display_name: "Partner Settlement Desk",
    kind: "greet",
    local: false,
    message_id: "msg_launch_002",
    peer_from: settlementPeerCard.peer_id,
    preview_text:
      "Partner desks are connected and waiting on the final pricing and replay decision.",
    timestamp: "2026-04-17T17:51:00Z",
  },
  sayMessage(
    "msg_launch_003",
    "Launch room command brief",
    storySessionIds.product,
    primaryPeerCard.peer_id,
    "2026-04-17T17:52:00Z",
    "T-minus 38 minutes. Checklist is green except partner-bank timeout copy and the BR support queue."
  ),
  sayMessage(
    "msg_launch_004",
    "Executive launch review",
    storySessionIds.cto,
    ctoPeerCard.peer_id,
    "2026-04-17T17:53:00Z",
    "If the timeout copy slips, we gate BR to the fallback banner and keep MX fully live."
  ),
  sayMessage(
    "msg_launch_005",
    "Launch revenue watch",
    storySessionIds.cfo,
    financePeerCard.peer_id,
    "2026-04-17T17:54:00Z",
    "Current GMV forecast still clears $2.2M if both corridors open by 18:30 UTC."
  ),
  directMessage(
    "msg_launch_006",
    "Partner Settlement Desk",
    "2026-04-17T17:55:00Z",
    "Updated settlement replay ETA is 18:22 UTC. No fund risk on the delayed BR batch.",
    {
      direction: "received",
      local: false,
      peerFrom: settlementPeerCard.peer_id,
      peerTo: primaryPeerCard.peer_id,
    }
  ),
  sayMessage(
    "msg_launch_007",
    "CRM launch timing",
    storySessionIds.marketing,
    growthPeerCard.peer_id,
    "2026-04-17T17:56:00Z",
    "Ads stay paused until the hero pricing line is final. CRM batch is staged for 18:34 UTC."
  ),
  sayMessage(
    "msg_launch_008",
    "Headline claim polish",
    storySessionIds.copywriter,
    growthPeerCard.peer_id,
    "2026-04-17T17:57:00Z",
    "Approved headline: Launch checkout in days, not quarters. I removed every zero-fee claim from the hero."
  ),
  {
    body: {
      type: "ownership_query",
      query: "Which launch room owns BR merchant payout delays after go-live?",
      peer_card: { peer_id: supportPeerCard.peer_id },
    },
    channel: storyHeroNetworkChannel,
    direction: "sent",
    display_name: "Launch support swarm",
    kind: "whois",
    local: true,
    message_id: "msg_launch_009",
    peer_from: supportPeerCard.peer_id,
    preview_text: "Which launch room owns BR merchant payout delays after go-live?",
    session_id: storySessionIds.support,
    timestamp: "2026-04-17T17:58:00Z",
  },
  {
    body: {
      state: "running",
      message: "CDN invalidate for the hero rollout is running in São Paulo and Querétaro POPs.",
    },
    channel: storyHeroNetworkChannel,
    direction: "sent",
    display_name: "Partner webhook stability",
    kind: "trace",
    local: true,
    message_id: "msg_launch_010",
    peer_from: frontendPeerCard.peer_id,
    preview_text: "CDN invalidate for the hero rollout is running in São Paulo and Querétaro POPs.",
    session_id: storySessionIds.platform,
    timestamp: "2026-04-17T17:59:00Z",
    trace_id: "trace_cdn_441",
  },
  {
    body: {
      status: "ack",
      for_id: "msg_launch_006",
      detail: "Partner bank confirmed the replay ETA and attached the settlement checkpoint.",
    },
    channel: storyHeroNetworkChannel,
    direction: "received",
    display_name: "Partner Settlement Desk",
    kind: "receipt",
    local: false,
    message_id: "msg_launch_011",
    peer_from: settlementPeerCard.peer_id,
    preview_text: "Partner bank confirmed the replay ETA and attached the settlement checkpoint.",
    timestamp: "2026-04-17T18:00:00Z",
  },
  {
    body: {
      capability: {
        id: "tools",
        summary: "Replay verification handoff",
        outcome: "Partner bank can report the final replay state back to launch command.",
        version: "0.2.0",
        digest: "sha256:partner-replay-handoff",
        execution_outline: [
          "Check the replay cursor.",
          "Confirm whether merchant funds are safe.",
          "Post the next ETA to launch command.",
        ],
      },
    },
    channel: storyHeroNetworkChannel,
    direction: "received",
    display_name: "Partner Settlement Desk",
    kind: "capability",
    local: false,
    message_id: "msg_launch_012",
    peer_from: settlementPeerCard.peer_id,
    preview_text: "Replay verification handoff",
    timestamp: "2026-04-17T18:01:00Z",
  },
  sayMessage(
    "msg_launch_013",
    "Landing page launch QA",
    storySessionIds.frontend,
    frontendPeerCard.peer_id,
    "2026-04-17T18:02:00Z",
    "Mobile hero wrap is fixed at 360px and 390px. I also validated the pricing banner truncation."
  ),
  sayMessage(
    "msg_launch_014",
    "Launch support swarm",
    storySessionIds.support,
    supportPeerCard.peer_id,
    "2026-04-17T18:03:00Z",
    "Support backlog peaked at 27 tickets, but the VIP queue is down to 4 and pricing questions now dominate."
  ),
  directMessage(
    "msg_launch_015",
    "Creative Studio Desk",
    "2026-04-17T18:04:00Z",
    "Meta and Google creatives are staged. We only need the final pricing line for the carousel variant.",
    {
      direction: "received",
      local: false,
      peerFrom: creativePeerCard.peer_id,
      peerTo: growthPeerCard.peer_id,
    }
  ),
  sayMessage(
    "msg_launch_016",
    "Claim compliance review",
    storySessionIds.compliance,
    primaryPeerCard.peer_id,
    "2026-04-17T18:05:00Z",
    "BR timeout copy is approved if we avoid guaranteed-settlement wording and keep the fallback ETA generic."
  ),
  {
    body: {
      state: "progress",
      message:
        "Canary is moving from 10% to 25%. Error budget remains inside the launch threshold.",
    },
    channel: storyHeroNetworkChannel,
    direction: "sent",
    display_name: "Release control canary",
    kind: "trace",
    local: true,
    message_id: "msg_launch_017",
    peer_from: ctoPeerCard.peer_id,
    preview_text:
      "Canary is moving from 10% to 25%. Error budget remains inside the launch threshold.",
    session_id: storySessionIds.release,
    timestamp: "2026-04-17T18:06:00Z",
    trace_id: "trace_canary_882",
  },
  {
    body: {
      status: "staged",
      for_id: "msg_launch_015",
      detail:
        "Creative variants are staged and will publish immediately after launch-room release.",
    },
    channel: storyHeroNetworkChannel,
    direction: "received",
    display_name: "Creative Studio Desk",
    kind: "receipt",
    local: false,
    message_id: "msg_launch_018",
    peer_from: creativePeerCard.peer_id,
    preview_text:
      "Creative variants are staged and will publish immediately after launch-room release.",
    timestamp: "2026-04-17T18:07:00Z",
  },
  sayMessage(
    "msg_launch_019",
    "Launch room command brief",
    storySessionIds.product,
    primaryPeerCard.peer_id,
    "2026-04-17T18:08:00Z",
    "Once canary hits 25%, marketing can release the CRM batch and copy can publish the hero claim update."
  ),
  sayMessage(
    "msg_launch_020",
    "Launch revenue watch",
    storySessionIds.cfo,
    financePeerCard.peer_id,
    "2026-04-17T18:09:00Z",
    "Burn impact is flat. Refund reserve buffer remains inside policy even if BR opens five minutes late."
  ),
  {
    body: {
      type: "ownership_query",
      query: "Who owns MX cashback wording on the landing page after the hero update ships?",
      peer_card: { peer_id: growthPeerCard.peer_id },
    },
    channel: storyHeroNetworkChannel,
    direction: "sent",
    display_name: "Launch room command brief",
    kind: "whois",
    local: true,
    message_id: "msg_launch_021",
    peer_from: primaryPeerCard.peer_id,
    preview_text: "Who owns MX cashback wording on the landing page after the hero update ships?",
    session_id: storySessionIds.product,
    timestamp: "2026-04-17T18:10:00Z",
  },
  sayMessage(
    "msg_launch_022",
    "Headline claim polish",
    storySessionIds.copywriter,
    growthPeerCard.peer_id,
    "2026-04-17T18:11:00Z",
    "Copywriter owns it. The final approved line is already in Pricing Claims Guardrails and the hero patch is ready."
  ),
  directMessage(
    "msg_launch_023",
    "Partner Settlement Desk",
    "2026-04-17T18:12:00Z",
    "Replay finished. BR timeout copy is no longer blocked on partner evidence.",
    {
      direction: "received",
      local: false,
      peerFrom: settlementPeerCard.peer_id,
      peerTo: primaryPeerCard.peer_id,
    }
  ),
  {
    body: {
      status: "ready",
      for_id: "trace_canary_882",
      detail: "Canary reached 25% with no material error increase. Rollback remains warm.",
    },
    channel: storyHeroNetworkChannel,
    direction: "sent",
    display_name: "Release control canary",
    kind: "receipt",
    local: true,
    message_id: "msg_launch_024",
    peer_from: ctoPeerCard.peer_id,
    preview_text: "Canary reached 25% with no material error increase. Rollback remains warm.",
    session_id: storySessionIds.release,
    timestamp: "2026-04-17T18:13:00Z",
  },
  {
    body: {
      state: "done",
      message:
        "Support macro sync completed. Launch-safe responses are now live in the escalation inbox.",
    },
    channel: storyHeroNetworkChannel,
    direction: "sent",
    display_name: "Launch support swarm",
    kind: "trace",
    local: true,
    message_id: "msg_launch_025",
    peer_from: supportPeerCard.peer_id,
    preview_text:
      "Support macro sync completed. Launch-safe responses are now live in the escalation inbox.",
    session_id: storySessionIds.support,
    timestamp: "2026-04-17T18:14:00Z",
    trace_id: "trace_support_122",
  },
  sayMessage(
    "msg_launch_026",
    "Executive launch review",
    storySessionIds.cto,
    ctoPeerCard.peer_id,
    "2026-04-17T18:16:00Z",
    "Open both corridors at 18:30 UTC. Keep the fallback banner armed for the first 15 minutes."
  ),
];

export const networkDirectRoomMessagesFixture: NetworkDirectRoomMessage[] = [
  directMessage(
    "msg_dm_001",
    "Launch room command brief",
    "2026-04-17T17:55:00Z",
    "Can you confirm whether the BR settlement replay clears before launch cutover?",
    {
      direction: "sent",
      local: true,
      peerFrom: primaryPeerCard.peer_id,
      peerTo: settlementPeerCard.peer_id,
      sessionId: storySessionIds.product,
    }
  ),
  directMessage(
    "msg_dm_002",
    "Partner Settlement Desk",
    "2026-04-17T18:00:00Z",
    "Replay ETA is 18:22 UTC and no merchant funds are at risk during the delay.",
    {
      direction: "received",
      local: false,
      peerFrom: settlementPeerCard.peer_id,
      peerTo: primaryPeerCard.peer_id,
    }
  ),
  directMessage(
    "msg_dm_003",
    "Launch room command brief",
    "2026-04-17T18:06:00Z",
    "We only need confirmation that the public timeout copy can reference a generic banking delay.",
    {
      direction: "sent",
      local: true,
      peerFrom: primaryPeerCard.peer_id,
      peerTo: settlementPeerCard.peer_id,
      sessionId: storySessionIds.product,
    }
  ),
  directMessage(
    "msg_dm_004",
    "Partner Settlement Desk",
    "2026-04-17T18:12:00Z",
    "Confirmed. Generic delay language is fine and the replay is now complete.",
    {
      direction: "received",
      local: false,
      peerFrom: settlementPeerCard.peer_id,
      peerTo: primaryPeerCard.peer_id,
    }
  ),
];

export const networkPeersFixture: NetworkPeerSummary[] = [
  {
    channel: storyHeroNetworkChannel,
    display_name: "Northstar Launch Control",
    joined_at: "2026-04-17T14:00:00Z",
    local: true,
    peer_card: primaryPeerCard,
    peer_id: primaryPeerCard.peer_id,
    presence_state: "local",
    session_id: storySessionIds.product,
  },
  {
    channel: storyChannels.financeWatch,
    display_name: "Northstar Finance Desk",
    joined_at: "2026-04-17T14:04:00Z",
    local: true,
    peer_card: financePeerCard,
    peer_id: financePeerCard.peer_id,
    presence_state: "local",
    session_id: storySessionIds.cfo,
  },
  {
    channel: storyChannels.growthLaunch,
    display_name: "Northstar Growth Desk",
    joined_at: "2026-04-17T14:05:00Z",
    local: true,
    peer_card: growthPeerCard,
    peer_id: growthPeerCard.peer_id,
    presence_state: "local",
    session_id: storySessionIds.marketing,
  },
  {
    channel: storyChannels.supportSwarm,
    display_name: "Northstar Support Desk",
    joined_at: "2026-04-17T14:06:00Z",
    local: true,
    peer_card: supportPeerCard,
    peer_id: supportPeerCard.peer_id,
    presence_state: "local",
    session_id: storySessionIds.support,
  },
  {
    channel: storyChannels.landingPage,
    display_name: "Northstar Frontend Desk",
    joined_at: "2026-04-17T14:07:00Z",
    local: true,
    peer_card: frontendPeerCard,
    peer_id: frontendPeerCard.peer_id,
    presence_state: "local",
    session_id: storySessionIds.frontend,
  },
  {
    channel: storyChannels.execSignal,
    display_name: "Northstar CTO Office",
    joined_at: "2026-04-17T14:02:00Z",
    local: true,
    peer_card: ctoPeerCard,
    peer_id: ctoPeerCard.peer_id,
    presence_state: "local",
    session_id: storySessionIds.cto,
  },
  {
    channel: storyHeroNetworkChannel,
    display_name: "Partner Settlement Desk",
    joined_at: "2026-04-17T14:08:00Z",
    local: true,
    peer_card: settlementPeerCard,
    peer_id: settlementPeerCard.peer_id,
    presence_state: "local",
    session_id: "sess_partner_bank",
  },
  {
    channel: storyHeroNetworkChannel,
    display_name: "Creative Studio Desk",
    joined_at: "2026-04-17T14:10:00Z",
    local: true,
    peer_card: creativePeerCard,
    peer_id: creativePeerCard.peer_id,
    presence_state: "local",
    session_id: "sess_creative_studio",
  },
];

export const networkPeerFixture: NetworkPeerDetail = {
  channel: storyHeroNetworkChannel,
  display_name: "Northstar Launch Control",
  joined_at: "2026-04-17T14:00:00Z",
  local: true,
  metrics: {
    sent: 64,
    received: 41,
    delivered: 60,
    rejected: 1,
  },
  peer_card: primaryPeerCard,
  capability_catalog: primaryCapabilityCatalog,
  peer_id: primaryPeerCard.peer_id,
  presence_state: "local",
  session_id: storySessionIds.product,
};

export const networkSettlementPeerFixture: NetworkPeerDetail = {
  channel: storyHeroNetworkChannel,
  display_name: "Partner Settlement Desk",
  joined_at: "2026-04-17T14:08:00Z",
  local: true,
  metrics: {
    sent: 15,
    received: 19,
    delivered: 18,
    rejected: 0,
  },
  peer_card: settlementPeerCard,
  capability_catalog: settlementCapabilityCatalog,
  peer_id: settlementPeerCard.peer_id,
  presence_state: "local",
  session_id: "sess_partner_bank",
};

export const createNetworkChannelFixture: CreateNetworkChannelResponse = {
  channel: networkChannelFixture,
};

export const networkThreadsFixture: NetworkThreadSummary[] = [
  {
    channel: storyHeroNetworkChannel,
    last_activity_at: "2026-04-17T18:16:00Z",
    last_message_preview:
      "Open both corridors at 18:30 UTC. Keep the fallback banner armed for the first 15 minutes.",
    message_count: 28,
    open_work_count: 2,
    opened_at: "2026-04-17T17:50:00Z",
    opened_by_peer_id: primaryPeerCard.peer_id,
    opened_session_id: storySessionIds.product,
    participant_count: 6,
    root_message_id: "msg_launch_001",
    thread_id: "thread_launch_command",
    title: "Launch command brief",
  },
  {
    channel: storyHeroNetworkChannel,
    last_activity_at: "2026-04-17T18:11:00Z",
    last_message_preview:
      "Copywriter owns it. The final approved line is already in Pricing Claims Guardrails.",
    message_count: 4,
    open_work_count: 0,
    opened_at: "2026-04-17T18:10:00Z",
    opened_by_peer_id: primaryPeerCard.peer_id,
    opened_session_id: storySessionIds.product,
    participant_count: 3,
    root_message_id: "msg_launch_021",
    thread_id: "thread_pricing_ownership",
    title: "MX cashback wording",
  },
];

export const networkThreadDetailFixture: NetworkThreadDetail = {
  channel: storyHeroNetworkChannel,
  last_activity_at: "2026-04-17T18:16:00Z",
  last_message_preview:
    "Open both corridors at 18:30 UTC. Keep the fallback banner armed for the first 15 minutes.",
  message_count: 28,
  open_work_count: 2,
  opened_at: "2026-04-17T17:50:00Z",
  opened_by_peer_id: primaryPeerCard.peer_id,
  opened_session_id: storySessionIds.product,
  participant_count: 6,
  root_message_id: "msg_launch_001",
  task_links: [
    {
      channel: storyHeroNetworkChannel,
      digest: "Launch command brief",
      origin_message_id: "msg_launch_001",
      task_id: "task_story_thread_promoted",
      thread_id: "thread_launch_command",
      workspace_id: storyWorkspaceIds.hq,
    },
  ],
  thread_id: "thread_launch_command",
  title: "Launch command brief",
};

export const networkDirectRoomsFixture: NetworkDirectRoomSummary[] = [
  {
    channel: storyHeroNetworkChannel,
    direct_id: "direct_story_launch_corridor",
    last_activity_at: "2026-04-17T18:12:00Z",
    last_message_preview:
      "Replay finished. BR timeout copy is no longer blocked on partner evidence.",
    message_count: 4,
    open_work_count: 1,
    opened_at: "2026-04-17T17:55:00Z",
    session_a: "sess_partner_bank",
    session_b: storySessionIds.product,
  },
];

export const networkDirectRoomDetailFixture: NetworkDirectRoomDetail = {
  workspace_id: storyWorkspaceIds.hq,
  channel: storyHeroNetworkChannel,
  direct_id: "direct_story_launch_corridor",
  last_activity_at: "2026-04-17T18:12:00Z",
  last_message_preview:
    "Replay finished. BR timeout copy is no longer blocked on partner evidence.",
  message_count: 4,
  open_work_count: 1,
  opened_at: "2026-04-17T17:55:00Z",
  session_a: "sess_partner_bank",
  session_b: storySessionIds.product,
};

export const networkWorkFixture: NetworkWorkDetail = {
  channel: storyHeroNetworkChannel,
  direct_id: "direct_story_launch_corridor",
  last_activity_at: "2026-04-17T18:12:00Z",
  opened_at: "2026-04-17T17:55:00Z",
  opened_session_id: storySessionIds.product,
  state: "working",
  surface: "direct",
  target_session_id: "sess_partner_bank",
  work_id: "work_story_launch_corridor",
};
