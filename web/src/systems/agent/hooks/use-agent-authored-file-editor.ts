import { useSelector } from "@xstate/store-react";
import { type ReactNode, type SetStateAction } from "react";

import { useStoreBinding } from "@/hooks/use-store-binding";

import {
  createAuthoredFileEditorLogic,
  isAuthoredFileMissing,
  shouldAdoptAuthoredFileSource,
  type AuthoredFileKind,
  type AuthoredFilePayload,
} from "../stores/agent-authored-file-editor-store";
import type { AgentHeartbeatHistoryResponse, AgentSoulHistoryResponse } from "../types";
import { useUnsavedGuard } from "./use-unsaved-guard";

export type { AuthoredFileKind, AuthoredFilePayload };
export { isAuthoredFileMissing };

export interface UseAgentAuthoredFileEditorArgs {
  resourceKey: string;
  kind: AuthoredFileKind;
  payload: AuthoredFilePayload | undefined;
  history: AgentSoulHistoryResponse | AgentHeartbeatHistoryResponse | undefined;
  onValidate: (body: string) => Promise<{
    diagnostics?: Array<{ message: string; line?: number; source_path?: string }>;
    validation_status?: string;
  }>;
  onSave: (body: string, expectedDigest: string) => Promise<AuthoredFilePayload>;
  onRestore: (revisionId: string, expectedDigest: string) => Promise<AuthoredFilePayload>;
}

const CREATE_BODIES: Record<AuthoredFileKind, string> = {
  soul: "# Soul\n\nDefine persona and constraints.\n",
  heartbeat: "---\nenabled: true\n---\n\nCheck for pending work.\n",
};

export function buildAuthoredFileResourceKey(
  workspaceId: string | null,
  agentName: string,
  kind: AuthoredFileKind
): string {
  return JSON.stringify([workspaceId, agentName, kind]);
}

export function useAgentAuthoredFileEditor({
  resourceKey,
  kind,
  payload,
  history,
  onValidate,
  onSave,
  onRestore,
}: UseAgentAuthoredFileEditorArgs) {
  const fileLabel = kind === "soul" ? "SOUL.md" : "HEARTBEAT.md";
  const input = { resourceKey, kind, payload };
  const { replace, store } = useStoreBinding(
    resourceKey,
    () => createAuthoredFileEditorLogic().createStore(input),
    () => createAuthoredFileEditorLogic().createStore(input),
    (current, nextResourceKey) =>
      current.key !== nextResourceKey ||
      shouldAdoptAuthoredFileSource(current.store.getSnapshot().context, input)
  );
  const editor = useSelector(store, snapshot => snapshot.context);

  const dirty = editor.draft !== editor.baseline.body;
  const guard = useUnsavedGuard({ dirty, entityName: fileLabel });
  const editorError = "error" in editor ? editor.error : null;
  const revisions = history && "revisions" in history ? (history.revisions ?? []) : [];

  return {
    fileLabel,
    draft: editor.draft,
    setDraft: (draft: string) => store.trigger.draftChanged({ draft }),
    diagnostics: editor.diagnostics,
    phase: editor.phase,
    error: editorError,
    showHistory: editor.showHistory,
    setShowHistory: (update: SetStateAction<boolean>) => {
      const visible = typeof update === "function" ? update(editor.showHistory) : update;
      store.trigger.historyVisibilityChanged({ visible });
    },
    dirty,
    guardDialog: guard.confirmDialog as ReactNode,
    handleValidate: () => store.trigger.validationRequested({ validate: onValidate }),
    handleSave: () => store.trigger.saveRequested({ save: onSave }),
    handleCreate: () => store.trigger.createRequested({ body: CREATE_BODIES[kind], save: onSave }),
    handleRestore: (revisionId: string) =>
      store.trigger.restoreRequested({ restore: onRestore, revisionId }),
    handleReload: replace,
    payload: editor.payload,
    isMissing:
      isAuthoredFileMissing(editor.payload) && editor.draft === "" && editor.baseline.body === "",
    status: editor.validationStatus ?? editor.payload?.validation_status ?? "valid",
    revisions,
  };
}
