/**
 * What the gallery shows, derived from the URL: the specs index, a spec overview, a prototype
 * (with its state) or a journey flow (with its node), plus the sidebar level that follows it
 * (index → specs, spec or flow → the spec's prototypes and flows, prototype → its states). Keeps the URL canonical, so a copied link always reopens
 * what is on screen.
 */
import { useEffect, useEffectEvent } from "react";

import { setParams } from "#gallery/contract";
import {
  flowById,
  groupOf,
  groupOfEntry,
  groupOfFlow,
  type Entry,
  type Registry,
} from "#gallery/shell/registry-model";
import {
  canonicalParams,
  readView,
  resolveSelection,
  type ResolvedSelection,
} from "#gallery/shell/view";
import { useSearch } from "#gallery/use-search";

export type NavLevel = "specs" | "prototypes" | "states";

export const navLevels: readonly NavLevel[] = ["specs", "prototypes", "states"];

/** The item after (or before) `current`, wrapping around; the first one when nothing is current. */
export function step<T>(items: readonly T[], current: T | undefined, direction: 1 | -1) {
  if (items.length === 0) return undefined;
  const index = current === undefined ? -1 : items.indexOf(current);
  return items[(index + direction + items.length) % items.length];
}

/** Sidebar level of each selection: the drill-down mirrors the URL. */
export function levelOf(kind: ResolvedSelection["kind"]): NavLevel {
  if (kind === "prototype") return "states";
  return kind === "spec" || kind === "flow" ? "prototypes" : "specs";
}

export function useGallerySelection(registry: Registry) {
  const search = useSearch();
  const view = readView(search);
  const resolved = resolveSelection(view, {
    prototypeExists: slug => registry.entries.some(entry => entry.slug === slug),
    specExists: spec => groupOf(registry, spec) !== undefined,
    flowNodes: id => flowById(registry, id)?.nodes.map(node => node.id) ?? null,
  });
  const flow = resolved.kind === "flow" ? (flowById(registry, resolved.flow) ?? null) : null;
  const current: Entry | null =
    resolved.kind === "prototype"
      ? (registry.entries.find(entry => entry.slug === resolved.slug) ?? null)
      : null;
  const group =
    resolved.kind === "spec"
      ? groupOf(registry, resolved.spec)
      : current
        ? groupOfEntry(registry, current)
        : flow
          ? groupOfFlow(registry, flow)
          : undefined;
  const states = current?.meta.states ?? [];
  const state = states.find(option => option.id === view.state)?.id ?? states[0]?.id ?? "";

  // Make the URL name what is on screen, so a copied link always reopens it.
  const syncUrl = useEffectEvent(() => {
    const patch = canonicalParams(search, resolved, current ? state : null);
    if (patch) setParams(patch);
  });
  useEffect(() => syncUrl(), [search]);

  const node = resolved.kind === "flow" ? resolved.node : null;
  return {
    search,
    view,
    resolved,
    current,
    flow,
    node,
    group,
    states,
    state,
    level: levelOf(resolved.kind),
  };
}

export type GallerySelection = ReturnType<typeof useGallerySelection>;
