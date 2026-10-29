import { existsSync, readdirSync, readFileSync, watch, type FSWatcher } from "node:fs";
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
  let source: string;
  try {
    source = readFileSync(join(directory, "_spec.md"), "utf8");
  } catch (error) {
    // No `_spec.md` yet, or removed while the catalog is read: the id is the title.
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

/**
 * Calls `onChange` when a spec is added, removed or retitled while `proto dev` runs. It watches
 * `.compozy/tasks/`, each spec folder and its `_spec.md` without recursion: task folders hold
 * thousands of files the catalog never reads, and a folder watch alone misses in-place edits on
 * macOS. Bursts are coalesced. Returns the function that stops watching.
 */
export function watchSpecs(repoRoot: string, onChange: () => void) {
  const tasks = join(repoRoot, ".compozy", "tasks");
  if (existsSync(tasks)) return watchTasks(tasks, onChange);
  // Not there yet: watch the nearest existing parent until `.compozy/tasks/` appears.
  let parent: FSWatcher | null = null;
  let stopTasks: (() => void) | null = null;
  const arm = () => {
    parent?.close();
    parent = null;
    if (existsSync(tasks)) {
      stopTasks = watchTasks(tasks, onChange);
      onChange();
      return;
    }
    const compozy = join(repoRoot, ".compozy");
    try {
      parent = watch(existsSync(compozy) ? compozy : repoRoot, arm).on("error", () => undefined);
    } catch {
      // The parent vanished between the check and the watch; nothing to follow.
    }
  };
  arm();
  return () => {
    parent?.close();
    stopTasks?.();
  };
}

function watchTasks(tasks: string, onChange: () => void) {
  type Watched = { folder: FSWatcher; spec: FSWatcher | null };
  const watched = new Map<string, Watched>();
  const ignore = () => undefined;
  let timer: ReturnType<typeof setTimeout> | undefined;
  const changed = () => {
    clearTimeout(timer);
    timer = setTimeout(onChange, 50);
  };
  const tryWatch = (path: string, listener: (event: string, file: string | null) => void) => {
    try {
      return watch(path, listener).on("error", ignore);
    } catch {
      return null; // Removed between the listing and the watch.
    }
  };
  // Atomic saves replace the file, so the spec watcher is re-armed on every folder event.
  const armSpec = (id: string, entry: Watched) => {
    entry.spec?.close();
    entry.spec = tryWatch(join(tasks, id, "_spec.md"), changed);
  };
  const sync = () => {
    let ids: Set<string>;
    try {
      ids = new Set(
        readdirSync(tasks, { withFileTypes: true }).flatMap(entry =>
          entry.isDirectory() && specIdPattern.test(entry.name) ? [entry.name] : []
        )
      );
    } catch {
      ids = new Set();
    }
    for (const [id, entry] of watched) {
      if (ids.has(id)) continue;
      entry.folder.close();
      entry.spec?.close();
      watched.delete(id);
    }
    for (const id of ids) {
      if (watched.has(id)) continue;
      const folder = tryWatch(join(tasks, id), (_event, file) => {
        if (file !== "_spec.md") return;
        const entry = watched.get(id);
        if (entry) armSpec(id, entry);
        changed();
      });
      if (!folder) continue;
      const entry: Watched = { folder, spec: null };
      watched.set(id, entry);
      armSpec(id, entry);
    }
  };
  sync();
  const root = tryWatch(tasks, () => {
    sync();
    changed();
  });
  return () => {
    clearTimeout(timer);
    root?.close();
    for (const entry of watched.values()) {
      entry.folder.close();
      entry.spec?.close();
    }
  };
}
