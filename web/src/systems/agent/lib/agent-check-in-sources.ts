/** Check-in sources (`context.sources`): reading them from the file, and saying how each one went. */
import type {
  AgentHeartbeatSourceCatalog,
  AgentHeartbeatSourceDecl,
  AgentInstanceSourceOutcome,
  AgentInstanceSourceStatus,
} from "../types";
import type { FrontmatterMap, FrontmatterValue } from "./agent-frontmatter";

export type CheckInSource = AgentHeartbeatSourceDecl;

const SOURCE_KEYS = new Set(["id", "kind", "script", "with"]);

function readSource(value: FrontmatterValue): CheckInSource | null {
  if (typeof value !== "object" || value === null || Array.isArray(value)) return null;
  const entry = value as FrontmatterMap;
  if (!Object.keys(entry).every(key => SOURCE_KEYS.has(key))) return null;
  const { id, kind, script } = entry;
  if (typeof id !== "string") return null;
  if (kind !== undefined && typeof kind !== "string") return null;
  if (script !== undefined && typeof script !== "string") return null;
  // Every key the file has is kept as written, so a save never drops a value the form doesn't show.
  return { ...entry } as CheckInSource;
}

/** The file's sources in order; `null` when one of them is something the form can't carry. */
export function readCheckInSources(value: FrontmatterValue | undefined): CheckInSource[] | null {
  if (value === undefined || value === null) return [];
  if (!Array.isArray(value)) return null;
  const sources: CheckInSource[] = [];
  for (const item of value as readonly FrontmatterValue[]) {
    const source = readSource(item);
    if (!source) return null;
    sources.push(source);
  }
  return sources;
}

type Catalog = Pick<AgentHeartbeatSourceCatalog, "kinds"> | null | undefined;

/** A kind's own title, or the script's name; the raw kind until the catalog answers. */
export function checkInSourceTitle(
  source: Pick<CheckInSource, "id" | "kind" | "script">,
  catalog?: Catalog
) {
  if (source.script) return source.script;
  if (!source.kind) return source.id;
  return catalog?.kinds.find(kind => kind.kind === source.kind)?.title ?? source.kind;
}

/** A report's `source_ref` is the kind, or `script:<name>`. */
export function checkInSourceRefTitle(ref: string, fallback: string, catalog?: Catalog) {
  if (ref.startsWith("script:")) return ref.slice("script:".length) || fallback;
  return ref ? checkInSourceTitle({ id: fallback, kind: ref }, catalog) : fallback;
}

/** The parameters worth a glance, such as `compozy/compozy`. */
export function checkInSourceParams(source: Pick<CheckInSource, "with">) {
  const values = source.with;
  if (typeof values !== "object" || values === null || Array.isArray(values)) return "";
  return Object.values(values as Record<string, unknown>)
    .filter((value): value is string => typeof value === "string" && value.trim() !== "")
    .slice(0, 2)
    .join(" · ");
}

export type SourceTone = "neutral" | "muted" | "destructive" | "warning";

export interface SourceStatusView {
  label: string;
  tone: SourceTone;
  /** The daemon's error code, for a tooltip. */
  code?: string;
}

const MINUTE = 60_000;

function checkedAgo(iso: string | undefined, now: number) {
  const at = iso ? Date.parse(iso) : Number.NaN;
  if (Number.isNaN(at)) return "Checked";
  const minutes = Math.floor(Math.max(0, now - at) / MINUTE);
  if (minutes < 1) return "Checked just now";
  if (minutes < 60) return `Checked ${minutes} min ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `Checked ${hours} hr ago`;
  const days = Math.floor(hours / 24);
  return `Checked ${days} ${days === 1 ? "day" : "days"} ago`;
}

const ERROR_WORDS: Record<string, string> = {
  source_credential_absent: "it isn't signed in",
  source_credential_expired: "its sign-in expired",
  source_rate_limited: "it was asked too often, so it waits",
  source_timeout: "it took too long",
};

function couldNot(error: AgentInstanceSourceStatus["error"]) {
  const detail = error?.detail?.trim() || ERROR_WORDS[error?.code ?? ""] || "something went wrong";
  return `Couldn't check: ${detail}`;
}

function extensionOf(kind: string | undefined) {
  return kind?.split("/")[0] || "";
}

/** One shared reading for a source's stored status and a dry run's outcome. */
function statusView(
  status: string,
  source: Pick<CheckInSource, "kind" | "script">,
  error: AgentInstanceSourceStatus["error"],
  ok: () => string,
  hash?: string
): SourceStatusView {
  switch (status) {
    case "ok":
      return { label: ok(), tone: "neutral" };
    case "failing":
      return { label: couldNot(error), tone: "destructive", code: error?.code };
    case "disabled":
      return { label: "Scripts are turned off", tone: "warning", code: error?.code };
    case "unapproved":
      return {
        label: hash
          ? `Waiting for your approval · ${hash.slice(0, 8)}`
          : "Waiting for your approval",
        tone: "warning",
        code: error?.code,
      };
    case "unavailable": {
      const extension = extensionOf(source.kind);
      return {
        label: extension ? `Extension ${extension} is off` : "Not available right now",
        tone: "warning",
        code: error?.code,
      };
    }
    default:
      return { label: "Not checked yet", tone: "muted" };
  }
}

/**
 * The source's row status; a source the daemon hasn't read yet is "Not checked yet", never OK.
 * `hash` is the script's current sha256, shown short while it waits for approval.
 */
export function checkInSourceStatus(
  source: Pick<CheckInSource, "kind" | "script">,
  state: AgentInstanceSourceStatus | undefined,
  now: number,
  hash?: string
): SourceStatusView {
  if (!state) return { label: "Not checked yet", tone: "muted" };
  const ok = () => {
    const checked = checkedAgo(state.last_success_at || state.last_polled_at, now);
    return state.last_item_count > 0 ? `${checked} · ${state.last_item_count} new` : checked;
  };
  return statusView(state.status, source, state.error, ok, hash);
}

/** A try's read of one source, in the same words as its row. */
export function checkInSourceOutcome(
  source: Pick<CheckInSource, "kind" | "script">,
  outcome: AgentInstanceSourceOutcome
): SourceStatusView {
  return statusView(outcome.result, source, outcome.error, () => {
    if (outcome.items === 0) return "Nothing new";
    return `${outcome.items}${outcome.more ? "+" : ""} new`;
  });
}
