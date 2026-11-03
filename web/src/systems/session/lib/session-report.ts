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

export interface ReportPayload {
  proposals: ReportProposal[];
  /** A proposed skill's review (S9); the card offers Review, not pick. */
  skillReviewId?: string;
  source?: ReportEventSource;
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

/**
 * The newest brake-pause report of an instance in the loaded thread; only it offers a live
 * Resume. Undefined when the thread holds none, so a lone report stays the newest.
 */
export function newestUnattendedReportId(
  messages: readonly ThreadMessageLike[] | undefined,
  instanceId: string
): string | undefined {
  let newest: SessionReportData | undefined;
  for (const message of messages ?? []) {
    if (!Array.isArray(message.content)) continue;
    for (const part of message.content) {
      const data = reportPartData(part);
      if (!data || data.instance_id !== instanceId || !isUnattendedPause(data)) continue;
      if (!newest || data.created_at > newest.created_at) newest = data;
    }
  }
  return newest?.report_id;
}
