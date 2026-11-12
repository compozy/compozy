import { useSelector, useStore } from "@xstate/store-react";
import type { KeyboardEvent } from "react";

import {
  terminalLauncherNewValue,
  terminalLauncherProjects,
  terminalLauncherProjectViews,
  terminalLauncherSearching,
  type TerminalLauncherDestination,
  type TerminalLauncherProjectView,
} from "../lib/terminal-launcher-model";
import { terminalLauncherLogic } from "../stores/terminal-launcher-store";

/**
 * The terminal launcher's rows for its interaction state: the search, the
 * selected row, and which projects show their worktrees. `defaultValue` is the
 * row selected until the operator moves; `readOnly` launchers offer no
 * new-terminal rows.
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
  const store = useStore(terminalLauncherLogic);
  const { chosen, query, unfolded, searchFlips } = useSelector(store, state => state.context);
  const projects = terminalLauncherProjectViews(
    terminalLauncherProjects(destinations),
    query,
    terminalLauncherSearching(query) ? searchFlips : unfolded
  );
  const selected = chosen ?? defaultValue ?? "";
  const select = (value: string) => store.trigger.rowSelected({ value, defaultValue });
  const toggle = (project: TerminalLauncherProjectView) =>
    store.trigger.projectFolded({ project: project.destination.value });

  return {
    query,
    projects,
    selected,
    select,
    toggle,
    search: (next: string) => store.trigger.searched({ query: next }),
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
