import { defineProtoConfig } from "@compozy/proto/config";
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { join, resolve } from "node:path";

const specIdPattern = /^[a-z0-9]+(?:[-_][a-z0-9]+)*$/;

/** The `_spec.md` H1 without a trailing "— spec"; the directory name when there is none. */
function specTitle(directory: string, id: string) {
  let source: string;
  try {
    source = readFileSync(join(directory, "_spec.md"), "utf8");
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "ENOENT") return id;
    throw error;
  }
  const heading = source.split("\n").find(line => line.startsWith("# "));
  const title = heading
    ?.slice(2)
    .replace(/\s+[—–-]\s+spec(?:ification)?$/i, "")
    .trim();
  return title || id;
}

/**
 * Specs a prototype or flow can name: the `.compozy/tasks/<id>/` directories of the repository.
 * That directory is local workflow state Git ignores, so it is absent or empty in CI and fresh
 * worktrees; `null` then tells the gallery to skip spec validation instead of failing every
 * prototype.
 */
function compozySpecs(webRoot: string) {
  const tasks = resolve(webRoot, "..", ".compozy", "tasks");
  if (!existsSync(tasks)) return null;
  const specs = Object.fromEntries(
    readdirSync(tasks, { withFileTypes: true })
      .filter(entry => entry.isDirectory() && specIdPattern.test(entry.name))
      .map(entry => entry.name)
      .sort((a, b) => a.localeCompare(b))
      .map(id => [id, specTitle(join(tasks, id), id)])
  );
  // An empty directory (a worktree that copied no active specs) is as unavailable as a missing one.
  return Object.keys(specs).length > 0 ? specs : null;
}

/** Prototype gallery of the web app: `bun run proto dev` (contract: docs/design/prototyping.md). */
export default defineProtoConfig({
  title: "CompozyOS",
  specs: compozySpecs,
  guards: {
    // Prototypes and drafts render views from fixtures; they never reach the daemon network layer.
    forbidRuntimeImports: [
      "src/lib/api-client.ts",
      "src/lib/ticketed-event-source.ts",
      "src/lib/ticketed-web-socket.ts",
      "src/lib/gateway-*.ts",
      "@compozy/client",
      "@tanstack/react-query",
    ],
  },
});
