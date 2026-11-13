import { useSelector } from "@xstate/store-react";
import { type ReactNode, type SetStateAction } from "react";

import { useStoreBinding } from "@/hooks/use-store-binding";

import {
  createAuthoredFileEditorLogic,
  idleAuthoredFileEditor,
  isAuthoredFileEditorDirty,
  isAuthoredFileMissing,
  shouldAdoptAuthoredFileSource,
  type AuthoredFileEditorInput,
  type AuthoredFileEditorState,
  type AuthoredFileKind,
  type AuthoredFilePayload,
  type AuthoredFileRestore,
  type AuthoredFileSave,
  type AuthoredFileValidate,
} from "../stores/agent-authored-file-editor-store";
import type { AgentHeartbeatHistoryResponse, AgentSoulHistoryResponse } from "../types";
import { retainedDraftKey, retainedEditorDraft } from "../lib/retained-editor-drafts";
import { type RetainedDraftPolicy, useRetainedEditorDraft } from "./use-retained-editor-draft";
import { useUnsavedGuard } from "./use-unsaved-guard";

export type { AuthoredFileKind, AuthoredFilePayload };
export { isAuthoredFileMissing };

interface AuthoredFileEditorCallbacks<K extends AuthoredFileKind> {
  onValidate: AuthoredFileValidate;
  onSave: AuthoredFileSave<K>;
  onRestore: AuthoredFileRestore<K>;
}

export type UseAgentAuthoredFileEditorArgs =
  | (AuthoredFileEditorInput<"soul"> &
      AuthoredFileEditorCallbacks<"soul"> & {
        history: AgentSoulHistoryResponse | undefined;
      })
  | (AuthoredFileEditorInput<"heartbeat"> &
      AuthoredFileEditorCallbacks<"heartbeat"> & {
        history: AgentHeartbeatHistoryResponse | undefined;
      });

const AUTHORED_FILE_DRAFT_POLICY: RetainedDraftPolicy<AuthoredFileEditorState> = {
  isDirty: isAuthoredFileEditorDirty,
  isBusy: context => context.phase === "saving" || context.phase === "validating",
  idle: idleAuthoredFileEditor,
};

const CREATE_BODIES: Record<AuthoredFileKind, string> = {
  soul: "# Soul\n\nDefine persona and constraints.\n",
  heartbeat: "---\nenabled: true\n---\n\nCheck for pending work.\n",
};

export function buildAuthoredFileResourceKey(
  workspaceId: string | null,
  agentName: string,
  kind: AuthoredFileKind,
  profile = "default"
): string {
  return JSON.stringify([workspaceId, agentName, kind, profile]);
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
  const input: AuthoredFileEditorInput =
    kind === "soul"
      ? { resourceKey, kind: "soul", payload }
      : { resourceKey, kind: "heartbeat", payload };
  const draftKey = retainedDraftKey("authored-file", resourceKey);
  // Edits kept from an editor of this file that went away unsaved come back as unsaved (#1653);
  // a store replaced under the same key (a reload, following the server) does not resume them.
  const resume = () =>
    createAuthoredFileEditorLogic(
      retainedEditorDraft<AuthoredFileEditorState>(draftKey)
    ).createStore(input);
  const { replace, store } = useStoreBinding(
    resourceKey,
    resume,
    (_previous, previousKey) =>
      previousKey === resourceKey ? createAuthoredFileEditorLogic().createStore(input) : resume(),
    (current, nextResourceKey) =>
      current.key !== nextResourceKey ||
      shouldAdoptAuthoredFileSource(current.store.getSnapshot().context, input)
  );
  const { forget } = useRetainedEditorDraft(draftKey, store, AUTHORED_FILE_DRAFT_POLICY);
  const editor = useSelector(store, snapshot => snapshot.context);

  const dirty = isAuthoredFileEditorDirty(editor);
  const guard = useUnsavedGuard({
    dirty,
    entityName: fileLabel,
    onDiscard: () => {
      store.trigger.discarded();
      forget();
    },
    resourceKey,
  });
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
