import { useSelector } from "@xstate/store-react";
import { useRef, useState, type RefObject } from "react";

import { resolveCommandSelection } from "@compozy/ui";

import { resolvePaletteRowSubject } from "../lib/cmd-palette-row-actions";
import { cmdPaletteExecutionStore } from "../stores/cmd-palette-execution-store";
import type { CmdPaletteDispatch } from "./use-cmd-palette-dispatch";
import { useOsPaletteExecution, type OsPaletteExecutionModel } from "./use-os-palette-execution";
import { useOsPaletteRoot, type OsPaletteRootModel } from "./use-os-palette-root";
import { useOsPaletteViewStack, type OsPaletteViewStackModel } from "./use-os-palette-view-stack";

export interface OsPaletteSurfaceModel {
  readonly root: OsPaletteRootModel;
  readonly execution: OsPaletteExecutionModel;
  readonly viewStack: OsPaletteViewStackModel;
  /** The palette's content element; the action panel anchors inside it. */
  readonly contentRef: RefObject<HTMLDivElement | null>;
  /** Command ids the daemon is currently running for this client. */
  readonly pending: ReadonlySet<string>;
  /** Every row on screen, in reading order. */
  readonly values: readonly string[];
  readonly selected: string;
  onSelectionChange(next: string): void;
}

export interface UseOsPaletteSurfaceOptions {
  readonly open: boolean;
  onOpenChange(open: boolean): void;
  readonly dispatch: CmdPaletteDispatch;
}

/**
 * Everything the palette is currently showing, assembled once.
 *
 * The root model, the execution state and the keyboard selection are separate
 * concerns that nevertheless have to agree on one thing — which row is
 * highlighted — so they are composed here rather than in the component. That
 * leaves the component with the two jobs only it can do: mounting the dialog and
 * ordering the Escape ladder.
 */
export function useOsPaletteSurface({
  open,
  onOpenChange,
  dispatch,
}: UseOsPaletteSurfaceOptions): OsPaletteSurfaceModel {
  const viewStack = useOsPaletteViewStack();
  const contentRef = useRef<HTMLDivElement>(null);
  const pending = useSelector(cmdPaletteExecutionStore, snapshot => snapshot.context.pending);
  const root = useOsPaletteRoot({
    open,
    onOpenChange,
    dispatch: (command, query) => dispatch.run(command, { query }),
    setPinned: (command, pinned) => void dispatch.setPinned(command, pinned),
  });
  const values = [
    ...root.sections.flatMap(section => section.commands.map(command => command.id)),
    ...(root.fallback === null ? [] : [root.fallback.value]),
    ...root.entities.sessions.map(session => `session:${session.sessionId}`),
    ...(root.destination ? [] : root.entities.tabs.map(tab => `tab:${tab.windowId}`)),
    ...(root.destination ? [] : root.entities.worktrees.map(entry => `worktree:${entry.key}`)),
    ...root.domainSections.flatMap(section => section.rows.map(row => row.key)),
  ];
  const [selection, setSelection] = useState<{ previous: readonly string[]; value: string }>(
    () => ({ previous: values, value: values[0] ?? "" })
  );
  // The highlight survives the catalog moving underneath it, falling to the
  // nearest neighbour only when its own row leaves.
  const selected = resolveCommandSelection(selection.previous, values, selection.value);
  const execution = useOsPaletteExecution({
    open,
    registry: root.registry,
    pins: root.pins,
    selected: resolvePaletteRowSubject(root.rowSources, selected),
    contentRef,
    runAction: root.runRowAction,
    runCommand: (command, options) => void dispatch.run(command, options),
  });

  return {
    root,
    execution,
    viewStack,
    contentRef,
    pending: new Set(Object.keys(pending)),
    values,
    selected,
    onSelectionChange: next => setSelection({ previous: values, value: next }),
  };
}
