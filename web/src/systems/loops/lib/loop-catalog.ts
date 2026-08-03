import type { LoopCatalogEntry } from "../types";

/** Catalog kind grouping — read-only packaged/user sources vs workspace-authored copies. */
export type LoopKind = "read-only" | "workspace";

/** Kind filter values, including the "all" pass-through. */
export type LoopKindFilter = "all" | LoopKind;

/** The unbounded iteration-cap glyph (watch loops declare `iteration_cap: 0`). */
export const UNBOUNDED_CAP = "∞";

/** Only workspace-sourced loops are writable; every other source must be forked first. */
export function loopKind(entry: Pick<LoopCatalogEntry, "source">): LoopKind {
  return entry.source === "workspace" ? "workspace" : "read-only";
}

/**
 * Operator-facing name for a loop's provenance. `workspace` copies are the ones an
 * operator authored here (Custom); every other source ships with the runtime or the
 * operator's home and stays read-only until forked (Built-in).
 */
export function loopSourceLabel(entry: Pick<LoopCatalogEntry, "source">): string {
  return GROUP_LABEL[loopKind(entry)];
}

/**
 * The category a catalog row groups under. The daemon exposes a single free-text
 * `catalog.category`; the filter pills are derived from the values actually present
 * (truthful UI — no invented Engineering/Operations taxonomy the daemon never sends).
 */
export function loopCategory(entry: LoopCatalogEntry): string | null {
  const category = entry.catalog?.category?.trim();
  return category ? category : null;
}

/** Declared-input count for the row meta line. */
export function loopInputCount(entry: LoopCatalogEntry): number {
  return entry.inputs ? Object.keys(entry.inputs).length : 0;
}

/** True when the loop runs unbounded (watch loops). */
export function isUnboundedCap(entry: LoopCatalogEntry): boolean {
  return entry.contract.iteration_cap === 0;
}

/** Iteration-cap label, rendering `0` as the unbounded glyph. */
export function iterationCapLabel(iterationCap: number): string {
  return iterationCap === 0 ? UNBOUNDED_CAP : String(iterationCap);
}

/** A loop declares a human gate when any verification criterion is a human check. */
export function hasHumanGate(entry: LoopCatalogEntry): boolean {
  return Boolean(entry.contract.verification?.some(criterion => criterion.type === "human"));
}

/** Formats a `0..1` success rate as a whole-percent label (`92%`). */
export function successRateLabel(rate: number): string {
  if (!Number.isFinite(rate)) return "—";
  return `${Math.round(rate * 100)}%`;
}

export type LoopStatusFilter = NonNullable<LoopCatalogEntry["last_run"]>["status"];

export interface LoopCatalogFilter {
  kind: LoopKindFilter;
  category: string | null;
  status: LoopStatusFilter | null;
}

/** Applies kind + category + last-run status filters to one row. */
export function matchesLoopFilter(entry: LoopCatalogEntry, filter: LoopCatalogFilter): boolean {
  if (filter.kind !== "all" && loopKind(entry) !== filter.kind) return false;
  if (filter.category && loopCategory(entry) !== filter.category) return false;
  if (filter.status && entry.last_run?.status !== filter.status) return false;
  return true;
}

export function hasActiveLoopFilters(query: string, filter: LoopCatalogFilter): boolean {
  return (
    query.trim() !== "" ||
    filter.kind !== "all" ||
    filter.category !== null ||
    filter.status !== null
  );
}

export interface LoopCatalogGroup {
  kind: LoopKind;
  label: string;
  entries: LoopCatalogEntry[];
}

const GROUP_ORDER: readonly LoopKind[] = ["read-only", "workspace"];
const GROUP_LABEL: Record<LoopKind, string> = {
  "read-only": "Built-in",
  workspace: "Custom",
};

/**
 * Splits the filtered catalog into built-in and custom groups, dropping empty
 * groups so a catalog with no copies never renders an empty custom section.
 * Row order within a group is preserved from the caller's sort.
 */
export function groupLoopCatalog(
  entries: readonly LoopCatalogEntry[],
  filter: LoopCatalogFilter
): LoopCatalogGroup[] {
  const groups: LoopCatalogGroup[] = GROUP_ORDER.map(kind => ({
    kind,
    label: GROUP_LABEL[kind],
    entries: [],
  }));
  const byKind = new Map(groups.map(group => [group.kind, group]));
  for (const entry of entries) {
    if (!matchesLoopFilter(entry, filter)) continue;
    byKind.get(loopKind(entry))?.entries.push(entry);
  }
  return groups.filter(group => group.entries.length > 0);
}
