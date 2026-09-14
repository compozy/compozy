import { createStoreLogic } from "@xstate/store";
import { useSelector, useStore } from "@xstate/store-react";

import { deriveMCPManagementFilter, mcpDefinitionKey } from "../lib/mcp-management-target";
import {
  toMCPOverrideDraft,
  toMCPOverrideRequest,
  validateMCPOverride,
  type MCPOverrideDraft,
} from "../lib/mcp-override-model";
import type { SettingsMCPServerEntry } from "../types";
import { useDeleteSettingsMCPServer, usePutSettingsMCPServer } from "./use-settings-mutations";

type Editor = { entry: SettingsMCPServerEntry; draft: MCPOverrideDraft };
type State = { editor: Editor | null; pending: boolean; error: string | null };
const overrideLogic = createStoreLogic({
  context: (): State => ({ editor: null, pending: false, error: null }),
  on: {
    opened: (context, event: { entry: SettingsMCPServerEntry }) =>
      context.pending
        ? undefined
        : {
            ...context,
            editor: { entry: event.entry, draft: toMCPOverrideDraft(event.entry) },
            error: null,
          },
    changed: (context, event: { draft: MCPOverrideDraft }) =>
      !context.editor || context.pending
        ? undefined
        : {
            ...context,
            editor: { ...context.editor, draft: event.draft },
            error: null,
          },
    dismissed: context => (context.pending ? undefined : { ...context, editor: null, error: null }),
    requested: (context, event: { execute: () => Promise<unknown> }, enqueue) => {
      if (!context.editor || context.pending) return;
      enqueue.effect(async ({ trigger }) => {
        try {
          await event.execute();
          trigger.succeeded();
        } catch (error) {
          trigger.failed({
            message: error instanceof Error ? error.message : "Could not save the override",
          });
        }
      });
      return { ...context, pending: true, error: null };
    },
    succeeded: context => ({ ...context, pending: false, editor: null, error: null }),
    failed: (context, event: { message: string }) => ({
      ...context,
      pending: false,
      error: event.message,
    }),
  },
});

/** Both Marketplace and Settings use this exact definition target and override-only write path. */
export function useMCPOverrideEditor(enabled = true) {
  const store = useStore(overrideLogic);
  const state = useSelector(store, snapshot => snapshot.context);
  const put = usePutSettingsMCPServer();
  const remove = useDeleteSettingsMCPServer();
  const editor = state.editor;
  const filter = editor ? deriveMCPManagementFilter(editor.entry) : null;
  const validation = editor ? validateMCPOverride(editor.draft, editor.entry.transport) : null;
  const openEdit = (entry: SettingsMCPServerEntry) => {
    const target = deriveMCPManagementFilter(entry);
    if (enabled && target?.owner?.startsWith("extension:")) store.trigger.opened({ entry });
  };
  const save = () => {
    if (!enabled || !editor || !filter || !validation?.valid) return;
    const name = editor.entry.name;
    const body = toMCPOverrideRequest(name, editor.draft);
    store.trigger.requested({ execute: () => put.mutateAsync({ name, filter, body }) });
  };
  const reset = () => {
    if (!enabled || !editor || !filter) return;
    const name = editor.entry.name;
    store.trigger.requested({ execute: () => remove.mutateAsync({ name, filter }) });
  };
  return {
    openEdit,
    editorProps:
      !enabled || !editor
        ? null
        : {
            open: true,
            entry: editor.entry,
            identity: mcpDefinitionKey(editor.entry),
            draft: editor.draft,
            errors: validation?.errors ?? {},
            isValid: validation?.valid ?? false,
            isSaving: state.pending,
            saveError: state.error,
            onChange: (draft: MCPOverrideDraft) => store.trigger.changed({ draft }),
            onClose: () => store.trigger.dismissed(),
            onSave: save,
            onReset: reset,
          },
  };
}
