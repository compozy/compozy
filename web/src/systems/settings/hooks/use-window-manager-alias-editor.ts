import { useState } from "react";

import {
  WindowManagerSettingsError,
  type WindowManagerAliasMap,
  type WindowManagerSettingsSection,
} from "@/systems/os";

import type { WindowManagerBindingMutations } from "./use-window-manager-binding-mutations";

/** The rule, stated the way the daemon states it (`ValidateCmdPaletteAlias`). */
export const ALIAS_RULE_HINT = "1–32 characters, no whitespace";

export interface AliasCellState {
  value: string;
  /** `grammar` for the local rule; otherwise the daemon's own message. */
  problem: string | null;
  saving: boolean;
}

/** An alias the daemon refused because another command already answers to it. */
export interface AliasConflict {
  commandId: string;
  alias: string;
  owner: string;
  ownerTitle: string;
  desired: WindowManagerAliasMap;
}

export interface AliasEditorModel {
  cell: (commandId: string) => AliasCellState;
  conflict: AliasConflict | null;
  change: (commandId: string, value: string) => void;
  commit: (commandId: string) => void;
  cancel: (commandId: string) => void;
  overwrite: () => void;
  dismissConflict: () => void;
}

function grammarProblem(alias: string): boolean {
  if (alias === "") return false;
  return alias.length > 32 || /\s/.test(alias);
}

/**
 * Inline alias editing against the daemon's alias table.
 *
 * Aliases are unique per workspace, so assigning one that is taken is refused
 * with the current owner named and an explicit transfer offered — the same
 * shape a contested chord takes (US-023.AC-2).
 */
export function useWindowManagerAliasEditor(
  section: WindowManagerSettingsSection,
  mutations: WindowManagerBindingMutations,
  titleFor: (commandId: string) => string
): AliasEditorModel {
  const [drafts, setDrafts] = useState<Readonly<Record<string, string>>>({});
  const [problems, setProblems] = useState<Readonly<Record<string, string>>>({});
  const [pending, setPending] = useState<string | null>(null);
  const [conflict, setConflict] = useState<AliasConflict | null>(null);
  const stored = section.aliases;
  const { commit } = mutations;

  const clearDraft = (commandId: string) => {
    setDrafts(current => {
      const next = { ...current };
      delete next[commandId];
      return next;
    });
    setProblems(current => {
      const next = { ...current };
      delete next[commandId];
      return next;
    });
  };

  const send = async (commandId: string, desired: WindowManagerAliasMap, alias: string) => {
    setPending(commandId);
    try {
      await commit({ aliases: desired });
      clearDraft(commandId);
      setConflict(null);
    } catch (cause) {
      if (cause instanceof WindowManagerSettingsError && cause.code === "alias_conflict") {
        const owner = cause.owner ?? "";
        setConflict({ commandId, alias, owner, ownerTitle: titleFor(owner), desired });
        return;
      }
      setProblems(current => ({
        ...current,
        [commandId]: cause instanceof Error ? cause.message : "Unable to save the alias.",
      }));
    } finally {
      setPending(null);
    }
  };

  return {
    cell: commandId => ({
      value: drafts[commandId] ?? stored[commandId] ?? "",
      problem: problems[commandId] ?? null,
      saving: pending === commandId,
    }),
    conflict,
    change: (commandId, value) => {
      setDrafts(current => ({ ...current, [commandId]: value }));
      setProblems(current => {
        const next = { ...current };
        // Validate as they type here rather than on blur: the field is one word
        // long and the only reachable fault is a space, so waiting to report it
        // would be slower feedback for no gain in calm.
        if (grammarProblem(value)) next[commandId] = "grammar";
        else delete next[commandId];
        return next;
      });
    },
    commit: commandId => {
      const draft = drafts[commandId];
      if (draft === undefined) return;
      const alias = draft.trim();
      if (grammarProblem(draft)) return;
      if (alias === (stored[commandId] ?? "")) {
        clearDraft(commandId);
        return;
      }
      const desired: Record<string, string> = { ...stored };
      if (alias === "") delete desired[commandId];
      else desired[commandId] = alias;
      void send(commandId, desired, alias);
    },
    cancel: commandId => clearDraft(commandId),
    overwrite: () => {
      if (conflict === null) return;
      const { commandId, desired, alias } = conflict;
      setPending(commandId);
      void commit({ aliases: desired, overwrite: true })
        .then(() => {
          clearDraft(commandId);
          setConflict(null);
        })
        .catch((cause: unknown) => {
          setProblems(current => ({
            ...current,
            [commandId]: cause instanceof Error ? cause.message : `Unable to move ${alias}.`,
          }));
        })
        .finally(() => setPending(null));
    },
    dismissConflict: () => setConflict(null),
  };
}
