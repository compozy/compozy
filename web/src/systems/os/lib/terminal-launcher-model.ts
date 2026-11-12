import type { TerminalInfo } from "@/systems/terminal";
import { DESTINATION_COPY, USER_SCOPE_SELECTOR } from "@/systems/workspace";

import { normalizeRankingText, subsequenceMatch } from "./ranking/normalize";

/** A place a new terminal can open: a project, one of its managed worktrees, or User (`@user`). */
export interface TerminalLauncherDestination {
  /** Unique row key: the project id, `@user`, or `<project id>/<worktree id>`. */
  value: string;
  /** The scope the terminal opens in. */
  workspaceId: string;
  /** The managed worktree the terminal starts in; `null` for the project root or User. */
  worktreeId: string | null;
  label: string;
  /** Why a terminal cannot start here (the project folder is gone); `null` when it can. */
  unavailableReason: string | null;
}

/** The slice of a worktree row the launcher reads. */
export interface TerminalLauncherWorktree {
  id: string;
  workspace_id: string;
  name: string;
  branch: string;
  state: string;
  profile_name: string;
}

/** A launcher row's cmdk value for a new-terminal destination. */
export const terminalLauncherNewValue = (destination: string) => `new:${destination}`;

/** A launcher row's cmdk value for a running terminal. */
export const terminalLauncherAttachValue = (terminalId: string) => `attach:${terminalId}`;

/** Shared with the projects overview: a registered project whose folder no longer exists. */
export const TERMINAL_LAUNCHER_FOLDER_MISSING = "Folder not found";

/**
 * Every project followed by its ready worktrees owned by `profile`, then User,
 * in the catalog's order. A terminal binds only to a worktree of its own
 * profile, and a pending, failed, or removed one has no checkout to start in.
 */
export function terminalLauncherDestinations(
  workspaces: readonly { id: string; name: string; root_missing?: boolean }[],
  worktrees: readonly TerminalLauncherWorktree[] = [],
  profile: string | null = null
): TerminalLauncherDestination[] {
  const destinations: TerminalLauncherDestination[] = [];
  for (const workspace of workspaces) {
    const missing = workspace.root_missing === true;
    destinations.push({
      value: workspace.id,
      workspaceId: workspace.id,
      worktreeId: null,
      label: workspace.name,
      unavailableReason: missing ? TERMINAL_LAUNCHER_FOLDER_MISSING : null,
    });
    if (missing) continue;
    for (const worktree of worktrees) {
      if (worktree.workspace_id !== workspace.id || worktree.state !== "ready") continue;
      if (profile !== null && worktree.profile_name !== profile) continue;
      destinations.push({
        value: `${workspace.id}/${worktree.id}`,
        workspaceId: workspace.id,
        worktreeId: worktree.id,
        label: worktree.name,
        unavailableReason: null,
      });
    }
  }
  destinations.push({
    value: USER_SCOPE_SELECTOR,
    workspaceId: USER_SCOPE_SELECTOR,
    worktreeId: null,
    label: DESTINATION_COPY.user,
    unavailableReason: null,
  });
  return destinations;
}

/** The remembered project when a terminal can start there, else the first that can (User always can). */
export function defaultTerminalLauncherDestination(
  destinations: readonly TerminalLauncherDestination[],
  rememberedWorkspaceId: string | null | undefined
): string {
  const available = destinations.filter(destination => destination.unavailableReason === null);
  const remembered = available.find(destination => destination.value === rememberedWorkspaceId);
  return (remembered ?? available[0])?.value ?? USER_SCOPE_SELECTOR;
}

/** The create body for a destination: a worktree row names its worktree. */
export function terminalLauncherCreateInput(destination: TerminalLauncherDestination) {
  return destination.worktreeId === null ? {} : { worktree: destination.worktreeId };
}

/** Running terminals only: an exited one is history, read from the journal. */
export function runningTerminals(terminals: readonly TerminalInfo[]): TerminalInfo[] {
  return terminals.filter(terminal => terminal.state === "running");
}

/** A project (or User) row and the worktrees that fold under it. */
export interface TerminalLauncherProject {
  destination: TerminalLauncherDestination;
  /** Every worktree destination of the project, in catalog order. */
  worktrees: readonly TerminalLauncherDestination[];
}

/** One project row as the launcher shows it for the current search and folds. */
export interface TerminalLauncherProjectView extends TerminalLauncherProject {
  /**
   * Whether the project row is a result itself. A project shown only as the
   * parent of matching worktrees is context: its row cannot be chosen, so a
   * search selects the worktree that was searched for.
   */
  matches: boolean;
  /** Whether the worktrees show; always `false` for a project without any. */
  expanded: boolean;
  /** The worktree rows on the project's branch: the matching ones while searching. */
  shown: readonly TerminalLauncherDestination[];
}

/** Folds the flat destination list into project rows, each owning the worktrees that follow it. */
export function terminalLauncherProjects(
  destinations: readonly TerminalLauncherDestination[]
): TerminalLauncherProject[] {
  const projects: {
    destination: TerminalLauncherDestination;
    worktrees: TerminalLauncherDestination[];
  }[] = [];
  for (const destination of destinations) {
    const parent = projects.at(-1);
    if (
      destination.worktreeId !== null &&
      parent?.destination.workspaceId === destination.workspaceId
    ) {
      parent.worktrees.push(destination);
    } else {
      projects.push({ destination, worktrees: [] });
    }
  }
  return projects;
}

/** Whether `query` filters the launcher: anything but separators and blanks. */
export function terminalLauncherSearching(query: string): boolean {
  return normalizeRankingText(query).compact !== "";
}

const termReaches = (term: string, field: string) =>
  subsequenceMatch(term, normalizeRankingText(field).compact) !== null;

/**
 * Whether every word of `query` reaches one of `fields`: its letters appear in
 * order, ignoring case, accents, and separators (the palette's subsequence
 * match), so a search can combine words from several fields. An empty query
 * matches everything.
 */
export function terminalLauncherMatches(query: string, fields: readonly string[]): boolean {
  return normalizeRankingText(query).tokens.every(term =>
    fields.some(field => termReaches(term, field))
  );
}

/**
 * Whether a search reaches a worktree: every word reaches its name or its
 * project's, and at least one reaches its own name, so the project's name
 * alone never lists every worktree of the project.
 */
function worktreeMatches(query: string, worktree: string, project: string): boolean {
  return (
    terminalLauncherMatches(query, [worktree, project]) &&
    normalizeRankingText(query).tokens.some(term => termReaches(term, worktree))
  );
}

/**
 * The project rows for a search. Without a query every project shows and its
 * worktrees follow `toggled` (the projects the operator unfolded). With one, a
 * project shows when it or one of its worktrees matches; a project holding a
 * match unfolds on its own, and `toggled` then flips that default, so a match
 * can be folded and a project matched by name unfolded. An unfolded project
 * shows only its matching worktrees, or all of them when none match.
 */
export function terminalLauncherProjectViews(
  projects: readonly TerminalLauncherProject[],
  query: string,
  toggled: ReadonlySet<string>
): TerminalLauncherProjectView[] {
  const searching = terminalLauncherSearching(query);
  const views: TerminalLauncherProjectView[] = [];
  for (const project of projects) {
    const label = project.destination.label;
    const matching = searching
      ? project.worktrees.filter(worktree => worktreeMatches(query, worktree.label, label))
      : [];
    const matches = terminalLauncherMatches(query, [label]);
    if (searching && matching.length === 0 && !matches) continue;
    const flipped = toggled.has(project.destination.value);
    const expanded =
      project.worktrees.length > 0 && (searching ? matching.length > 0 !== flipped : flipped);
    const shown = !expanded ? [] : matching.length > 0 ? matching : project.worktrees;
    views.push({ ...project, matches, expanded, shown });
  }
  return views;
}
