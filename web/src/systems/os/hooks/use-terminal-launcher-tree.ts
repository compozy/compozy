import { useState, type KeyboardEvent } from "react";

import {
  terminalLauncherNewValue,
  terminalLauncherProjects,
  terminalLauncherProjectViews,
  terminalLauncherSearching,
  type TerminalLauncherDestination,
  type TerminalLauncherProjectView,
} from "../lib/terminal-launcher-model";

const flip = (set: ReadonlySet<string>, value: string) => {
  const next = new Set(set);
  if (!next.delete(value)) next.add(value);
  return next;
};

/**
 * The terminal launcher's interaction state: the search, the selected row, and
 * which projects show their worktrees. `defaultValue` is the row selected
 * until the operator moves; `readOnly` launchers offer no new-terminal rows.
 */
export function useTerminalLauncherTree({
  destinations,
  defaultValue,
  readOnly,
}: {
  destinations: readonly TerminalLauncherDestination[];
  defaultValue: string | undefined;
  readOnly: boolean;
}) {
  // `null` follows the default, so a default that settles later (the catalog
  // arriving) re-selects; a row the operator moves to, or a search, sticks.
  const [chosen, setChosen] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  // Folds the operator flipped: `unfolded` while browsing, `searchFlips` against
  // a search's own unfolding (cleared as the search changes).
  const [unfolded, setUnfolded] = useState<ReadonlySet<string>>(() => new Set());
  const [searchFlips, setSearchFlips] = useState<ReadonlySet<string>>(() => new Set());
  const searching = terminalLauncherSearching(query);
  const projects = terminalLauncherProjectViews(
    terminalLauncherProjects(destinations),
    query,
    searching ? searchFlips : unfolded
  );
  const selected = chosen ?? defaultValue ?? "";
  const select = (value: string) => setChosen(value === defaultValue ? null : value);
  const toggle = (project: TerminalLauncherProjectView) => {
    const value = project.destination.value;
    if (searching) setSearchFlips(current => flip(current, value));
    else setUnfolded(current => flip(current, value));
  };

  return {
    query,
    projects,
    selected,
    select,
    toggle,
    search: (next: string) => {
      setQuery(next);
      setSearchFlips(new Set());
    },
    // → unfolds the selected project, ← folds it or climbs from a worktree to
    // its project. Only on an empty search field, where the arrows move no caret.
    onTreeKey: (event: KeyboardEvent<HTMLElement>) => {
      if (readOnly || query !== "" || (event.key !== "ArrowRight" && event.key !== "ArrowLeft")) {
        return;
      }
      const opening = event.key === "ArrowRight";
      for (const project of projects) {
        if (terminalLauncherNewValue(project.destination.value) === selected) {
          if (project.worktrees.length === 0 || project.expanded === opening) return;
          event.preventDefault();
          toggle(project);
          return;
        }
        if (
          !opening &&
          project.shown.some(row => terminalLauncherNewValue(row.value) === selected)
        ) {
          event.preventDefault();
          select(terminalLauncherNewValue(project.destination.value));
          return;
        }
      }
    },
  };
}
