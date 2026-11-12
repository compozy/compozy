import { createStoreLogic } from "@xstate/store";

import { terminalLauncherSearching } from "../lib/terminal-launcher-model";

interface TerminalLauncherContext {
  /** The row the operator moved to; `null` follows the launcher's default row. */
  chosen: string | null;
  query: string;
  /** Projects the operator unfolded while browsing. */
  unfolded: ReadonlySet<string>;
  /** Projects whose search unfolding the operator flipped; cleared as the search changes. */
  searchFlips: ReadonlySet<string>;
}

type TerminalLauncherEvents = {
  /** A row became selected; the default row keeps following a default that settles later. */
  rowSelected: { value: string; defaultValue: string | undefined };
  searched: { query: string };
  /** The project's fold flipped, against browsing or the current search. */
  projectFolded: { project: string };
};

const flip = (set: ReadonlySet<string>, value: string) => {
  const next = new Set(set);
  if (!next.delete(value)) next.add(value);
  return next;
};

/** The terminal launcher's interaction state: search, selected row, and project folds. */
export const terminalLauncherLogic = createStoreLogic<
  TerminalLauncherContext,
  TerminalLauncherEvents
>({
  context: { chosen: null, query: "", unfolded: new Set(), searchFlips: new Set() },
  on: {
    rowSelected: (context, event) => ({
      ...context,
      chosen: event.value === event.defaultValue ? null : event.value,
    }),
    searched: (context, event) => ({ ...context, query: event.query, searchFlips: new Set() }),
    projectFolded: (context, event) =>
      terminalLauncherSearching(context.query)
        ? { ...context, searchFlips: flip(context.searchFlips, event.project) }
        : { ...context, unfolded: flip(context.unfolded, event.project) },
  },
});
