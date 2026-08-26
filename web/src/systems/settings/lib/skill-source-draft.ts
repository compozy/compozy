import type { SettingsSkillSource } from "../types";

/**
 * Pre-flight for the "add your own folder" field.
 *
 * These checks mirror the daemon's rules so the operator hears about a bad entry
 * next to the input instead of after a round trip. The daemon stays the
 * authority: a save it rejects still renders its own verbatim error.
 */

export interface SkillSourceEntryError {
  /** Plain sentence first. */
  message: string;
  /** The daemon code this mirrors, kept verbatim for support and logs. */
  code: string;
}

const WORKSPACE_SCOPE_HINT =
  "Folders inside a project can only be added in Workspace scope — switch the scope above.";

function isWorkspaceRelative(path: string): boolean {
  return !path.startsWith("/") && !path.startsWith("~");
}

function normalizePath(path: string): string {
  const trimmed = path.trim();
  const withoutTrailingSlash = trimmed.replace(/\/+$/u, "");
  return withoutTrailingSlash === "" ? trimmed : withoutTrailingSlash;
}

function baseName(path: string): string {
  const index = path.lastIndexOf("/");
  return index === -1 ? path : path.slice(index + 1);
}

/**
 * Finds the source that already owns this path. A measured source answers with
 * its daemon label; an entry added but not yet saved has no label yet, so its
 * basename stands in — the same shape the daemon will derive for it.
 */
function owningSource(
  entry: string,
  customEntries: readonly string[],
  sources: readonly SettingsSkillSource[]
): string | null {
  const normalized = normalizePath(entry);
  for (const source of sources) {
    const configuredPaths = [source.path, source.global_path, source.workspace_path];
    if (
      configuredPaths.some(
        candidate => typeof candidate === "string" && normalizePath(candidate) === normalized
      )
    ) {
      return source.label;
    }
    for (const root of source.roots) {
      if (normalizePath(root.path) === normalized) return source.label;
    }
  }
  for (const existing of customEntries) {
    if (normalizePath(existing) === normalized) return baseName(normalized);
  }
  return null;
}

export function validateSkillSourceEntry(
  entry: string,
  options: {
    customEntries: readonly string[];
    sources: readonly SettingsSkillSource[];
    workspaceScope: boolean;
  }
): SkillSourceEntryError | null {
  const trimmed = entry.trim();
  if (trimmed === "") return null;
  if (!options.workspaceScope && isWorkspaceRelative(trimmed)) {
    return { message: WORKSPACE_SCOPE_HINT, code: "invalid_source_path" };
  }
  const owner = owningSource(trimmed, options.customEntries, options.sources);
  if (owner !== null) {
    return {
      message: `This folder is already on the list as ${owner}.`,
      code: "duplicate_skill_source",
    };
  }
  return null;
}

export function addSkillSourceEntry(entries: readonly string[], entry: string): string[] {
  return [...entries, entry.trim()];
}

export function removeSkillSourceEntry(entries: readonly string[], entry: string): string[] {
  const normalized = normalizePath(entry);
  return entries.filter(candidate => normalizePath(candidate) !== normalized);
}

export function toggleSkillSourcePreset(
  slugs: readonly string[],
  slug: string,
  enabled: boolean
): string[] {
  if (enabled) return slugs.includes(slug) ? [...slugs] : [...slugs, slug];
  return slugs.filter(candidate => candidate !== slug);
}
