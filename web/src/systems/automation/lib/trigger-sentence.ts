/**
 * Detail-page rule language for a persisted trigger.
 *
 * Composes the existing vocabulary sources — `trigger-catalog` labels,
 * `trigger-event-id` parsing, and `automation-target` projections — into the
 * plain-language lede sentence, pause line, and When/If descriptors the
 * trigger detail surface renders. Pure derivation: no React, no side effects.
 */

import { getEventDef, type EventIconKey } from "./trigger-catalog";
import { parseEventSelection, type EventSelection } from "./trigger-event-id";
import { projectAutomationTarget } from "./automation-target";
import type { AutomationTrigger } from "../types";

export interface TriggerSentenceSegment {
  text: string;
  /** Rendered one step dimmer than the surrounding sentence (reference `<em>`). */
  em?: boolean;
}

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

/** Verb rule: agent targets run, loop targets start. */
export function triggerVerb(trigger: AutomationTrigger): "run" | "start" {
  return projectAutomationTarget(trigger).kind === "loop" ? "start" : "run";
}

/** Display name of the configured target (agent or loop). */
export function triggerTargetName(trigger: AutomationTrigger): string {
  const target = projectAutomationTarget(trigger);
  return target.kind === "loop" ? target.loopName : target.agentName;
}

function humanizeFilterKey(key: string): string {
  return key.replace(/^data\./, "").replaceAll(/[._]/g, " ");
}

function filterEntries(trigger: AutomationTrigger): Array<[string, string]> {
  return Object.entries(trigger.filter ?? {}).filter(([key]) => key.trim() !== "");
}

function eventClause(
  trigger: AutomationTrigger,
  selection: EventSelection
): TriggerSentenceSegment[] {
  switch (selection.catalogId) {
    case "session.created":
      return [{ text: "a session starts" }];
    case "session.stopped":
      return [{ text: "a session stops" }];
    case "memory.consolidated":
      return [{ text: "memory is consolidated" }];
    case "hook.completed":
      return [
        { text: "the " },
        { text: selection.hookName, em: true },
        { text: " hook completes" },
      ];
    case "webhook": {
      const slug = trigger.endpoint_slug?.trim();
      return slug
        ? [{ text: "a signed POST arrives at " }, { text: slug, em: true }]
        : [{ text: "a signed POST arrives" }];
    }
    default:
      return [{ text: trigger.event, em: true }, { text: " fires" }];
  }
}

function workspaceClause(
  trigger: AutomationTrigger,
  selection: EventSelection,
  workspaceName: string | null
): TriggerSentenceSegment[] {
  const name = workspaceName ?? trigger.workspace_id ?? "";
  if (trigger.scope !== "workspace" || name === "") {
    // Webhook global paths carry no workspace clause; other events read "in any workspace".
    return selection.family === "webhook" ? [] : [{ text: " in any workspace" }];
  }
  // The webhook sentence keeps the workspace plain; the slug carries the emphasis.
  return selection.family === "webhook"
    ? [{ text: ` in ${name}` }]
    : [{ text: " in " }, { text: name, em: true }];
}

function ifClauseSentence(trigger: AutomationTrigger): string {
  const entries = filterEntries(trigger);
  if (entries.length === 0) return "";
  if (entries.length === 1) {
    const [key, value] = entries[0];
    return `, if the ${humanizeFilterKey(key)} is ${value}`;
  }
  const clauses = entries.map(([key, value]) => `${humanizeFilterKey(key)} is ${value}`);
  return `, if ${clauses.join(" and ")}`;
}

/**
 * The page-head sentence: `When <event> in <workspace>, if <filter>, run <target>.`
 */
export function buildTriggerLede(
  trigger: AutomationTrigger,
  workspaceName: string | null
): TriggerSentenceSegment[] {
  const selection = parseEventSelection(trigger.event);
  const segments: TriggerSentenceSegment[] = [{ text: "When " }];
  segments.push(...eventClause(trigger, selection));
  segments.push(...workspaceClause(trigger, selection, workspaceName));
  const ifClause = ifClauseSentence(trigger);
  if (ifClause !== "") segments.push({ text: ifClause });
  segments.push({ text: `, ${triggerVerb(trigger)} ` });
  segments.push({ text: triggerTargetName(trigger), em: true });
  segments.push({ text: "." });
  return segments;
}

/** Shown under the sentence while the trigger is disabled. */
export function triggerPauseLine(trigger: AutomationTrigger): string {
  const noun = trigger.event === "webhook" ? "deliveries" : "events";
  const action = triggerVerb(trigger) === "start" ? "start the loop" : "run the agent";
  return `Matching ${noun} will not ${action} until this trigger is enabled.`;
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
      ? `In workspace ${workspaceName ?? trigger.workspace_id ?? ""}`.trimEnd()
      : "In any workspace";
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
        sub: "Hook name is in the event id",
      };
    case "webhook":
      return {
        icon,
        headline: "Incoming webhook",
        eventId: trigger.event,
        sub: `${trigger.scope === "workspace" ? "Workspace" : "Global"} path — always answers on this daemon. Public URL is a separate fact.`,
      };
    default:
      return {
        icon,
        headline: "Extension event",
        eventId: trigger.event,
        sub: "Across the runtime",
      };
  }
}

export function describeTriggerIf(trigger: AutomationTrigger): TriggerIfDescriptor {
  const entries = filterEntries(trigger);
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
