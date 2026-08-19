import { createStore } from "@xstate/store";

import type { PaletteArgsState } from "../lib/cmd-palette-args";
import type { CmdPaletteConfirmation, ResolvedPaletteCommand } from "../lib/cmd-palette-types";

/**
 * The two pieces of execution state that must outlive the palette tree.
 *
 * Everything else about an execution — which panel is open, what is typed into
 * an argument field — dies with the palette, which is exactly what makes
 * "reopening starts at root" a property of the state instead of a rule every
 * dismissal path has to remember. These two cannot: the entry intent is written
 * *before* the palette mounts (a bound hotkey opening straight into arguments,
 * US-015.EC-3), and the pending map is written by the dispatch seam, which lives
 * in the shell and keeps running after the operator closes the overlay
 * (US-017.AC-2).
 */

export type CmdPaletteEntryKind = "args" | "confirm";

export interface CmdPaletteEntryIntent {
  readonly kind: CmdPaletteEntryKind;
  readonly commandId: string;
  /** Values already collected — an args submission carries them into the confirm step. */
  readonly args: Readonly<Record<string, unknown>>;
  /**
   * The confirmation copy as declared when the operator triggered the command.
   * Snapshotting it is what lets an invalidated target still render its own
   * words plus the honest reason, instead of a blank dialog (US-016.EC-2).
   */
  readonly confirmation: CmdPaletteConfirmation | null;
  readonly destructive: boolean;
}

export interface CmdPalettePendingCommand {
  readonly commandId: string;
  readonly title: string;
}

/**
 * What the operator has typed into the current argument step.
 *
 * It lives beside the entry rather than in the component because its lifetime
 * *is* the entry's lifetime: every path that ends the step — Escape, a submit,
 * the overlay closing — clears both in the same transition, so a password value
 * cannot outlive the field it was typed into (US-015.EC-4, Safety Invariant 6).
 */
export interface CmdPaletteArgsDraft {
  readonly commandId: string;
  readonly state: PaletteArgsState;
}

export interface CmdPaletteExecutionState {
  readonly entry: CmdPaletteEntryIntent | null;
  readonly draft: CmdPaletteArgsDraft | null;
  /** In-flight daemon invocations, keyed by command id. */
  readonly pending: Readonly<Record<string, CmdPalettePendingCommand>>;
}

type CmdPaletteExecutionEvents = {
  entryRequested: { intent: CmdPaletteEntryIntent };
  entryCleared: {};
  argsDrafted: { draft: CmdPaletteArgsDraft };
  pendingStarted: { pending: CmdPalettePendingCommand };
  pendingSettled: { commandId: string };
};

function initialState(): CmdPaletteExecutionState {
  return { entry: null, draft: null, pending: {} };
}

export const cmdPaletteExecutionStore = createStore<
  CmdPaletteExecutionState,
  CmdPaletteExecutionEvents
>({
  context: initialState(),
  on: {
    entryRequested: (state, event: { intent: CmdPaletteEntryIntent }) => ({
      ...state,
      entry: event.intent,
      draft: null,
    }),
    entryCleared: state =>
      state.entry === null && state.draft === null
        ? undefined
        : { ...state, entry: null, draft: null },
    argsDrafted: (state, event: { draft: CmdPaletteArgsDraft }) => ({
      ...state,
      draft: event.draft,
    }),
    pendingStarted: (state, event: { pending: CmdPalettePendingCommand }) => ({
      ...state,
      pending: { ...state.pending, [event.pending.commandId]: event.pending },
    }),
    pendingSettled: (state, event: { commandId: string }) => {
      if (state.pending[event.commandId] === undefined) return undefined;
      const pending = { ...state.pending };
      delete pending[event.commandId];
      return { ...state, pending };
    },
  },
});

/** Opens the palette on a command's argument step. */
export function requestPaletteArgs(command: ResolvedPaletteCommand): void {
  cmdPaletteExecutionStore.trigger.entryRequested({
    intent: {
      kind: "args",
      commandId: command.id,
      args: {},
      confirmation: null,
      destructive: command.destructive,
    },
  });
}

/** Opens the palette on a command's declared confirmation, carrying any collected args. */
export function requestPaletteConfirmation(
  command: ResolvedPaletteCommand,
  args: Readonly<Record<string, unknown>>
): void {
  cmdPaletteExecutionStore.trigger.entryRequested({
    intent: {
      kind: "confirm",
      commandId: command.id,
      args,
      confirmation: command.confirmation ?? null,
      destructive: command.destructive,
    },
  });
}

/**
 * Dropped whenever the overlay opens or closes, beside the view stack — one
 * place decides that a fresh palette starts clean.
 */
export function resetPaletteExecutionEntry(): void {
  cmdPaletteExecutionStore.trigger.entryCleared({});
}
