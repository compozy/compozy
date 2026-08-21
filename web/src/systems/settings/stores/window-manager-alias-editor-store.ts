import { createStoreLogic } from "@xstate/store";

import type { WindowManagerAliasMap } from "@/systems/os";

export interface AliasConflict {
  commandId: string;
  alias: string;
  owner: string;
  ownerTitle: string;
  desired: WindowManagerAliasMap;
}

export interface AliasEditorContext {
  conflict: AliasConflict | null;
  drafts: Readonly<Record<string, string>>;
  problems: Readonly<Record<string, string>>;
}

function withoutKey(
  source: Readonly<Record<string, string>>,
  commandId: string
): Readonly<Record<string, string>> {
  if (!(commandId in source)) return source;
  const next = { ...source };
  delete next[commandId];
  return next;
}

export const aliasEditorLogic = createStoreLogic({
  context: (): AliasEditorContext => ({
    conflict: null,
    drafts: {},
    problems: {},
  }),
  on: {
    draftChanged(
      context,
      event: { commandId: string; problem: string | null; value: string }
    ): AliasEditorContext {
      const problems = { ...context.problems };
      if (event.problem === null) delete problems[event.commandId];
      else problems[event.commandId] = event.problem;
      return {
        ...context,
        drafts: { ...context.drafts, [event.commandId]: event.value },
        problems,
      };
    },
    draftCleared(context, event: { commandId: string }): AliasEditorContext {
      return {
        ...context,
        drafts: withoutKey(context.drafts, event.commandId),
        problems: withoutKey(context.problems, event.commandId),
      };
    },
    conflictSet(context, event: { conflict: AliasConflict }): AliasEditorContext {
      return { ...context, conflict: event.conflict };
    },
    conflictDismissed(context): AliasEditorContext {
      return { ...context, conflict: null };
    },
    problemSet(context, event: { commandId: string; problem: string }): AliasEditorContext {
      return {
        ...context,
        problems: { ...context.problems, [event.commandId]: event.problem },
      };
    },
  },
});
