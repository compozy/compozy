import { findPrototypes } from "../node/discovery.ts";
import { findFlows } from "../node/flows.ts";
import { repoRoot, webRoot } from "../node/paths.ts";
import { findSpecs } from "../node/specs.ts";

/** Discovery as the CLI sees it: the spec catalog, prototypes and problems. */
export function discover() {
  const catalog = findSpecs(repoRoot);
  return { catalog, ...findPrototypes(webRoot, repoRoot, catalog) };
}

/** Prototype discovery plus the journey flows resolved against it. */
export function discoverFlows() {
  const result = discover();
  const found = findFlows(webRoot, repoRoot, result.entries, result.catalog);
  return { ...result, flows: found.flows, flowProblems: found.problems, warnings: found.warnings };
}

export function findEntry(slug: string) {
  const entry = discover().entries.find(item => item.slug === slug);
  if (!entry) throw new Error(`Unknown prototype "${slug}" (bun run proto list)`);
  return entry;
}
