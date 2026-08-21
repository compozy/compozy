import { useEffect, useRef } from "react";
import { useSelector, useStore } from "@xstate/store-react";

import {
  WindowManagerSettingsError,
  type WindowManagerAliasMap,
  type WindowManagerSettingsSection,
} from "@/systems/os";

import { aliasEditorLogic, type AliasConflict } from "../stores/window-manager-alias-editor-store";
import type { WindowManagerBindingMutations } from "./use-window-manager-binding-mutations";

export const ALIAS_RULE_HINT = "1–32 characters, no whitespace";

export interface AliasCellState {
  value: string;
  problem: string | null;
  saving: boolean;
}

export type { AliasConflict };

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
  const store = useStore(aliasEditorLogic);
  const drafts = useSelector(store, snapshot => snapshot.context.drafts);
  const problems = useSelector(store, snapshot => snapshot.context.problems);
  const conflict = useSelector(store, snapshot => snapshot.context.conflict);
  const aliasesRef = useRef(section.aliases);
  useEffect(() => {
    aliasesRef.current = section.aliases;
  }, [section.aliases]);
  const sendTail = useRef<Promise<void> | null>(null);
  const { commit } = mutations;

  const send = async (commandId: string, desired: WindowManagerAliasMap, alias: string) => {
    try {
      const nextSection = await commit({ aliases: desired });
      aliasesRef.current = nextSection.aliases;
      store.trigger.draftCleared({ commandId });
      store.trigger.conflictDismissed();
    } catch (cause) {
      if (cause instanceof WindowManagerSettingsError && cause.code === "alias_conflict") {
        const owner = cause.owner ?? "";
        store.trigger.conflictSet({
          conflict: { commandId, alias, owner, ownerTitle: titleFor(owner), desired },
        });
        return;
      }
      store.trigger.problemSet({
        commandId,
        problem: cause instanceof Error ? cause.message : "Unable to save the alias.",
      });
    }
  };

  return {
    cell: commandId => ({
      value: drafts[commandId] ?? aliasesRef.current[commandId] ?? "",
      problem: problems[commandId] ?? null,
      saving: mutations.saving,
    }),
    conflict,
    change: (commandId, value) => {
      store.trigger.draftChanged({
        commandId,
        problem: grammarProblem(value) ? "grammar" : null,
        value,
      });
    },
    commit: commandId => {
      const draft = drafts[commandId];
      if (draft === undefined) return;
      const alias = draft.trim();
      if (grammarProblem(draft)) return;
      const run = async () => {
        if (alias === (aliasesRef.current[commandId] ?? "")) {
          store.trigger.draftCleared({ commandId });
          return;
        }
        const desired: Record<string, string> = { ...aliasesRef.current };
        if (alias === "") delete desired[commandId];
        else desired[commandId] = alias;
        await send(commandId, desired, alias);
      };
      const next = (sendTail.current ?? Promise.resolve()).then(run, run);
      sendTail.current = next.then(
        () => undefined,
        () => undefined
      );
      void next;
    },
    cancel: commandId => store.trigger.draftCleared({ commandId }),
    overwrite: () => {
      if (conflict === null) return;
      const { commandId, desired, alias } = conflict;
      void commit({ aliases: desired, overwrite: true })
        .then(nextSection => {
          aliasesRef.current = nextSection.aliases;
          store.trigger.draftCleared({ commandId });
          store.trigger.conflictDismissed();
        })
        .catch((cause: unknown) => {
          store.trigger.problemSet({
            commandId,
            problem: cause instanceof Error ? cause.message : `Unable to move ${alias}.`,
          });
        });
    },
    dismissConflict: () => store.trigger.conflictDismissed(),
  };
}
