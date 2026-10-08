/**
 * Detail-page rule descriptors for a persisted trigger (Starts / Only if
 * descriptors, webhook path). The sentence itself is
 * `describeAutomation()` in `automation-sentence.ts`. Pure derivation.
 */

import { getEventDef, type EventIconKey } from "./trigger-catalog";
import { parseEventSelection } from "./trigger-event-id";
import { humanizeFilterKey, triggerFilterEntries } from "./trigger-filter";
import type { AutomationTrigger } from "../types";

export interface TriggerWhenDescriptor {
  icon: EventIconKey;
  headline: string;
  /** Raw runtime event id shown as the mono pill. */
  eventId: string;
  sub: string | null;
}

export interface TriggerIfClause {
  /** Lead-in prose ending in "is", e.g. `Stop reason is` / `and branch is`. */
  lead: string;
  /** Exact-match value rendered as a badge. */
  value: string;
}

/** Quiet path line under the If row: prose lead plus the raw filter paths. */
export interface TriggerIfPathNote {
  lead: string;
  paths: string[];
}

export interface TriggerIfDescriptor {
  /** Empty when the trigger has no filter ("Any event of this kind"). */
  clauses: TriggerIfClause[];
  note: TriggerIfPathNote | null;
}

/** Event display label for the subhead pill and the rail Event row. */
export function triggerEventLabel(trigger: AutomationTrigger): string {
  const selection = parseEventSelection(trigger.event);
  return getEventDef(selection.catalogId)?.label ?? trigger.event;
}

export function describeTriggerWhen(
  trigger: AutomationTrigger,
  workspaceName: string | null
): TriggerWhenDescriptor {
  const selection = parseEventSelection(trigger.event);
  const def = getEventDef(selection.catalogId);
  const icon = def?.icon ?? "extension";
  const workspaceSub =
    trigger.scope === "workspace"
      ? `In project ${workspaceName ?? trigger.workspace_id ?? ""}`.trimEnd()
      : "In any project";
  switch (selection.catalogId) {
    case "session.created":
      return { icon, headline: "A session starts", eventId: trigger.event, sub: workspaceSub };
    case "session.stopped":
      return { icon, headline: "A session stops", eventId: trigger.event, sub: workspaceSub };
    case "memory.consolidated":
      return { icon, headline: "Memory consolidated", eventId: trigger.event, sub: workspaceSub };
    case "hook.completed":
      return {
        icon,
        headline: `Hook ${selection.hookName} completed`,
        eventId: trigger.event,
        sub: "Runs when this hook finishes",
      };
    case "webhook":
      return {
        icon,
        headline: "Another app calls this link",
        eventId: trigger.event,
        sub: "Requests must be signed with the secret. Unsigned or older-than-5-minute requests are rejected.",
      };
    default:
      return {
        icon,
        headline: "Extension event",
        eventId: trigger.event,
        sub: "Anywhere in CompozyOS",
      };
  }
}

export function describeTriggerIf(trigger: AutomationTrigger): TriggerIfDescriptor {
  const entries = triggerFilterEntries(trigger);
  if (entries.length === 0) {
    return { clauses: [], note: null };
  }
  const clauses = entries.map(([key, value], index) => {
    const prose = humanizeFilterKey(key);
    const lead =
      index === 0 ? `${prose.charAt(0).toUpperCase()}${prose.slice(1)} is` : `and ${prose} is`;
    return { lead, value };
  });
  const paths = entries.map(([key]) => key);
  const note = entries.length === 1 ? { lead: "Exact match on", paths } : { lead: "AND of", paths };
  return { clauses, note };
}

/**
 * Local delivery path for a webhook trigger, from its persisted fields only.
 * Mirrors the runtime route `/api/webhooks/{global|workspaces/{ws}}/{slug}--{id}`;
 * null when the trigger is not webhook-backed or the identity is incomplete.
 */
export function triggerWebhookPath(trigger: AutomationTrigger): string | null {
  if (trigger.event !== "webhook") return null;
  const slug = trigger.endpoint_slug?.trim();
  const webhookId = trigger.webhook_id?.trim();
  if (!slug || !webhookId) return null;
  const base =
    trigger.scope === "workspace" && trigger.workspace_id
      ? `workspaces/${trigger.workspace_id}`
      : "global";
  return `/api/webhooks/${base}/${slug}--${webhookId}`;
}

/**
 * Signed-delivery example for the local webhook path. Header names are the ones
 * the daemon actually verifies (`internal/api/core/automation.go`), not the
 * shortened prototype spelling.
 */
export function triggerWebhookCurl(path: string): string {
  return [
    `curl -sS -X POST "$COMPOZY_BASE${path}" \\`,
    `  -H "Content-Type: application/json" \\`,
    `  -H "X-Compozy-Webhook-Timestamp: <unix>" \\`,
    `  -H "X-Compozy-Webhook-Signature: sha256=<hmac>" \\`,
    `  -H "X-Compozy-Webhook-Delivery-ID: <unique>" \\`,
    `  -d '<json payload>'`,
  ].join("\n");
}
