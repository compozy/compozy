/**
 * Operator-facing runtime facts for the trigger Inspect sheet.
 *
 * Two panes, one model: the diagnostics tiles are the raw runtime enums the
 * calmer detail page deliberately keeps out of the default read, and the
 * envelope is the activation payload the daemon would hand this trigger —
 * built from the same catalog sample the create flow previews, never invented.
 */

import { getEventDef, type TriggerEnvelope } from "./trigger-catalog";
import { parseEventSelection } from "./trigger-event-id";
import { projectAutomationTarget } from "./automation-target";
import type { AutomationRetry, AutomationTrigger } from "../types";

export interface TriggerDiagnosticTile {
  id: string;
  label: string;
  value: string;
}

function filterEntries(trigger: AutomationTrigger): Array<[string, string]> {
  return Object.entries(trigger.filter ?? {}).filter(([key]) => key.trim() !== "");
}

function targetTile(trigger: AutomationTrigger): string {
  const target = projectAutomationTarget(trigger);
  return target.kind === "loop" ? `loop · ${target.loopName}` : `agent · ${target.agentName}`;
}

/** Raw dispatch-retry internals, the shape the inspection surface exists for. */
function retryTile(retry: AutomationRetry): string {
  if (retry.strategy === "none") return "none";
  return `backoff · ${retry.max_retries} · ${retry.base_delay}`;
}

/**
 * Six tiles: identity, provenance, and the one reliability fact that differs by
 * event — a webhook's secret presence, everything else's retry policy.
 */
export function buildTriggerDiagnostics(trigger: AutomationTrigger): TriggerDiagnosticTile[] {
  const isWebhook = trigger.event === "webhook";
  return [
    { id: "id", label: "Id", value: trigger.name },
    { id: "source", label: "Source", value: trigger.source.toUpperCase() },
    { id: "scope", label: "Scope", value: trigger.scope.toUpperCase() },
    { id: "kind", label: "Kind", value: trigger.event },
    { id: "target", label: "Target", value: targetTile(trigger) },
    isWebhook
      ? {
          id: "secret",
          label: "Signing secret",
          value: trigger.webhook_secret_present ? "present" : "absent",
        }
      : { id: "retry", label: "Retry", value: retryTile(trigger.retry) },
  ];
}

/**
 * The internals the tiles cannot hold, as one quiet line. Every clause is a
 * persisted fact; nothing is added when the field is unset.
 */
export function buildTriggerDiagnosticsNote(trigger: AutomationTrigger): string {
  const parts: string[] = [];
  if (trigger.event === "webhook" && trigger.endpoint_slug && trigger.webhook_id) {
    parts.push(`Slug ${trigger.endpoint_slug} · id ${trigger.webhook_id}.`);
  }
  const entries = filterEntries(trigger);
  if (entries.length > 0) {
    parts.push(`Filter ${entries.map(([key, value]) => `${key}=${value}`).join(" AND ")}.`);
  }
  const target = projectAutomationTarget(trigger);
  if (target.kind === "loop") {
    const mapping = Object.entries(target.inputMapping);
    if (mapping.length > 0) {
      parts.push(`Mapping ${mapping.map(([key, path]) => `${key} ← ${path}`).join(", ")}.`);
    }
  }
  parts.push(`Fire limit ${trigger.fire_limit.max} / ${trigger.fire_limit.window}.`);
  parts.push("Target kind is immutable after create.");
  return parts.join(" ");
}

/** Sheet description — what this read is and what it deliberately withholds. */
export function triggerInspectDescription(trigger: AutomationTrigger): string {
  if (trigger.event === "webhook") {
    return "Runtime fields. The signing secret is write-only — this read shows presence, never the value.";
  }
  if (projectAutomationTarget(trigger).kind === "loop") {
    return "Runtime fields. Target kind is immutable after create.";
  }
  return "Runtime fields for this trigger. The envelope matches the create-flow sample for this event.";
}

/**
 * A sample activation envelope for this trigger: the catalog's realistic
 * payload for the event, overlaid with the trigger's own filter values so the
 * sample is one the rule would actually match, plus the webhook identity the
 * runtime attaches to a delivery.
 */
export function buildTriggerEnvelopeSample(trigger: AutomationTrigger): TriggerEnvelope {
  const selection = parseEventSelection(trigger.event);
  const def = getEventDef(selection.catalogId);
  const data: Record<string, string> = { ...def?.sample.data };
  for (const [key, value] of filterEntries(trigger)) {
    if (key.startsWith("data.")) data[key.slice(5)] = value;
  }
  if (trigger.event === "webhook") {
    if (trigger.endpoint_slug) data.endpoint_slug = trigger.endpoint_slug;
    if (trigger.webhook_id) data.webhook_id = trigger.webhook_id;
  }
  return {
    kind: trigger.event,
    scope: trigger.scope,
    workspace_id: trigger.scope === "workspace" ? (trigger.workspace_id ?? "") : "",
    source: def?.sample.source ?? "observer",
    data,
  };
}

/** Pretty-printed envelope for the Envelope pane's code block. */
export function formatTriggerEnvelope(trigger: AutomationTrigger): string {
  return JSON.stringify(buildTriggerEnvelopeSample(trigger), null, 2);
}
