import type { AgentPayload } from "@/systems/agent";

import type {
  NetworkConversationMessage,
  NetworkCreateChannelDraft,
  NetworkKindFilter,
  NetworkPresenceState,
  NetworkPeerSummary,
  NetworkSignalTone,
} from "../types";

const NETWORK_SUPPORTED_KINDS: ReadonlyArray<Exclude<NetworkKindFilter, "all">> = [
  "say",
  "receipt",
  "capability",
  "greet",
  "whois",
  "trace",
];

const NETWORK_KIND_LABELS: Record<Exclude<NetworkKindFilter, "all">, string> = {
  capability: "capability",
  greet: "greet",
  receipt: "receipt",
  say: "say",
  trace: "trace",
  whois: "whois",
};

const NETWORK_KIND_TONES: Record<Exclude<NetworkKindFilter, "all">, NetworkSignalTone> = {
  capability: "info",
  greet: "success",
  receipt: "warning",
  say: "neutral",
  trace: "info",
  whois: "warning",
};

function parseTimestampOrZero(value?: string | null): number {
  if (!value) {
    return 0;
  }
  const parsed = new Date(value).getTime();
  return Number.isNaN(parsed) ? 0 : parsed;
}

export function getMostRecentTimestamp(
  primaryValue?: string | null,
  secondaryValue?: string | null
): string | null {
  if (!primaryValue) {
    return secondaryValue ?? null;
  }
  if (!secondaryValue) {
    return primaryValue;
  }

  return parseTimestampOrZero(secondaryValue) > parseTimestampOrZero(primaryValue)
    ? secondaryValue
    : primaryValue;
}

export function createNetworkChannelDraft(): NetworkCreateChannelDraft {
  return {
    channelName: "",
    purpose: "",
    selectedAgentNames: [],
    fanoutPolicy: "capability_match",
  };
}

export function toggleDraftAgent(
  draft: NetworkCreateChannelDraft,
  agentName: string
): NetworkCreateChannelDraft {
  const selectedAgentNames = draft.selectedAgentNames.includes(agentName)
    ? draft.selectedAgentNames.filter(name => name !== agentName)
    : [...draft.selectedAgentNames, agentName];

  return {
    ...draft,
    selectedAgentNames,
  };
}

export function formatNetworkRelativeTime(value?: string | null): string {
  if (!value) {
    return "Unavailable";
  }

  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return "Unavailable";
  }

  const diffMs = Date.now() - parsed.getTime();
  if (diffMs < 0) {
    return "just now";
  }

  const diffSeconds = Math.floor(diffMs / 1000);
  if (diffSeconds < 5) {
    return "just now";
  }
  if (diffSeconds < 60) {
    return `${diffSeconds}s ago`;
  }

  const diffMinutes = Math.floor(diffSeconds / 60);
  if (diffMinutes < 60) {
    return `${diffMinutes}m ago`;
  }

  const diffHours = Math.floor(diffMinutes / 60);
  if (diffHours < 24) {
    return `${diffHours}h ago`;
  }

  const diffDays = Math.floor(diffHours / 24);
  return `${diffDays}d ago`;
}

export function getPeerDisplayName(
  peer: Pick<NetworkPeerSummary, "display_name" | "peer_card" | "peer_id">
): string {
  return peer.display_name ?? peer.peer_card.display_name ?? peer.peer_id;
}

export function getPeerRecencyAt(
  peer: Pick<NetworkPeerSummary, "joined_at"> | null | undefined
): string | null {
  return peer?.joined_at ?? null;
}

export function toNetworkPresenceState(_value?: string | null): NetworkPresenceState {
  return "local";
}

export function formatNetworkPresenceLabel(state: NetworkPresenceState): string {
  return state;
}

export function getNetworkStatusTone(status?: string | null): NetworkSignalTone {
  switch (status?.trim()) {
    case "active":
      return "success";
    case "ready":
      return "info";
    default:
      return "neutral";
  }
}

export function formatNetworkKindLabel(kind: string): string {
  const normalized = toNetworkKindFilter(kind);
  return normalized ? NETWORK_KIND_LABELS[normalized] : kind;
}

export function getNetworkKindTone(kind: string): NetworkSignalTone {
  const normalized = toNetworkKindFilter(kind);
  return normalized ? NETWORK_KIND_TONES[normalized] : "neutral";
}

export function toNetworkKindFilter(kind: string): Exclude<NetworkKindFilter, "all"> | null {
  if ((NETWORK_SUPPORTED_KINDS as ReadonlyArray<string>).includes(kind)) {
    return kind as Exclude<NetworkKindFilter, "all">;
  }

  return null;
}

export function getMessageAuthorInitial(
  message: Pick<NetworkConversationMessage, "display_name" | "peer_from">
): string {
  const author = (message.display_name ?? message.peer_from ?? "").trim();
  return author.charAt(0).toUpperCase() || "?";
}

export function sortAgentsForNetwork(agents: AgentPayload[]) {
  return [...agents].sort((left, right) => left.name.localeCompare(right.name));
}

export type NetworkWorkState =
  | "submitted"
  | "working"
  | "needs_input"
  | "completed"
  | "failed"
  | "canceled";

const WORK_STATE_VALUES: ReadonlyArray<NetworkWorkState> = [
  "submitted",
  "working",
  "needs_input",
  "completed",
  "failed",
  "canceled",
];

const TERMINAL_WORK_STATES: ReadonlyArray<NetworkWorkState> = ["completed", "failed", "canceled"];

const WORK_STATE_LABELS: Record<NetworkWorkState, string> = {
  submitted: "submitted",
  working: "working",
  needs_input: "needs input",
  completed: "completed",
  failed: "failed",
  canceled: "canceled",
};

export function isNetworkWorkState(value: string | null | undefined): value is NetworkWorkState {
  return typeof value === "string" && (WORK_STATE_VALUES as ReadonlyArray<string>).includes(value);
}

export function isTerminalNetworkWorkState(value: string | null | undefined): boolean {
  return isNetworkWorkState(value) && TERMINAL_WORK_STATES.includes(value);
}

/**
 * `submitted` and `completed` are deliberately silent (`_design.md` §6.6).
 * Returns `null` for those states so callers can suppress the chip entirely.
 */
export function shouldRenderNetworkWorkChip(value: string | null | undefined): boolean {
  if (!isNetworkWorkState(value)) {
    return false;
  }
  return value !== "submitted" && value !== "completed";
}

export function formatNetworkWorkStateLabel(value: string | null | undefined): string {
  if (!isNetworkWorkState(value)) {
    return value ?? "";
  }
  return WORK_STATE_LABELS[value];
}
