/**
 * Pure derivation of the trigger live-preview view-model from the form draft.
 *
 * Everything the event preview shows — the sample-event JSON with matched
 * filter keys, the selected target, displayed request, match badge, and
 * webhook endpoint/curl — is computed from `draft`.
 * No React, no side effects: the orchestrator wraps a single call in `useMemo`.
 */

import type { CreateAutomationTriggerRequest, UpdateAutomationTriggerRequest } from "../types";
import {
  buildAutomationTriggerRequest,
  projectAutomationTriggerRequest,
  type AutomationEditorMode,
  type AutomationRequestProjection,
} from "./automation-requests";
import { projectAutomationTarget, type AutomationTargetProjection } from "./automation-target";
import {
  ENVELOPE_KEYS,
  availableDataFields,
  getEventDef,
  type EventDef,
  type TriggerEnvelope,
} from "./trigger-catalog";
import {
  formatEventKind,
  parseEventSelection,
  sampleEventKind,
  type EventSelection,
} from "./trigger-event-id";
import {
  renderTemplate,
  tokenizeTemplate,
  type RenderToken,
  type TemplateToken,
} from "./trigger-template";

export interface WorkspaceOption {
  id: string;
  name: string;
}

export interface TriggerPreviewContext {
  mode?: AutomationEditorMode;
  targetIssue?: string | null;
}

export type MatchState = "all" | "match" | "nomatch";

export type JsonRow =
  | { id: string; kind: "open"; indent: number; label?: string }
  | { id: string; kind: "close"; indent: number; comma: boolean }
  | {
      id: string;
      kind: "pair";
      indent: number;
      keyName: string;
      value: string;
      comma: boolean;
      highlighted: boolean;
    };

export interface CurlSegment {
  text: string;
  tone?: "keyword" | "string";
}
export type CurlLine = CurlSegment[];

export interface WebhookPreview {
  url: string;
  curl: CurlLine[];
}

export interface TriggerPreviewModel {
  eventKind: string;
  json: JsonRow[];
  matchState: MatchState;
  matchLabel: string;
  /** The first condition the sample fails, for "Stop reason must be error." */
  failingCondition: { key: string; value: string } | null;
  rendered: RenderToken[];
  templateTokens: TemplateToken[];
  request: AutomationRequestProjection<
    CreateAutomationTriggerRequest | UpdateAutomationTriggerRequest
  >;
  target: AutomationTargetProjection;
  targetIssue: string | null;
  webhook: WebhookPreview | null;
}

type Draft = CreateAutomationTriggerRequest;
type Filter = Record<string, string>;

function getFilter(draft: Draft): Filter {
  return (draft.filter ?? {}) as Filter;
}

function activeFilterEntries(filter: Filter): Array<[string, string]> {
  return Object.entries(filter).filter(([key]) => key.trim() !== "");
}

function buildEnvelope(
  draft: Draft,
  def: EventDef | undefined,
  selection: EventSelection
): TriggerEnvelope {
  if (!def) {
    return {
      kind: draft.event.trim() || "event",
      scope: draft.scope,
      workspace_id: draft.scope === "workspace" ? (draft.workspace_id ?? "") : "",
      source: "observer",
      data: {},
    };
  }

  const isWebhook = def.family === "webhook";
  return {
    kind: sampleEventKind(selection, def.sample.kind),
    scope: isWebhook ? "global" : draft.scope,
    workspace_id: isWebhook ? "" : draft.scope === "workspace" ? (draft.workspace_id ?? "") : "",
    source: def.sample.source,
    data: { ...def.sample.data },
  };
}

function resolveField(env: TriggerEnvelope, key: string): string | undefined {
  if (key.startsWith("data.")) {
    return env.data[key.slice(5)];
  }
  switch (key) {
    case "kind":
      return env.kind;
    case "scope":
      return env.scope;
    case "source":
      return env.source;
    case "workspace_id":
      return env.workspace_id;
    default:
      return undefined;
  }
}

/** The first active condition the sample envelope fails, or `null` when all match. */
function firstFailingCondition(
  env: TriggerEnvelope,
  filter: Filter
): { key: string; value: string } | null {
  for (const [key, value] of activeFilterEntries(filter)) {
    if (String(resolveField(env, key)) !== String(value)) {
      return { key, value: String(value) };
    }
  }
  return null;
}

function appendObject(
  obj: Record<string, unknown>,
  indent: number,
  prefix: string,
  filteredKeys: ReadonlySet<string>,
  rows: JsonRow[]
): void {
  const entries = Object.entries(obj);
  entries.forEach(([key, value], index) => {
    const comma = index < entries.length - 1;
    const fullKey = prefix ? `${prefix}.${key}` : key;
    if (value !== null && typeof value === "object") {
      rows.push({ id: `open:${fullKey}`, kind: "open", indent, label: key });
      appendObject(value as Record<string, unknown>, indent + 1, fullKey, filteredKeys, rows);
      rows.push({ id: `close:${fullKey}`, kind: "close", indent, comma });
    } else {
      rows.push({
        id: `pair:${fullKey}`,
        kind: "pair",
        indent,
        keyName: key,
        value: String(value),
        comma,
        highlighted: filteredKeys.has(fullKey),
      });
    }
  });
}

function buildJsonRows(env: TriggerEnvelope, filteredKeys: ReadonlySet<string>): JsonRow[] {
  const rows: JsonRow[] = [{ id: "open:$", kind: "open", indent: 0 }];
  appendObject(
    {
      kind: env.kind,
      scope: env.scope,
      workspace_id: env.workspace_id,
      source: env.source,
      data: env.data,
    },
    1,
    "",
    filteredKeys,
    rows
  );
  rows.push({ id: "close:$", kind: "close", indent: 0, comma: false });
  return rows;
}

/** Real webhook endpoint path — webhooks are always global. */
export function webhookUrl(draft: Draft): string {
  const slug = draft.endpoint_slug?.trim() || "slug";
  const id = draft.webhook_id?.trim() || "wbh_id";
  return `/api/webhooks/global/${slug}--${id}`;
}

function buildWebhookCurl(url: string): CurlLine[] {
  return [
    [{ text: "curl", tone: "keyword" }, { text: " -X POST \\" }],
    [{ text: `  https://your-host${url} \\` }],
    [
      { text: "  -H " },
      { text: '"X-Compozy-Webhook-Signature: sha256=…"', tone: "string" },
      { text: " \\" },
    ],
    [
      { text: "  -H " },
      { text: '"X-Compozy-Webhook-Timestamp: …"', tone: "string" },
      { text: " \\" },
    ],
    [{ text: "  -d " }, { text: `'{"action":"deploy_started"}'`, tone: "string" }],
  ];
}

/**
 * Drops filter conditions whose key is not valid for the freshly-selected
 * event. Open-payload events keep any `data.<path>`; fixed events keep only
 * envelope keys and their declared data fields.
 */
export function retainValidFilters(filter: Filter, def: EventDef | undefined): Filter {
  if (!def) return filter;
  const valid = new Set<string>([...ENVELOPE_KEYS, ...availableDataFields(def)]);
  const next: Filter = {};
  for (const [key, value] of Object.entries(filter)) {
    if (valid.has(key) || (def.openPayload && key.startsWith("data."))) {
      next[key] = value;
    }
  }
  return next;
}

export function buildTriggerPreview(
  draft: Draft,
  context: TriggerPreviewContext = {}
): TriggerPreviewModel {
  const request = projectAutomationTriggerRequest(draft, context.mode ?? "create");
  const normalizedDraft = buildAutomationTriggerRequest(draft);
  const target = projectAutomationTarget(normalizedDraft);
  const selection = parseEventSelection(normalizedDraft.event);
  const def = getEventDef(selection.catalogId);
  const env = buildEnvelope(normalizedDraft, def, selection);
  const filter = getFilter(normalizedDraft);
  const filteredKeys = new Set(Object.keys(filter).filter(key => key.trim() !== ""));
  const active = activeFilterEntries(filter);

  const failingCondition = active.length === 0 ? null : firstFailingCondition(env, filter);
  let matchState: MatchState;
  let matchLabel: string;
  if (active.length === 0) {
    matchState = "all";
    matchLabel = "starts on every event";
  } else if (failingCondition === null) {
    matchState = "match";
    matchLabel = "matches this sample";
  } else {
    matchState = "nomatch";
    matchLabel = "won't start on this sample";
  }

  const url = def?.family === "webhook" ? webhookUrl(normalizedDraft) : null;

  return {
    eventKind: formatEventKind(selection),
    json: buildJsonRows(env, filteredKeys),
    matchState,
    matchLabel,
    failingCondition,
    rendered: renderTemplate(normalizedDraft.prompt ?? "", env),
    templateTokens: tokenizeTemplate(normalizedDraft.prompt ?? ""),
    request,
    target,
    targetIssue: context.targetIssue ?? null,
    webhook: url ? { url, curl: buildWebhookCurl(url) } : null,
  };
}
