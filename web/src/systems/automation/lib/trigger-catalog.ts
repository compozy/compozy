/**
 * Curated catalog of runtime events a trigger can listen for.
 *
 * Every entry maps to a real `ActivationEnvelope` the AGH runtime emits
 * (`internal/automation/trigger.go`). `fields` and `sample.data` mirror the
 * exact `data.*` keys each builder populates, so the prompt variable bar,
 * filter key options, and live preview stay truthful. The catalog is the single
 * source for event grouping, icons, and per-event data shape — swapping a field
 * name is a one-line change here.
 */

export type EventFamily = "fixed" | "hook" | "ext" | "webhook";

export type EventIconKey =
  | "session-start"
  | "session-stop"
  | "memory"
  | "hook"
  | "webhook"
  | "extension";

export type EnvelopeScope = "global" | "workspace";

export type EnvelopeSource = "observer" | "hook" | "webhook" | "extension";

export interface TriggerEnvelope {
  kind: string;
  scope: EnvelopeScope;
  workspace_id: string;
  source: EnvelopeSource;
  data: Record<string, string>;
}

export interface EventDef {
  /** Stable catalog key used for selection (not always the literal event id). */
  id: string;
  group: EventGroup;
  family: EventFamily;
  icon: EventIconKey;
  label: string;
  description: string;
  /** Fixed `data.*` paths the runtime guarantees for this event. */
  fields: string[];
  /** Realistic envelope used to drive the live preview. */
  sample: TriggerEnvelope;
  /**
   * Open-payload events (memory, webhook, extension) carry arbitrary
   * user-defined `data.*` keys, so filters accept any `data.<path>`.
   */
  openPayload?: boolean;
}

export type EventGroup = "Session lifecycle" | "Memory" | "Hooks" | "External" | "Extensions";

export const EVENT_GROUP_ORDER: readonly EventGroup[] = [
  "Session lifecycle",
  "Memory",
  "Hooks",
  "External",
  "Extensions",
];

/** Top-level envelope keys available to every filter and template. */
export const ENVELOPE_KEYS = ["kind", "scope", "source", "workspace_id"] as const;

const SESSION_FIELDS = [
  "data.session_id",
  "data.session_name",
  "data.session_type",
  "data.agent_name",
  "data.state",
  "data.workspace",
  "data.workspace_id",
  "data.acp_session_id",
  "data.created_at",
  "data.updated_at",
];

export const EVENTS: readonly EventDef[] = [
  {
    id: "session.created",
    group: "Session lifecycle",
    family: "fixed",
    icon: "session-start",
    label: "Session started",
    description: "An agent session is initialized in a workspace.",
    fields: SESSION_FIELDS,
    sample: {
      kind: "session.created",
      scope: "workspace",
      workspace_id: "ws_checkout_api",
      source: "observer",
      data: {
        session_id: "sess_9f2a1c",
        session_name: "nightly-review",
        session_type: "interactive",
        agent_name: "reviewer",
        state: "running",
        workspace: "checkout-api",
        workspace_id: "ws_checkout_api",
        acp_session_id: "acp_71d0e3",
        created_at: "2026-06-01T03:11:08Z",
        updated_at: "2026-06-01T03:11:08Z",
      },
    },
  },
  {
    id: "session.stopped",
    group: "Session lifecycle",
    family: "fixed",
    icon: "session-stop",
    label: "Session stopped",
    description: "A session ends — completed, cancelled, or failed with a reason.",
    fields: [...SESSION_FIELDS, "data.stop_reason", "data.stop_detail"],
    sample: {
      kind: "session.stopped",
      scope: "workspace",
      workspace_id: "ws_checkout_api",
      source: "observer",
      data: {
        session_id: "sess_9f2a1c",
        session_name: "nightly-review",
        session_type: "interactive",
        agent_name: "reviewer",
        state: "stopped",
        workspace: "checkout-api",
        workspace_id: "ws_checkout_api",
        acp_session_id: "acp_71d0e3",
        stop_reason: "error",
        stop_detail: "context deadline exceeded",
        created_at: "2026-06-01T03:11:08Z",
        updated_at: "2026-06-01T03:42:55Z",
      },
    },
  },
  {
    id: "memory.consolidated",
    group: "Memory",
    family: "fixed",
    icon: "memory",
    label: "Memory consolidated",
    description: "Durable-memory consolidation finishes for a workspace.",
    openPayload: true,
    fields: ["data.workspace_id", "data.completed_at"],
    sample: {
      kind: "memory.consolidated",
      scope: "workspace",
      workspace_id: "ws_checkout_api",
      source: "observer",
      data: {
        workspace_id: "ws_checkout_api",
        completed_at: "2026-06-01T04:00:12Z",
      },
    },
  },
  {
    id: "hook.completed",
    group: "Hooks",
    family: "hook",
    icon: "hook",
    label: "Hook completed",
    description: "A named hook finishes — succeeded or failed.",
    fields: [
      "data.session_id",
      "data.session_name",
      "data.session_type",
      "data.agent_name",
      "data.hook_name",
      "data.hook_event",
      "data.hook_source",
      "data.hook_mode",
      "data.hook_outcome",
      "data.dispatch_depth",
      "data.required",
      "data.recorded_at",
      "data.error",
    ],
    sample: {
      kind: "hook.transform.completed",
      scope: "workspace",
      workspace_id: "ws_checkout_api",
      source: "hook",
      data: {
        session_id: "sess_9f2a1c",
        session_name: "nightly-review",
        session_type: "interactive",
        agent_name: "reviewer",
        hook_name: "transform",
        hook_event: "after_agent_run",
        hook_source: "config",
        hook_mode: "blocking",
        hook_outcome: "failed",
        dispatch_depth: "0",
        required: "true",
        recorded_at: "2026-06-01T03:42:55Z",
        error: "schema validation failed",
      },
    },
  },
  {
    id: "webhook",
    group: "External",
    family: "webhook",
    icon: "webhook",
    label: "Incoming webhook",
    description: "A signed external HTTP POST arrives at your endpoint.",
    openPayload: true,
    fields: [
      "data.payload",
      "data.endpoint",
      "data.endpoint_slug",
      "data.webhook_id",
      "data.timestamp",
    ],
    sample: {
      kind: "webhook",
      scope: "global",
      workspace_id: "",
      source: "webhook",
      data: {
        payload: '{"action":"deploy_started","version":"1.4.0"}',
        endpoint: "ci-deploys--wbh_a1b2c3",
        endpoint_slug: "ci-deploys",
        webhook_id: "wbh_a1b2c3",
        timestamp: "2026-06-01T14:30:45Z",
      },
    },
  },
  {
    id: "ext",
    group: "Extensions",
    family: "ext",
    icon: "extension",
    label: "Extension event",
    description: "A custom event published by an installed extension — payload is fully open.",
    openPayload: true,
    fields: [],
    sample: {
      kind: "ext.release.deploy-started",
      scope: "workspace",
      workspace_id: "ws_checkout_api",
      source: "extension",
      data: {
        status: "started",
        repo: "acme/checkout-api",
        version: "1.4.0",
      },
    },
  },
];

const EVENTS_BY_ID = new Map<string, EventDef>(EVENTS.map(event => [event.id, event]));

export function getEventDef(id: string): EventDef | undefined {
  return EVENTS_BY_ID.get(id);
}

export interface EventGroupBucket {
  group: EventGroup;
  events: EventDef[];
}

/** Events filtered by `query` and bucketed into their groups, in canonical order. */
export function listEventGroups(query = ""): EventGroupBucket[] {
  const normalized = query.trim().toLowerCase();
  const matches = (event: EventDef) => {
    if (normalized === "") return true;
    const haystack = `${event.id} ${event.label} ${event.description} ${event.group}`.toLowerCase();
    return haystack.includes(normalized);
  };

  return EVENT_GROUP_ORDER.flatMap(group => {
    const events = EVENTS.filter(event => event.group === group && matches(event));
    return events.length > 0 ? [{ group, events }] : [];
  });
}

/**
 * `data.*` paths available for the event: the fixed runtime fields plus, for
 * open-payload events, the keys present on the sample payload (those sources
 * carry arbitrary user-defined data).
 */
export function availableDataFields(def: EventDef): string[] {
  const out = [...def.fields];
  const fields = new Set(out);
  if (def.openPayload) {
    for (const key of Object.keys(def.sample.data)) {
      const path = `data.${key}`;
      if (!fields.has(path)) {
        fields.add(path);
        out.push(path);
      }
    }
  }
  return out;
}

/** All filter key options for an event: envelope keys + available data paths. */
export function filterKeyOptions(def: EventDef): string[] {
  return [...ENVELOPE_KEYS, ...availableDataFields(def)];
}
