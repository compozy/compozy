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
  source_script_not_executable: "the file isn't executable",
};

function couldNot(error: AgentInstanceSourceStatus["error"]) {
  const detail = error?.detail?.trim() || ERROR_WORDS[error?.code ?? ""] || "something went wrong";
  return `Couldn't check: ${detail}`;
}

function extensionOf(kind: string | undefined) {
  return kind?.split("/")[0] || "";
}

/** The first 8 hex digits of a `sha256:<hex>` hash. */
export function shortHash(hash: string) {
  return hash.replace(/^sha256:/, "").slice(0, 8);
}

/** One shared reading for a source's stored status and a dry run's outcome. */
function statusView(
  status: string,
  source: Pick<CheckInSource, "kind" | "script">,
  error: AgentInstanceSourceStatus["error"],
  ok: () => string
): SourceStatusView {
  switch (status) {
    case "ok":
      return { label: ok(), tone: "neutral" };
    case "failing":
      return { label: couldNot(error), tone: "destructive", code: error?.code };
    case "disabled":
      return { label: "Scripts are turned off", tone: "warning", code: error?.code };
    case "unapproved":
      // The page's approval notice carries the details; the row only says it waits.
      return { label: "Needs your approval", tone: "warning", code: error?.code };
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

/** The catalog's read of a script: its current sha256 and whether those bytes are approved. */
export interface CheckInScriptRead {
  sha256: string;
  approved: boolean;
  /** A file without the exec bit can't run, so there is nothing to approve yet. */
  executable: boolean;
}

/** A script the operator can approve now, whether or not a check has read it yet. */
export function checkInScriptAwaitsApproval(
  script: CheckInScriptRead | undefined
): script is CheckInScriptRead {
  return Boolean(script && script.executable && !script.approved);
}

/** The declared scripts whose current bytes wait for the operator, in the file's order. */
export function checkInScriptsAwaitingApproval(
  sources: readonly Pick<CheckInSource, "script">[],
  catalog: Pick<AgentHeartbeatSourceCatalog, "scripts" | "scripts_enabled"> | null | undefined
) {
  // With scripts off nothing runs, so approving promises nothing: the row says they're off.
  if (!catalog?.scripts_enabled) return [];
  const names = sources.flatMap(source => (source.script ? [source.script] : []));
  return [...new Set(names)].filter(name =>
    checkInScriptAwaitsApproval(catalog?.scripts.find(item => item.name === name))
  );
}

/**
 * The source's row status; a source the daemon hasn't read yet is "Not checked yet", never OK.
 * `script` is the catalog's read of a script source. Its approval leads the poll state: a new or
 * paused source can't be checked until it's approved, so waiting for a check would never end.
 */
export function checkInSourceStatus(
  source: Pick<CheckInSource, "kind" | "script">,
  state: AgentInstanceSourceStatus | undefined,
  now: number,
  script?: CheckInScriptRead,
  /** The catalog's `scripts_enabled`; `undefined` until the catalog answers. */
  scriptsEnabled?: boolean
): SourceStatusView {
  // Scripts that are turned off say so first; approving them changes nothing until they're on.
  if (source.script && scriptsEnabled === false) {
    return statusView("disabled", source, state?.error, () => "");
  }
  const unread = !state || state.status === "unapproved" || state.status === "pending_first_poll";
  // Approval covers bytes only: a file without its exec bit can't run, approved or not.
  if (unread && script && !script.executable) {
    return { label: "Can't run: the file isn't executable", tone: "warning" };
  }
  if (state?.status !== "disabled" && checkInScriptAwaitsApproval(script)) {
    return statusView("unapproved", source, state?.error, () => "");
  }
  if (!state) return { label: "Not checked yet", tone: "muted" };
  if (unread && script?.approved) {
    return { label: "Approved · runs at the next check-in", tone: "muted" };
  }
  const ok = () => {
    const checked = checkedAgo(state.last_success_at || state.last_polled_at, now);
    return state.last_item_count > 0 ? `${checked} · ${state.last_item_count} new` : checked;
  };
  return statusView(state.status, source, state.error, ok);
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
