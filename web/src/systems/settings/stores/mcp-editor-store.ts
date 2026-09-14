import { createStoreLogic } from "@xstate/store";

import { notifyUser } from "@/lib/user-feedback";

import type { SettingsLayeredScope } from "../types";
import type { MCPDraft } from "../lib/mcp-editor-model";
import type {
  SettingsMCPServerEntry,
  SettingsMCPServerTarget,
  SettingsMutationResult,
} from "../types";

export type MCPEditorState =
  | { mode: "closed" }
  | (MCPEditorScopeContext & {
      draft: MCPDraft;
      mode: "create";
    })
  | (MCPEditorScopeContext & {
      draft: MCPDraft;
      entry: SettingsMCPServerEntry;
      mode: "edit";
    });

export interface MCPEditorScopeContext {
  scope: SettingsLayeredScope;
  target: SettingsMCPServerTarget;
  workspaceId?: string;
  profileName?: string;
}

export type MCPEditorFlow = {
  editor: MCPEditorState;
  nextAttempt: number;
  pendingSaveAttempt: number | null;
};

/** Per-editor workflow; vault inventory stays Query-owned and accepted saves run in store effects. */
export const mcpEditorLogic = createStoreLogic<
  MCPEditorFlow,
  {
    editorOpened: { editor: Exclude<MCPEditorState, { mode: "closed" }> };
    draftChanged: { draft: MCPDraft };
    editorDismissed: {};
    saveFailed: { attempt: number };
    saveRequested: {
      execute: () => Promise<SettingsMutationResult>;
      name: string;
    };
    saveSucceeded: { attempt: number; name: string; result: SettingsMutationResult };
    targetChanged: { target: SettingsMCPServerTarget };
  }
>({
  context: (): MCPEditorFlow => ({
    editor: { mode: "closed" },
    nextAttempt: 0,
    pendingSaveAttempt: null,
  }),
  on: {
    editorOpened: (context, event: { editor: Exclude<MCPEditorState, { mode: "closed" }> }) => ({
      ...context,
      editor: event.editor,
      pendingSaveAttempt: null,
    }),
    draftChanged: (context, event: { draft: MCPDraft }) =>
      context.editor.mode === "closed"
        ? undefined
        : { ...context, editor: { ...context.editor, draft: event.draft } },
    editorDismissed: context =>
      context.pendingSaveAttempt === null ? { ...context, editor: { mode: "closed" } } : undefined,
    saveFailed: (context, event: { attempt: number }) =>
      context.pendingSaveAttempt === event.attempt
        ? { ...context, pendingSaveAttempt: null }
        : undefined,
    saveRequested: (context, event, enqueue) => {
      if (context.editor.mode === "closed" || context.pendingSaveAttempt !== null) return;
      const attempt = context.nextAttempt + 1;
      enqueue.effect(async ({ trigger }) => {
        try {
          trigger.saveSucceeded({
            attempt,
            name: event.name,
            result: await event.execute(),
          });
        } catch {
          trigger.saveFailed({ attempt });
        }
      });
      return { ...context, nextAttempt: attempt, pendingSaveAttempt: attempt };
    },
    saveSucceeded: (context, event, enqueue) => {
      if (context.pendingSaveAttempt !== event.attempt) return;
      enqueue.effect(() =>
        notifyUser({ message: mcpSaveMessage(event.name, event.result), tone: "success" })
      );
      return { ...context, editor: { mode: "closed" }, pendingSaveAttempt: null };
    },
    targetChanged: (context, event: { target: SettingsMCPServerTarget }) =>
      context.editor.mode === "closed"
        ? undefined
        : { ...context, editor: { ...context.editor, target: event.target } },
  },
});

function mcpSaveMessage(name: string, result: SettingsMutationResult): string {
  const target = result.write_target ?? "write target unavailable";
  const lifecycle = result.restart_required ? "restart required" : "applied now";
  return `Saved "${name}" · ${target} · ${lifecycle}`;
}
