import { Check, RotateCcw, ShieldCheck, type LucideIcon } from "lucide-react";

import { formatRelativeTime, type PillTone } from "@compozy/ui";

import type { LoopNodeInventoryItem, LoopNodeInventoryState } from "../types";
import { readQuarantineEntry } from "./loop-quarantine-entry";
import { humanizeLoopNodeId } from "./loop-node-labels";

/**
 * Inventory presentation model (US-024 AC-2, VC-R5). The daemon owns the page,
 * its order, and its filters; this module only turns one row into the sentence
 * an operator reads — the reason it is in this state and how long it has been
 * there, both taken from the row's own fields.
 */

export const LOOP_NODE_INVENTORY_LABELS = {
  waiting: "Waiting",
  quarantined: "Set aside",
  attention: "Needs attention",
  retrying: "Retrying",
} as const satisfies Record<LoopNodeInventoryState, string>;

export const LOOP_NODE_INVENTORY_TONES = {
  waiting: "info",
  quarantined: "danger",
  attention: "warning",
  retrying: "neutral",
} as const satisfies Record<LoopNodeInventoryState, PillTone>;

const INVENTORY_EMPTY_ICONS = {
  waiting: Check,
  quarantined: ShieldCheck,
  attention: Check,
  retrying: RotateCcw,
} as const satisfies Record<LoopNodeInventoryState, LucideIcon>;

export interface LoopNodeInventoryRowView {
  key: string;
  runId: string;
  loopName: string;
  nodeId: string;
  label: string;
  itemIndex: number;
  generation: number;
  state: LoopNodeInventoryState;
  /** Plain-language reason this row is in this state. */
  reason: string;
  /** `49m` — time in this state, from `state_at`. */
  age: string;
  /** Seconds in this state, from `state_at` or the daemon's `age_seconds`. */
  ageSeconds: number;
  /** Plain secondary line under the step name. */
  micro: string;
}

export { isLoopNodeInventoryState, LOOP_NODE_INVENTORY_STATES } from "./loop-node-inventory-state";

/** `1080` → `18m`; the inventory's age column form. */
export function inventoryAgeLabel(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return "—";
  if (seconds < 60) return `${Math.floor(seconds)}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h`;
  return `${Math.floor(hours / 24)}d`;
}

const WAIT_REASON: Record<string, string> = {
  timer: "timer",
  event: "event",
  approval_escalation: "approval",
};

const ATTENTION_REASON: Record<string, string> = {
  silence: "No activity for a while",
  dependency_quarantined: "Waiting on a step that was set aside",
  expired_wait: "Waited too long with no fallback",
};

function waitReason(item: LoopNodeInventoryItem, nowMs: number): string {
  const wait = item.wait;
  if (!wait) return "waiting";
  const kind = WAIT_REASON[wait.kind] ?? wait.kind;
  if (wait.kind === "timer" && wait.resume_at) {
    return `${kind} — resumes ${formatRelativeTime(wait.resume_at, nowMs)}`;
  }
  if (wait.kind === "approval_escalation") {
    return wait.escalation_cursor > 0
      ? `${kind} — escalated ${wait.escalation_cursor}×`
      : `${kind} — awaiting a decision`;
  }
  return kind;
}

function quarantineReason(item: LoopNodeInventoryItem): string {
  const entry = readQuarantineEntry(item.control?.quarantine_entry);
  if (!entry) return "set aside";
  const episodes = entry.episodes.length;
  const attempts = entry.attemptCount;
  const parts = [
    attempts > 0 ? `${attempts} tries` : null,
    episodes > 1 ? `${episodes} times` : null,
    entry.target === "" ? null : entry.target,
  ].filter(Boolean);
  return parts.length === 0 ? "set aside" : parts.join(" · ");
}

function retryReason(item: LoopNodeInventoryItem, nowMs: number): string {
  const output = item.output;
  if (!output) return "retrying";
  const parts = [
    output.attempt ? `attempt ${output.attempt}` : null,
    output.failure_class ? `${output.failure_class} failure` : null,
    output.next_attempt_at ? `next ${formatRelativeTime(output.next_attempt_at, nowMs)}` : null,
  ].filter(Boolean);
  return parts.length === 0 ? "retrying" : parts.join(" · ");
}

function rowReason(item: LoopNodeInventoryItem, nowMs: number): string {
  switch (item.state) {
    case "waiting":
      return waitReason(item, nowMs);
    case "quarantined":
      return quarantineReason(item);
    case "attention":
      return (
        ATTENTION_REASON[item.control?.attention_flag ?? ""] ??
        item.control?.attention_reason ??
        "flagged for attention"
      );
    case "retrying":
      return retryReason(item, nowMs);
    default:
      return item.state;
  }
}

/**
 * Seconds this row has been in its state. `waiting` rows carry the daemon's own
 * `age_seconds`; everything else derives from `state_at` against the supplied
 * clock, so a fixture and the live page agree without a wall-clock read here.
 */
export function inventoryAgeSeconds(item: LoopNodeInventoryItem, nowMs: number): number {
  if (item.state === "waiting" && item.wait) return item.wait.age_seconds;
  const stateAt = Date.parse(item.state_at);
  if (Number.isNaN(stateAt)) return 0;
  return Math.max(0, Math.floor((nowMs - stateAt) / 1000));
}

export function buildInventoryRow(
  item: LoopNodeInventoryItem,
  nowMs: number
): LoopNodeInventoryRowView {
  const ageSeconds = inventoryAgeSeconds(item, nowMs);
  return {
    key: `${item.loop_run_id}:${item.node_id}:${item.item_index}`,
    runId: item.loop_run_id,
    loopName: item.loop_name,
    nodeId: item.node_id,
    label: humanizeLoopNodeId(item.node_id),
    itemIndex: item.item_index,
    generation: item.generation,
    state: item.state,
    reason: rowReason(item, nowMs),
    age: inventoryAgeLabel(ageSeconds),
    ageSeconds,
    micro: `Round ${item.generation}`,
  };
}

/** Truthful, filter-aware empty copy — never "nothing is waiting" under a filter. */
export function inventoryEmptyCopy(
  state: LoopNodeInventoryState,
  filtered: boolean
): { title: string; description: string; icon: LucideIcon } {
  const icon = INVENTORY_EMPTY_ICONS[state];
  if (filtered) {
    return {
      title: "Nothing matches these filters",
      description: `No ${LOOP_NODE_INVENTORY_LABELS[state].toLowerCase()} steps match the Loop and run you picked. Clear the filters to see the whole project.`,
      icon,
    };
  }
  switch (state) {
    case "waiting":
      return {
        title: "Nothing is waiting",
        description:
          "No step in this project is waiting on a timer, an event, or a decision right now.",
        icon,
      };
    case "quarantined":
      return {
        title: "Nothing is set aside",
        description: "No step has been set aside. Steps that keep failing show up here.",
        icon,
      };
    case "attention":
      return {
        title: "Nothing needs attention",
        description:
          "No step is flagged. Steps show up here when they go quiet or wait on a step that was set aside.",
        icon,
      };
    case "retrying":
      return {
        title: "Nothing is retrying",
        description:
          "No step is retrying. Retrying steps show their attempt and next try time here.",
        icon,
      };
    default:
      return { title: "Nothing here", description: "", icon: Check };
  }
}
