import { existsSync, readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";

export type SpecInfo = { id: string; title: string };

/**
 * Specs a prototype can belong to, derived from `.compozy/tasks/<id>/` (the title is the
 * `_spec.md` H1 when present). The directory is local workflow state that Git ignores, so it is
 * absent in CI and fresh worktrees: `available: false` tells discovery to skip spec validation
 * there instead of failing every prototype that names a spec.
 */
export type SpecCatalog = { available: boolean; specs: SpecInfo[] };

const specIdPattern = /^[a-z0-9]+(?:[-_][a-z0-9]+)*$/;

function specTitle(directory: string, id: string) {
  const file = join(directory, "_spec.md");
  if (!existsSync(file)) return id;
  const heading = readFileSync(file, "utf8")
    .split("\n")
    .find(line => line.startsWith("# "));
  const title = heading
    ?.slice(2)
    .replace(/\s+[—–-]\s+spec(?:ification)?$/i, "")
    .trim();
  return title || id;
}

export function findSpecs(repoRoot: string): SpecCatalog {
  const tasks = join(repoRoot, ".compozy", "tasks");
  if (!existsSync(tasks)) return { available: false, specs: [] };
  const specs = readdirSync(tasks, { withFileTypes: true })
    .flatMap(entry =>
      entry.isDirectory() && specIdPattern.test(entry.name)
        ? [{ id: entry.name, title: specTitle(join(tasks, entry.name), entry.name) }]
        : []
    )
    .sort((a, b) => a.id.localeCompare(b.id));
  return { available: true, specs };
}
