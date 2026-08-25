import type { SettingsSkillSource, SettingsSkillSourceRoot } from "../types";

/**
 * Projections over the daemon `sources[]` read model.
 *
 * Every string here restates a field the daemon reported. Nothing is inferred:
 * a root with no reported count renders a word, never a zero, because "we did
 * not measure this" and "there is nothing here" are different facts.
 */

export type SkillSourceRootState = "measured" | "absent" | "unreadable" | "unavailable";

export interface SkillSourceSkipView {
  /** Last path segment — the entry the operator recognizes. */
  name: string;
  path: string;
  sentence: string;
}

export interface SkillSourceCollisionView {
  name: string;
  /** Label of the source that owns the winning root, or the raw id when unresolved. */
  winner: string;
  qualifiedForm: string;
}

export interface SkillSourceDiagnosticsView {
  summary: string;
  skipped: SkillSourceSkipView[];
  collisions: SkillSourceCollisionView[];
  verification: string | null;
}

export interface SkillSourceRootView {
  rootId: string;
  path: string;
  state: SkillSourceRootState;
  /** State sentence for absent/unreadable/truncated roots; null when plainly measured. */
  stateLabel: string | null;
  /** "3 skills" or "5 found · 3 usable"; null when the daemon reported no count. */
  countLabel: string | null;
  truncated: boolean;
  diagnostics: SkillSourceDiagnosticsView | null;
}

export interface SkillSourceView {
  slug: string;
  label: string;
  kind: string;
  enabled: boolean;
  alwaysOn: boolean;
  isCustom: boolean;
  /** Configured path for a custom source; presets list their roots instead. */
  path: string | null;
  /** Sum of reported per-root counts; null when no root reported one. */
  totalLabel: string | null;
  hasUnreadableRoot: boolean;
  hasTruncatedRoot: boolean;
  roots: SkillSourceRootView[];
}

const SKIP_SENTENCES: Record<string, string> = {
  dangling: "its shortcut points to a folder that no longer exists",
  escape: "points outside this source, so it isn't loaded",
  cycle: "its shortcut points back at itself",
};

export function skillCountLabel(count: number): string {
  return count === 1 ? "1 skill" : `${count} skills`;
}

function rootState(root: SettingsSkillSourceRoot): SkillSourceRootState {
  if (!root.exists) return "absent";
  if (!root.readable) return "unreadable";
  return "measured";
}

function rootCountLabel(root: SettingsSkillSourceRoot): string | null {
  if (!root.exists || !root.readable) return null;
  const skills = root.skill_count;
  if (typeof skills !== "number") return null;
  const scanned = root.scanned_count;
  // A truncated root explains its own gap in the state sentence, so the count
  // stays the plain effective number rather than repeating "found vs usable".
  if (!root.truncated && typeof scanned === "number" && scanned !== skills) {
    return `${scanned} found · ${skills} usable`;
  }
  return skillCountLabel(skills);
}

function rootStateLabel(root: SettingsSkillSourceRoot, state: SkillSourceRootState): string | null {
  if (state === "absent") return "no folder yet";
  if (state === "unreadable") return "can't read this folder";
  if (!root.truncated) return null;
  const scanned = root.scanned_count;
  return typeof scanned === "number"
    ? `large folder — first ${scanned} scanned`
    : "large folder — scanned in part";
}

function verificationLabel(root: SettingsSkillSourceRoot): string | null {
  const { blocked, warned } = root.verification;
  if (blocked === 0 && warned === 0) return null;
  const warnPart = warned === 0 ? "no warnings" : warned === 1 ? "1 warning" : `${warned} warnings`;
  const blockPart = blocked === 0 ? "nothing blocked" : `${blocked} blocked`;
  return `${warnPart} · ${blockPart}`;
}

function baseName(path: string): string {
  const trimmed = path.replace(/\/+$/u, "");
  const index = trimmed.lastIndexOf("/");
  return index === -1 ? trimmed : trimmed.slice(index + 1);
}

function diagnosticsSummary(root: SettingsSkillSourceRoot): string {
  const scanned = root.scanned_count;
  const skills = root.skill_count;
  if (typeof scanned === "number" && typeof skills === "number" && scanned !== skills) {
    return `Why only ${skills} of ${scanned}?`;
  }
  return "Scan details";
}

function rootDiagnostics(
  root: SettingsSkillSourceRoot,
  labelByRootId: ReadonlyMap<string, string>
): SkillSourceDiagnosticsView | null {
  const skipped = root.skipped_links.map(link => ({
    name: baseName(link.path),
    path: link.path,
    sentence: SKIP_SENTENCES[link.reason] ?? link.reason,
  }));
  const collisions = root.collisions.map(collision => ({
    name: collision.name,
    winner: labelByRootId.get(collision.winner_root_id) ?? collision.winner_root_id,
    qualifiedForm: collision.qualified_form,
  }));
  const verification = verificationLabel(root);
  if (skipped.length === 0 && collisions.length === 0 && verification === null) return null;
  return { summary: diagnosticsSummary(root), skipped, collisions, verification };
}

/** Maps every reported root id to the label of the source that owns it. */
export function sourceLabelByRootId(
  sources: readonly SettingsSkillSource[]
): ReadonlyMap<string, string> {
  const entries = new Map<string, string>();
  for (const source of sources) {
    for (const root of source.roots) entries.set(root.root_id, source.label);
  }
  return entries;
}

export function toSkillSourceView(
  source: SettingsSkillSource,
  labelByRootId: ReadonlyMap<string, string>,
  measurementsAvailable = true
): SkillSourceView {
  const roots = source.roots.map(root => {
    if (!measurementsAvailable) {
      return {
        rootId: root.root_id,
        path: root.path,
        state: "unavailable" as const,
        stateLabel: null,
        countLabel: null,
        truncated: false,
        diagnostics: null,
      };
    }
    const state = rootState(root);
    return {
      rootId: root.root_id,
      path: root.path,
      state,
      stateLabel: rootStateLabel(root, state),
      countLabel: rootCountLabel(root),
      truncated: state === "measured" && root.truncated,
      diagnostics: state === "measured" ? rootDiagnostics(root, labelByRootId) : null,
    };
  });
  const counted = measurementsAvailable
    ? source.roots.filter(
        root => root.exists && root.readable && typeof root.skill_count === "number"
      )
    : [];
  const total = counted.reduce((sum, root) => sum + (root.skill_count ?? 0), 0);
  return {
    slug: source.slug,
    label: source.label,
    kind: source.kind,
    enabled: source.enabled,
    alwaysOn: source.always_on,
    isCustom: source.kind === "custom",
    path: source.path ?? null,
    totalLabel: counted.length > 0 ? skillCountLabel(total) : null,
    hasUnreadableRoot: roots.some(root => root.state === "unreadable"),
    hasTruncatedRoot: roots.some(root => root.truncated),
    roots,
  };
}

export interface SkillSourceGroups {
  presets: SkillSourceView[];
  custom: SkillSourceView[];
}

/**
 * Presets and custom folders are two independently overridable config keys, so
 * they render as two groups even though the daemon reports one flat list.
 */
export function groupSkillSources(
  sources: readonly SettingsSkillSource[],
  measurementsAvailable = true
): SkillSourceGroups {
  const labelByRootId = sourceLabelByRootId(sources);
  const views = sources.map(source =>
    toSkillSourceView(source, labelByRootId, measurementsAvailable)
  );
  return {
    presets: views.filter(view => !view.isCustom),
    custom: views.filter(view => view.isCustom),
  };
}
