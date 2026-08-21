import { createStoreLogic } from "@xstate/store";

import { notifyUser } from "@/lib/user-feedback";

import type { SettingsLayeredScope } from "@/systems/settings";
import type {
  MCPDraft,
  SettingsMCPServerEntry,
  SettingsMCPServerTarget,
  SettingsMutationResult,
} from "@/systems/settings";

export type MarketplaceMCPEditorState =
  | { mode: "closed" }
  | (MarketplaceMCPEditorScopeContext & {
      draft: MCPDraft;
      mode: "create";
    })
  | (MarketplaceMCPEditorScopeContext & {
      draft: MCPDraft;
      entry: SettingsMCPServerEntry;
      mode: "edit";
    });

export interface MarketplaceMCPEditorScopeContext {
  scope: SettingsLayeredScope;
  target: SettingsMCPServerTarget;
  workspaceId?: string;
  profileName?: string;
}

export type MarketplaceMCPEditorFlow = {
  editor: MarketplaceMCPEditorState;
  nextAttempt: number;
  pendingSaveAttempt: number | null;
};

/** Per-editor workflow; vault inventory stays Query-owned and accepted saves run in store effects. */
export const marketplaceMCPEditorLogic = createStoreLogic<
  MarketplaceMCPEditorFlow,
  {
    editorOpened: { editor: Exclude<MarketplaceMCPEditorState, { mode: "closed" }> };
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
  context: (): MarketplaceMCPEditorFlow => ({
    editor: { mode: "closed" },
    nextAttempt: 0,
    pendingSaveAttempt: null,
  }),
  on: {
    editorOpened: (
      context,
      event: { editor: Exclude<MarketplaceMCPEditorState, { mode: "closed" }> }
    ) => ({
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
