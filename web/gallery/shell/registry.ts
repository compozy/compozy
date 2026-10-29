import {
  flowProblems,
  flows,
  problems,
  prototypes,
  specs,
  warnings,
} from "virtual:compozy-prototypes";

import { buildRegistry, type Entry } from "#gallery/shell/registry-model";

export function kindFor(slug: string) {
  return prototypes.find(entry => entry.slug === slug)?.kind;
}

export function loaderFor(slug: string) {
  return prototypes.find(entry => entry.slug === slug)?.load;
}

/** The gallery registry; the virtual module is static per page load (changes reload the page). */
export const registry = buildRegistry({
  entries: prototypes.flatMap((entry): Entry[] =>
    entry.meta ? [{ slug: entry.slug, kind: entry.kind, file: entry.file, meta: entry.meta }] : []
  ),
  specs,
  broken: problems.map(problem => problem.message),
  flows,
  flowProblems,
  warnings,
});
