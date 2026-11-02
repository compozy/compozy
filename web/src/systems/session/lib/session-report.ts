import type { SessionReportData } from "../types";

export interface ReportProposal {
  title: string;
  body: string;
}

export interface ReportPayload {
  proposals: ReportProposal[];
  detail?: string;
  sessionId?: string;
  tool?: string;
  expiredBy?: string;
}

function text(value: unknown) {
  return typeof value === "string" && value.trim() ? value : undefined;
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
    detail: text(raw.detail) ?? text(raw.reason),
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
