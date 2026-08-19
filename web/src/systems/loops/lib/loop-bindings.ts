export type LoopBindingKind = "schedule" | "webhook" | "trigger";

const BINDING_KIND_LABEL: Record<LoopBindingKind, string> = {
  schedule: "schedule",
  webhook: "webhook",
  trigger: "trigger",
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
