import { useSelector } from "@xstate/store-react";
import { useEffect, useEffectEvent, useState, type RefObject } from "react";

import {
  createArgsState,
  setArgValue,
  submitArgs,
  type PaletteArgsState,
} from "../lib/cmd-palette-args";
import {
  flattenRowActions,
  paletteRowActions,
  type PaletteRowAction,
  type PaletteRowActionModel,
  type PaletteRowSubject,
} from "../lib/cmd-palette-row-actions";
import type {
  CmdPaletteConfirmation,
  PaletteRegistry,
  ResolvedPaletteCommand,
} from "../lib/cmd-palette-types";
import {
  parseShortcutChord,
  primaryShortcutModifier,
  shortcutMatches,
} from "../lib/window-manager-shortcuts";
import {
  cmdPaletteExecutionStore,
  resetPaletteExecutionEntry,
} from "../stores/cmd-palette-execution-store";

/** The chord that opens the palette is the chord that toggles the panel (`_uiux.md` a11y notes). */
const PANEL_TOGGLE_COMMAND_ID = "palette.open";
const VANISHED_TARGET_REASON = "target changed — this command is no longer available";

export type OsPaletteMode = "search" | "args" | "confirm";

export interface OsPaletteConfirmState {
  readonly confirmation: CmdPaletteConfirmation;
  readonly destructive: boolean;
  /** Non-empty once the target stopped matching what the operator triggered. */
  readonly invalidatedReason: string;
}

export interface OsPalettePanelState {
  readonly open: boolean;
  readonly filter: string;
  readonly model: PaletteRowActionModel | null;
  readonly anchor: HTMLElement | null;
}

export interface OsPaletteExecutionModel {
  readonly mode: OsPaletteMode;
  readonly args: PaletteArgsState | null;
  readonly confirm: OsPaletteConfirmState | null;
  readonly panel: OsPalettePanelState;
  setPanelFilter(filter: string): void;
  setPanelOpen(open: boolean): void;
  changeArg(name: string, value: string): void;
  /** Returns the field that blocked the submit, so the bar can focus it. */
  submit(): string | null;
  runAction(action: PaletteRowAction): void;
  confirmNow(): void;
  cancel(): void;
  /** Consumes one Escape rung; `false` means the palette itself should close. */
  escape(): boolean;
}

export interface UseOsPaletteExecutionOptions {
  readonly open: boolean;
  readonly registry: PaletteRegistry;
  readonly pins: readonly string[];
  /** The row the operator has highlighted, or `null` when the list is empty. */
  readonly selected: PaletteRowSubject | null;
  /** The palette's content element; the panel anchors to the selected row inside it. */
  readonly contentRef: RefObject<HTMLElement | null>;
  /** Runs one panel action; the shell owns what each intent can touch. */
  runAction(action: PaletteRowAction): void;
  /** Runs a command with collected arguments and a cleared confirmation. */
  runCommand(
    command: ResolvedPaletteCommand,
    options: { args?: Readonly<Record<string, unknown>>; confirmed?: boolean }
  ): void;
}

function anchorFor(contentRef: RefObject<HTMLElement | null>, key: string): HTMLElement | null {
  const root = contentRef.current;
  if (root === null) return null;
  const selector =
    typeof CSS !== "undefined" && typeof CSS.escape === "function" ? CSS.escape(key) : key;
  const found = root.querySelector(`[data-palette-row="${selector}"]`);
  return found instanceof HTMLElement ? found : null;
}

/**
 * The palette's execution state: the action panel, the argument step, the
 * confirmation step, and the Escape ladder that orders them.
 *
 * All three live here rather than in their components because they are one
 * machine, not three: Escape has to know which rung is innermost, the toggle
 * chord has to know that arguments are open, and a row leaving the list has to
 * close a panel that was pointing at it. Splitting that across components is how
 * a ladder ends up with two owners disagreeing about the top rung.
 */
export function useOsPaletteExecution({
  open,
  registry,
  pins,
  selected,
  contentRef,
  runAction,
  runCommand,
}: UseOsPaletteExecutionOptions): OsPaletteExecutionModel {
  const entry = useSelector(cmdPaletteExecutionStore, snapshot => snapshot.context.entry);
  const draft = useSelector(cmdPaletteExecutionStore, snapshot => snapshot.context.draft);
  /** The row the panel was opened for; openness is derived from it. */
  const [panelKey, setPanelKey] = useState<string | null>(null);
  const [panelFilter, setPanelFilter] = useState("");
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);

  const liveCommand = entry === null ? null : (registry.byId.get(entry.commandId) ?? null);
  const model = selected === null ? null : paletteRowActions({ subject: selected, registry, pins });

  /*
   * The argument step is derived, not seeded: the fields are a pure function of
   * the command until the operator edits them, and the edits are a draft tied to
   * that command's id. Deriving it is what makes a bound hotkey land straight in
   * argument mode — a write scheduled from an effect would leave one frame
   * showing the search list the operator never asked for.
   */
  const args: PaletteArgsState | null =
    entry?.kind === "args" && liveCommand !== null
      ? draft?.commandId === liveCommand.id
        ? draft.state
        : createArgsState(liveCommand)
      : null;

  const mode: OsPaletteMode =
    entry?.kind === "confirm" && entry.confirmation !== null
      ? "confirm"
      : args !== null
        ? "args"
        : "search";

  /*
   * The confirmation renders the copy captured at trigger time, but its verdict
   * comes from the live catalog: a command that vanished or turned unavailable
   * between trigger and confirm shows the honest reason and loses its confirm
   * control rather than executing against a target that moved (US-016.EC-2).
   */
  const confirm: OsPaletteConfirmState | null =
    mode === "confirm" && entry?.confirmation != null
      ? {
          confirmation: entry.confirmation,
          destructive: entry.destructive,
          invalidatedReason:
            liveCommand === null
              ? VANISHED_TARGET_REASON
              : liveCommand.available
                ? ""
                : liveCommand.reason.trim() || VANISHED_TARGET_REASON,
        }
      : null;

  /*
   * The panel belongs to exactly one row, so its openness is that row still
   * being the selected one. When a live refresh takes the row away the selection
   * moves on and the panel is closed by derivation — no write during render, and
   * no frame in which it describes a row the operator is no longer on
   * (US-014.EC-1).
   */
  const panelOpen = panelKey !== null && panelKey === model?.key;

  const closePanel = () => {
    setPanelKey(null);
    setPanelFilter("");
    setAnchor(null);
  };

  const cancel = () => {
    closePanel();
    // Clearing the entry discards the draft with it, password fields included
    // (US-015.EC-1, US-015.EC-4).
    resetPaletteExecutionEntry();
  };

  const openPanel = () => {
    if (model === null) return;
    setAnchor(anchorFor(contentRef, model.key));
    setPanelKey(model.key);
    setPanelFilter("");
  };

  const runPanelAction = (action: PaletteRowAction) => {
    closePanel();
    runAction(action);
  };

  const submit = (): string | null => {
    // The catalog can move while the operator types. Running a command the
    // registry no longer carries would be acting on a guess, so the step stands
    // down instead and the palette returns to search.
    if (args === null || liveCommand === null) {
      if (args !== null) cancel();
      return null;
    }
    const submission = submitArgs(args);
    if (submission.values === null) {
      cmdPaletteExecutionStore.trigger.argsDrafted({
        draft: { commandId: liveCommand.id, state: submission.state },
      });
      return submission.state.focusField;
    }
    resetPaletteExecutionEntry();
    runCommand(liveCommand, { args: submission.values });
    return null;
  };

  const confirmNow = () => {
    if (confirm === null || confirm.invalidatedReason !== "" || liveCommand === null) return;
    const collected = entry?.args ?? {};
    resetPaletteExecutionEntry();
    runCommand(liveCommand, { args: collected, confirmed: true });
  };

  const handleKeyDown = useEffectEvent((event: KeyboardEvent) => {
    // A held key must not re-fire an action (US-014.EC-3).
    if (event.repeat) return;
    const primaryModifier = primaryShortcutModifier(navigator.platform);
    if (mode === "search") {
      for (const binding of registry.byId.get(PANEL_TOGGLE_COMMAND_ID)?.bindings ?? []) {
        const chord = parseShortcutChord(binding);
        if (chord === null || !shortcutMatches(event, chord, primaryModifier)) continue;
        event.preventDefault();
        // Stops the shell's own listener from reading this as "close the palette".
        event.stopPropagation();
        if (panelOpen) closePanel();
        else openPanel();
        return;
      }
    }
    if (model === null || mode !== "search") return;
    // An action's chord fires for the selected row wherever focus drifted inside
    // the palette — that is what the capture phase buys (US-014.EC-3).
    for (const action of flattenRowActions(model.sections)) {
      for (const binding of action.bindings) {
        const chord = parseShortcutChord(binding);
        if (chord === null || !shortcutMatches(event, chord, primaryModifier)) continue;
        event.preventDefault();
        event.stopPropagation();
        runPanelAction(action);
        return;
      }
    }
  });

  useEffect(() => {
    if (!open) return undefined;
    const listener = (event: KeyboardEvent) => handleKeyDown(event);
    document.addEventListener("keydown", listener, true);
    return () => document.removeEventListener("keydown", listener, true);
  }, [open]);

  const escape = (): boolean => {
    if (panelOpen) {
      closePanel();
      return true;
    }
    if (mode !== "search") {
      cancel();
      return true;
    }
    return false;
  };

  return {
    mode,
    args,
    confirm,
    panel: { open: panelOpen, filter: panelFilter, model, anchor },
    setPanelFilter,
    setPanelOpen: next => {
      if (next) openPanel();
      else closePanel();
    },
    changeArg: (name, value) => {
      if (args === null || liveCommand === null) return;
      cmdPaletteExecutionStore.trigger.argsDrafted({
        draft: { commandId: liveCommand.id, state: setArgValue(args, name, value) },
      });
    },
    submit,
    runAction: runPanelAction,
    confirmNow,
    cancel,
    escape,
  };
}
