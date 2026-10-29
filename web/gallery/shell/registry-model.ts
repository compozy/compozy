/**
 * Pure model behind the gallery navigation: prototypes grouped by spec, in catalog order, then
 * the prototypes without a spec, with the journey flows each spec owns or is crossed by, flow
 * membership of every prototype state and the coverage warnings. `registry.ts` feeds it from
 * `virtual:compozy-prototypes`; tests feed it fixtures.
 */
import type { ComponentType } from "react";

import type { PrototypeMeta, PrototypeProps } from "#gallery/contract";
import type { CoverageWarning, FlowEntry, FlowProblem, Occurrence } from "#gallery/flow";
import { flowCount, membership } from "#gallery/flow-graph";
import type { PrototypeKind } from "#gallery/node/discovery";
import type { SpecInfo } from "#gallery/node/specs";

export type PrototypeModule = { meta: PrototypeMeta; default: ComponentType<PrototypeProps> };

export type Entry = { slug: string; kind: PrototypeKind; file: string; meta: PrototypeMeta };

/** Group id of prototypes without a known spec. */
export const standaloneGroup = "standalone";

export type Group = {
  id: string;
  title: string;
  entries: Entry[];
  /** Flows this spec owns (`spec` field); for "No spec", the flows without an owner. */
  flows: FlowEntry[];
  /** Flows owned elsewhere with at least one prototype node of this spec. */
  crossing: FlowEntry[];
};

export type Registry = {
  groups: Group[];
  /** Every listed prototype, in navigation order (group, then title). */
  entries: Entry[];
  /** Discovery problems (prototypes and flows), shown in the sidebar until fixed. */
  broken: string[];
  /** Every valid flow, by owner spec then title ("Journeys"). */
  flows: FlowEntry[];
  warnings: CoverageWarning[];
  /** `${slug}:${state}` → every flow node using that state, in flow then step order. */
  membership: Map<string, Occurrence[]>;
};

type RegistryInput = {
  entries: Entry[];
  specs: readonly SpecInfo[];
  broken: string[];
  flows?: readonly FlowEntry[];
  flowProblems?: readonly FlowProblem[];
  warnings?: readonly CoverageWarning[];
};

function byTitle(a: Entry, b: Entry) {
  return a.meta.title.localeCompare(b.meta.title, "en", { numeric: true });
}

/**
 * Spec groups in catalog order, then "No spec". A spec is listed when it has prototypes or owns
 * flows; flows whose owner is unknown (or absent) belong to "No spec".
 */
export function buildRegistry(input: RegistryInput): Registry {
  const entries = [...input.entries].sort(byTitle);
  const flows = [...(input.flows ?? [])];
  const known = new Set(input.specs.map(spec => spec.id));
  const groups: Group[] = input.specs.map(spec => ({
    id: spec.id,
    title: spec.title,
    entries: entries.filter(entry => entry.meta.spec === spec.id),
    flows: flows.filter(flow => flow.spec === spec.id),
    crossing: flows.filter(flow => flow.spec !== spec.id && flow.specs.includes(spec.id)),
  }));
  groups.push({
    id: standaloneGroup,
    title: "No spec",
    entries: entries.filter(entry => !entry.meta.spec || !known.has(entry.meta.spec)),
    flows: flows.filter(flow => !flow.spec || !known.has(flow.spec)),
    crossing: [],
  });
  const listed = groups.filter(group => group.entries.length > 0 || group.flows.length > 0);
  return {
    groups: listed,
    entries: listed.flatMap(group => group.entries),
    broken: [...input.broken, ...(input.flowProblems ?? []).map(problem => problem.message)],
    flows,
    warnings: [...(input.warnings ?? [])],
    membership: membership(flows),
  };
}

export function groupOf(registry: Registry, id: string | null | undefined) {
  return registry.groups.find(group => group.id === id);
}

/** The group a prototype is listed under. */
export function groupOfEntry(registry: Registry, entry: Entry) {
  return registry.groups.find(group => group.entries.includes(entry));
}

/** Position of a spec in the gallery order (printed `01`, `02`…); "No spec" has none. */
export function specPosition(registry: Registry, id: string) {
  return id === standaloneGroup ? null : registry.groups.findIndex(group => group.id === id);
}

/** The spec id as printed ("no spec" for the standalone group). */
export function specSlug(group: Group) {
  return group.id === standaloneGroup ? "no spec" : group.id;
}

/** Position in an ordered list, as the gallery prints it (`01`, `02`…). */
export function ordinal(index: number) {
  return String(index + 1).padStart(2, "0");
}

export function plural(count: number, word: string) {
  return `${count} ${count === 1 ? word : `${word}s`}`;
}

/** A spec's number and id, `03 · sessions-kanban`. */
export function specCode(registry: Registry, group: Group) {
  const position = specPosition(registry, group.id);
  return position === null ? specSlug(group) : `${ordinal(position)} · ${specSlug(group)}`;
}

export function matches(entry: Entry, query: string) {
  const needle = query.trim().toLowerCase();
  if (!needle) return true;
  const { meta } = entry;
  return [meta.title, entry.slug, meta.route ?? "", meta.surface ?? "", meta.spec ?? ""].some(
    value => value.toLowerCase().includes(needle)
  );
}

/** Every state whose label matches the query, with its prototype. */
export function stateMatches(entries: readonly Entry[], query: string) {
  const needle = query.trim().toLowerCase();
  if (!needle) return [];
  return entries.flatMap(entry =>
    entry.meta.states.flatMap(option =>
      option.label.toLowerCase().includes(needle)
        ? [{ entry, id: option.id, label: option.label }]
        : []
    )
  );
}

export function flowMatches(flow: FlowEntry, query: string) {
  const needle = query.trim().toLowerCase();
  if (!needle) return true;
  return [flow.title, flow.id, flow.spec ?? ""].some(value => value.toLowerCase().includes(needle));
}

export function flowById(registry: Registry, id: string | null | undefined) {
  return registry.flows.find(flow => flow.id === id);
}

/** The group a flow is listed under: its owner spec, or "No spec". */
export function groupOfFlow(registry: Registry, flow: FlowEntry) {
  return registry.groups.find(group => group.flows.includes(flow));
}

/** Uncovered states in the specs this flow owns or crosses (the canvas warning count). */
export function flowWarnings(registry: Registry, flow: FlowEntry) {
  const specs = new Set(flow.specs);
  return registry.warnings.filter(warning => specs.has(warning.spec));
}

export function occurrencesOf(registry: Registry, slug: string, state: string) {
  return registry.membership.get(`${slug}:${state}`) ?? [];
}

/** Distinct flows that show this prototype state. */
export function stateFlowCount(registry: Registry, slug: string, state: string) {
  return flowCount(occurrencesOf(registry, slug, state));
}
