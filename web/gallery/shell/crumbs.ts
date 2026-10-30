/**
 * The levels of what is on screen, widest first (specs index → spec → prototype → state), for
 * the breadcrumb and the "copy link" menu.
 */
import type { FlowEntry } from "#gallery/flow";
import { stepLabel } from "#gallery/shell/flows/node-model";
import { plural, specSlug, type Entry, type Group } from "#gallery/shell/registry-model";
import type { Selection } from "#gallery/shell/view";

export type Crumb = { label: string; detail?: string; target: Selection };

export const indexCrumb: Crumb = { label: "Specs", target: { kind: "index" } };

export function specCrumb(group: Group): Crumb {
  return { label: group.title, detail: specSlug(group), target: { kind: "spec", spec: group.id } };
}

export function prototypeCrumbs(group: Group | undefined, entry: Entry, state: string): Crumb[] {
  const { meta } = entry;
  const index = meta.states.findIndex(option => option.id === state);
  const option = meta.states[index];
  const surface = meta.surface ? `${meta.surface} · ` : "";
  return [
    indexCrumb,
    ...(group ? [specCrumb(group)] : []),
    {
      label: meta.title,
      detail: `prototype · ${surface}/${entry.slug}`,
      target: { kind: "prototype", slug: entry.slug },
    },
    ...(option
      ? [
          {
            label: option.label,
            detail: `state ${index + 1} of ${meta.states.length} · ${option.id}`,
            target: { kind: "prototype", slug: entry.slug, state: option.id },
          } satisfies Crumb,
        ]
      : []),
  ];
}

export function flowCrumbs(
  group: Group | undefined,
  flow: FlowEntry,
  node: string | null
): Crumb[] {
  const step = flow.nodes.find(item => item.id === node);
  return [
    indexCrumb,
    ...(group ? [specCrumb(group)] : []),
    {
      label: flow.title,
      detail: `flow · ${flow.id} · ${plural(flow.nodes.length, "step")}`,
      target: { kind: "flow", flow: flow.id },
    },
    ...(step
      ? [
          {
            label: stepLabel(step),
            detail: `step ${step.step} of ${flow.nodes.length} · ${step.id}`,
            target: { kind: "flow", flow: flow.id, node: step.id },
          } satisfies Crumb,
        ]
      : []),
  ];
}

/** Every level that has its own view, deepest first: what "copy link" offers. */
export function copyTargets(crumbs: readonly Crumb[]) {
  return crumbs.filter(crumb => crumb.target.kind !== "index").reverse();
}
