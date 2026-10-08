export type LoopBindingKind = "schedule" | "webhook" | "trigger";

/** Plain kind words (board VC-04, COPY.md Automations aliases): schedule · event · link. */
const BINDING_KIND_LABEL: Record<LoopBindingKind, string> = {
  schedule: "schedule",
  webhook: "link",
  trigger: "event",
};

/**
 * One attached loop-target automation row for the detail Start-bindings panel.
 * `meta` is the kind-specific mono line (schedule: cron + next fire; webhook:
 * endpoint slug; trigger: event name), pre-formatted by the route layer so this
 * module carries no automation-system dependency.
 */
export interface LoopBindingRow {
  id: string;
  name: string;
  kind: LoopBindingKind;
  enabled: boolean;
  meta: string;
}

export function bindingKindLabel(kind: LoopBindingKind): string {
  return BINDING_KIND_LABEL[kind];
}

/**
 * The distinct binding kinds attached to one loop, sorted for a stable catalog
 * badge (`schedule` before `trigger` before `webhook`).
 */
export function summarizeBindingKinds(rows: readonly LoopBindingRow[]): LoopBindingKind[] {
  const seen = new Set<LoopBindingKind>();
  for (const row of rows) seen.add(row.kind);
  return [...seen].sort((a, b) => a.localeCompare(b));
}

interface LoopBindingPageCount {
  hasMore: boolean;
  loaded: number;
  total: number;
}

export interface LoopBindingCounts {
  /** True when at least one paginated source (jobs/triggers) was supplied. */
  paginated: boolean;
  hasMore: boolean;
  loaded: number;
  total: number;
}

/**
 * Loaded/total automation counts for the Start-bindings panel. Paginated
 * sources (schedules + triggers) own the totals when present; otherwise the
 * rendered rows are the whole set.
 */
export function countLoopBindings(
  rowCount: number,
  jobs?: LoopBindingPageCount,
  triggers?: LoopBindingPageCount
): LoopBindingCounts {
  if (!jobs && !triggers) {
    return { paginated: false, hasMore: false, loaded: rowCount, total: rowCount };
  }
  return {
    paginated: true,
    hasMore: Boolean(jobs?.hasMore || triggers?.hasMore),
    loaded: (jobs?.loaded ?? 0) + (triggers?.loaded ?? 0),
    total: (jobs?.total ?? 0) + (triggers?.total ?? 0),
  };
}

/** Rail-section gist: "Manual only" when nothing is attached, else the count. */
export function bindingsGist(total: number): string {
  if (total === 0) return "Manual only";
  return `${total} ${total === 1 ? "automation" : "automations"}`;
}

/** How an automation can start this Loop; the editor's Starts preselection. */
export type LoopAutomateStart = "schedule" | "event";

const EVENT_START_KINDS = new Set(["trigger", "webhook"]);

/** The Automate menu items the Loop's `start[]` allowlist permits, schedule first. */
export function loopAutomateStarts(declaredKinds: readonly string[]): LoopAutomateStart[] {
  const starts: LoopAutomateStart[] = [];
  if (declaredKinds.includes("schedule")) starts.push("schedule");
  if (declaredKinds.some(kind => EVENT_START_KINDS.has(kind))) starts.push("event");
  return starts;
}
