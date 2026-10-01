import { useSelector } from "@xstate/store-react";
import { useRef, useState, type RefObject } from "react";

import { resolveCommandSelection } from "@compozy/ui";

import { resolvePaletteRowSubject } from "../lib/cmd-palette-row-actions";
import { cmdPaletteExecutionStore } from "../stores/cmd-palette-execution-store";
import type { CmdPaletteDispatch } from "./use-cmd-palette-dispatch";
import { paletteSelectionValues } from "./os-palette-selection-values";
import { useOsPaletteExecution, type OsPaletteExecutionModel } from "./use-os-palette-execution";
import { useOsPaletteRoot, type OsPaletteRootModel } from "./use-os-palette-root";
import { useOsPaletteViewStack, type OsPaletteViewStackModel } from "./use-os-palette-view-stack";

export { paletteSelectionValues } from "./os-palette-selection-values";

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

interface PaletteSurfaceSelection {
  readonly previous: readonly string[];
  readonly value: string;
  /** True once the operator moved or acted on the highlight; false while it is automatic. */
  readonly chosen: boolean;
  /** The overlay state and query the selection was made under. */
  readonly open: boolean;
  readonly query: string;
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
    dispatch: (command, query, navigate) =>
      dispatch.run(command, { query, ...(navigate === undefined ? {} : { navigate }) }),
    setPinned: (command, pinned) => void dispatch.setPinned(command, pinned),
  });
  const values = paletteSelectionValues(root);
  const [selection, setSelection] = useState<PaletteSurfaceSelection>(() => ({
    previous: values,
    value: values[0] ?? "",
    chosen: false,
    open,
    query: root.query,
  }));
  // Opening the palette or changing the query starts over at the top result.
  // The hook outlives the overlay, so without this a row picked while the
  // palette was closed (or before the query changed) stayed highlighted even
  // when it had scrolled far out of view.
  const restart = selection.open !== open || selection.query !== root.query;
  const chosen = !restart && selection.chosen;
  // An automatic highlight follows the top row while ranking settles (recents
  // and rank signals arrive after the palette opens). A highlight the operator
  // chose — by keyboard, pointer, or by acting on it — survives the catalog
  // moving underneath it, falling to the nearest neighbour only when its own
  // row leaves.
  const selected = chosen
    ? resolveCommandSelection(selection.previous, values, selection.value)
    : (values[0] ?? "");
  if (
    restart ||
    selection.value !== selected ||
    selection.previous.length !== values.length ||
    selection.previous.some((value, index) => value !== values[index])
  ) {
    setSelection({ previous: values, value: selected, chosen, open, query: root.query });
  }
  const execution = useOsPaletteExecution({
    open,
    registry: root.registry,
    pins: root.pins,
    selected: resolvePaletteRowSubject(root.rowSources, selected),
    contentRef,
    runAction: root.runRowAction,
    runCommand: (command, options) => void dispatch.run(command, options),
  });

  // Acting on a row pins the highlight to it: the action panel must keep
  // pointing at the row it was opened for.
  if (execution.panel.open && !chosen) {
    setSelection({ previous: values, value: selected, chosen: true, open, query: root.query });
  }

  return {
    root,
    execution,
    viewStack,
    contentRef,
    pending: new Set(Object.keys(pending)),
    values,
    selected,
    onSelectionChange: next => {
      // cmdk also reports the value it already holds; only a move is a choice.
      if (next === selected) return;
      setSelection({ previous: values, value: next, chosen: true, open, query: root.query });
    },
  };
}
