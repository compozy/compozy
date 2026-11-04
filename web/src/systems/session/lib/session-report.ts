import type { SessionReportData } from "../types";

export interface ReportProposal {
  title: string;
  body: string;
}

/**
 * Where an event-derived proposal came from (S13): the trigger that stored the event, the event
 * name, and the daemon's capped summary. The summary is untrusted event data, shown as text only.
 */
export interface ReportEventSource {
  triggerName: string;
  event: string;
  summary?: string;
}

/**
 * One item a check-in source returned that a proposal cites (`kind:"source"`). `sourceRef` is the
 * kind or `script:<name>`; `url` is kept only when it is an http(s) link.
 */
export interface ReportItemSource {
  sourceId: string;
  sourceRef: string;
  itemKey: string;
  url?: string;
}

export interface ReportPayload {
  proposals: ReportProposal[];
  /** A proposed skill's review (S9); the card offers Review, not pick. */
  skillReviewId?: string;
  source?: ReportEventSource;
  /** Every source item the proposals cite, in order, without repeats. */
  itemSources: ReportItemSource[];
  detail?: string;
  sessionId?: string;
  tool?: string;
  action?: string;
  resource?: string;
  expiredBy?: string;
  reason?: string;
  cause?: string;
  blockers: string[];
}

/** Causes of an `interrupted` report about an approval; any other cause is a lost wake. */
const INTERACTION_CAUSES = new Set(["daemon_restart", "provider_interrupted"]);

/**
 * A main session's lost monitor wake (rule 21): an `interrupted` report keyed by the monitor,
 * whose cause is the wake's terminal state rather than an interrupted approval.
 */
export function isLostWake(kind: string, cause: string | undefined, reason?: string) {
  if (kind === "failure") return reason === "wake_lost";
  return kind === "interrupted" && Boolean(cause) && !INTERACTION_CAUSES.has(cause ?? "");
}

function text(value: unknown) {
  return typeof value === "string" && value.trim() ? value : undefined;
}

function eventSource(value: unknown): ReportEventSource | undefined {
  if (typeof value !== "object" || value === null) return undefined;
  const raw = value as Record<string, unknown>;
  if (raw.kind !== "event") return undefined;
  const triggerName = text(raw.trigger_name);
  const event = text(raw.event);
  if (!triggerName && !event) return undefined;
  return { triggerName: triggerName ?? "", event: event ?? "", summary: text(raw.summary) };
}

function webLink(value: unknown) {
  const url = text(value);
  if (!url) return undefined;
  try {
    const parsed = new URL(url);
    return parsed.protocol === "https:" || parsed.protocol === "http:" ? url : undefined;
  } catch {
    return undefined;
  }
}

function itemSource(value: unknown): ReportItemSource | undefined {
  if (typeof value !== "object" || value === null) return undefined;
  const raw = value as Record<string, unknown>;
  const sourceId = text(raw.source_id);
  if (raw.kind !== "source" || !sourceId) return undefined;
  return {
    sourceId,
    sourceRef: text(raw.source_ref) ?? "",
    itemKey: text(raw.item_key) ?? "",
    url: webLink(raw.url),
  };
}

/** The report's own source, then each proposal's, once each. */
function itemSources(raw: Record<string, unknown>, proposals: readonly unknown[]) {
  const candidates = [
    raw.source,
    ...proposals.flatMap(item => {
      const sources = (item as { sources?: unknown } | null)?.sources;
      return Array.isArray(sources) ? sources : [];
    }),
  ];
  const seen = new Set<string>();
  return candidates.flatMap(candidate => {
    const source = itemSource(candidate);
    const key = source ? `${source.sourceId}\u0000${source.itemKey}` : "";
    if (!source || seen.has(key)) return [];
    seen.add(key);
    return [source];
  });
}

/** The bounded per-kind payload, read defensively: a malformed field reads as absent. */
export function reportPayload(data: SessionReportData): ReportPayload {
  const raw = (
    typeof data.payload === "object" && data.payload !== null ? data.payload : {}
  ) as Record<string, unknown>;
  const proposals = Array.isArray(raw.proposals) ? raw.proposals : [];
  return {
    proposals: proposals.flatMap(item => {
      const title = text((item as { title?: unknown } | null)?.title);
      return title ? [{ title, body: text((item as { body?: unknown }).body) ?? "" }] : [];
    }),
    skillReviewId: text(raw.skill_review_id),
    source: eventSource(raw.source),
    itemSources: itemSources(raw, proposals),
    detail: text(raw.detail) ?? text(raw.reason),
    reason: text(raw.reason),
    cause: text(raw.cause),
    action: text(raw.action),
    resource: text(raw.resource),
    blockers: Array.isArray(raw.blockers)
      ? raw.blockers.flatMap(item => (text(item) ? [item as string] : []))
      : [],
    sessionId: text(raw.session_id),
    tool: text(raw.tool),
    expiredBy: text(raw.expired_by),
  };
}

interface ThreadMessageLike {
  role?: string;
  createdAt?: Date;
  content?: unknown;
}

function messageText(message: ThreadMessageLike) {
  if (!Array.isArray(message.content)) return "";
  return message.content
    .map(part =>
      (part as { type?: string; text?: string }).type === "text"
        ? (part as { text?: string }).text
        : ""
    )
    .join("")
    .trim();
}

/** A choice sends its title as an ordinary prompt, so that prompt is the record of the pick. */
export function pickedProposal(
  messages: readonly ThreadMessageLike[],
  proposals: readonly ReportProposal[],
  reportedAt: string
): string | undefined {
  const since = Date.parse(reportedAt);
  const titles = new Set(proposals.map(proposal => proposal.title));
  for (const message of messages) {
    if (message.role !== "user") continue;
    if (message.createdAt && message.createdAt.getTime() < since) continue;
    const said = messageText(message);
    if (titles.has(said)) return said;
  }
  return undefined;
}

interface ReportPartLike {
  type?: string;
  name?: string;
  data?: unknown;
}

function reportPartData(part: unknown): SessionReportData | null {
  const candidate = part as ReportPartLike | null;
  if (!candidate || typeof candidate !== "object") return null;
  const named =
    candidate.type === "data-compozy-report" ||
    (candidate.type === "data" && candidate.name === "compozy-report");
  const data = candidate.data as SessionReportData | undefined;
  return named && data && typeof data.report_id === "string" ? data : null;
}

/** A brake pause: the `failure` report with reason `unattended`. */
export function isUnattendedPause(data: SessionReportData) {
  return data.kind === "failure" && reportPayload(data).reason === "unattended";
}

type PauseIndex = ReadonlyMap<string, SessionReportData>;

const EMPTY_INDEX: PauseIndex = new Map();
// Per message: its brake pauses. Streaming replaces only the growing message, so the rest hit.
const pausesByMessage = new WeakMap<object, readonly SessionReportData[]>();
// Per messages array: every pause report on the thread shares one index.
const indexByThread = new WeakMap<readonly unknown[], PauseIndex>();

function messagePauses(message: ThreadMessageLike): readonly SessionReportData[] {
  const cached = pausesByMessage.get(message);
  if (cached) return cached;
  const pauses = Array.isArray(message.content)
    ? message.content.flatMap(part => {
        const data = reportPartData(part);
        return data && isUnattendedPause(data) ? [data] : [];
      })
    : [];
  pausesByMessage.set(message, pauses);
  return pauses;
}

/**
 * The newest brake pause of each instance in the loaded thread, built once per messages array
 * and shared by every pause report on it.
 */
export function newestUnattendedPauses(
  messages: readonly ThreadMessageLike[] | undefined
): PauseIndex {
  if (!messages) return EMPTY_INDEX;
  const cached = indexByThread.get(messages);
  if (cached) return cached;
  const index = new Map<string, SessionReportData>();
  for (const message of messages) {
    for (const data of messagePauses(message)) {
      const current = index.get(data.instance_id);
      if (!current || data.created_at > current.created_at) index.set(data.instance_id, data);
    }
  }
  indexByThread.set(messages, index);
  return index;
}

/**
 * The newest brake-pause report of an instance in the loaded thread; only it offers a live
 * Resume. Undefined when the thread holds none, so a lone report stays the newest.
 */
export function newestUnattendedReportId(
  messages: readonly ThreadMessageLike[] | undefined,
  instanceId: string
): string | undefined {
  return newestUnattendedPauses(messages).get(instanceId)?.report_id;
}
