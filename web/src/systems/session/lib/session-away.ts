import type { AgentInstanceAway, AgentInstanceReport } from "@/systems/agent";

/** Client-written away lines: counts from the daemon, no model call. */
export type AwayBeat = "quiet" | "reported" | "failed" | "needs-you";

export interface AwayEvent {
  id: string;
  tone: "failed" | "unanswered" | "reported";
  title: string;
  description: string;
  time: string;
}

export interface AwayDigestView {
  firstReportId: string;
  /** Told in the digest, so they don't repeat as cards. What needs you stays out of this set. */
  foldedIds: ReadonlySet<string>;
  headline: string;
  summary: string;
  stripLabel: string;
  beats: readonly AwayBeat[];
  hours: readonly string[];
  key: readonly { beat: AwayBeat; label: string; count: number }[];
  events: readonly AwayEvent[];
}

/** Below this, the reports read fine as cards; the digest is for a lot having happened. */
const DIGEST_MIN_CHECK_INS = 6;
const SLOT_MS = 15 * 60 * 1000;
const MAX_SLOTS = 48;

const WORDS = [
  "No",
  "One",
  "Two",
  "Three",
  "Four",
  "Five",
  "Six",
  "Seven",
  "Eight",
  "Nine",
  "Ten",
  "Eleven",
  "Twelve",
];

export function countWord(count: number) {
  return WORDS[count] ?? String(count);
}

export function reportClock(iso: string) {
  const at = new Date(iso);
  if (Number.isNaN(at.getTime())) return "";
  return `${String(at.getHours()).padStart(2, "0")}:${String(at.getMinutes()).padStart(2, "0")}`;
}

function beatOf(report: AgentInstanceReport): AwayBeat {
  if (report.kind === "proposal") return "needs-you";
  if (report.kind === "rotation_deferred") return "reported";
  return "failed";
}

function eventOf(report: AgentInstanceReport): AwayEvent {
  const time = reportClock(report.created_at);
  if (report.kind === "approval_expired") {
    return {
      id: report.id,
      tone: "unanswered",
      title: "A request went unanswered",
      description: report.tool ? `${report.tool} wasn't allowed.` : "It wasn't allowed.",
      time,
    };
  }
  if (report.kind === "interrupted") {
    return {
      id: report.id,
      tone: "failed",
      title: "Some work stopped",
      description: "CompozyOS restarted while it was waiting for an answer.",
      time,
    };
  }
  if (report.kind === "failure") {
    return {
      id: report.id,
      tone: "failed",
      title: "A check didn't finish",
      description: report.detail || "It stopped before it was done.",
      time,
    };
  }
  return {
    id: report.id,
    tone: "reported",
    title: "Still in the same conversation",
    description: "You don't need to do anything.",
    time,
  };
}

function hourLabels(start: number, end: number) {
  return [0, 1, 2, 3].map(step =>
    reportClock(new Date(start + ((end - start) * step) / 3).toISOString())
  );
}

/** `null` when the absence was short or empty: the reports stand on their own. */
export function deriveAwayDigest(
  away: AgentInstanceAway,
  agentLabel: string,
  now: number
): AwayDigestView | null {
  const reports = [...away.reports].sort((a, b) => a.created_at.localeCompare(b.created_at));
  const first = reports[0];
  const total = reports.length + away.quiet_check_ins;
  if (!first || total < DIGEST_MIN_CHECK_INS) return null;

  const firstAt = Date.parse(first.created_at);
  const start = Math.min(Date.parse(away.last_seen_at ?? first.created_at), firstAt);
  const slots = Math.min(Math.max(Math.ceil((now - start) / SLOT_MS), 8), MAX_SLOTS);
  const beats: AwayBeat[] = Array.from({ length: slots }, () => "quiet");
  for (const report of reports) {
    const slot = Math.floor(((Date.parse(report.created_at) - start) / (now - start)) * slots);
    beats[Math.min(Math.max(slot, 0), slots - 1)] = beatOf(report);
  }

  const needing = reports.filter(report => beatOf(report) === "needs-you");
  const worth = reports.filter(report => beatOf(report) === "reported");
  const wrong = reports.filter(report => beatOf(report) === "failed");
  const told = [...wrong, ...worth].sort((a, b) => a.created_at.localeCompare(b.created_at));
  const hours = Math.floor((now - start) / (60 * 60 * 1000));
  const span = hours === 1 ? "hour" : "hours";
  const decisions =
    needing.length === 0
      ? "nothing for you to decide."
      : `${countWord(needing.length).toLowerCase()} ${needing.length === 1 ? "decision" : "decisions"} for you.`;
  const mostlyQuiet = away.quiet_check_ins >= reports.length;
  const lead = mostlyQuiet ? "quiet " : "";

  return {
    firstReportId: first.id,
    foldedIds: new Set(told.map(report => report.id)),
    headline: `${countWord(hours)} ${lead}${span}, ${decisions}`,
    summary: mostlyQuiet
      ? `${agentLabel} checked in ${total} times. Most had nothing to report.`
      : `${agentLabel} checked in ${total} times.`,
    stripLabel: `Check-ins while you were away: ${needing.length} needs you, ${worth.length} worth knowing, ${wrong.length} went wrong, ${away.quiet_check_ins} with nothing to report`,
    beats,
    hours: hourLabels(start, now),
    key: [
      { beat: "needs-you", label: "Needs you", count: needing.length },
      { beat: "reported", label: "Worth knowing", count: worth.length },
      { beat: "failed", label: "Went wrong", count: wrong.length },
      { beat: "quiet", label: "Nothing to report", count: away.quiet_check_ins },
    ],
    events: told.map(eventOf),
  };
}
