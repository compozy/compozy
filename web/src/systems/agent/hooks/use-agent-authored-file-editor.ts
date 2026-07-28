import { useState, type ReactNode, type SetStateAction } from "react";
import { toast } from "sonner";

import { isAgentDigestConflict } from "../adapters/agent-api";
import {
  serializeAgentHeartbeatSource,
  serializeAgentSoulSource,
} from "../lib/agent-authored-file-source";
import type {
  AgentHeartbeatHistoryResponse,
  AgentHeartbeatPayload,
  AgentSoulHistoryResponse,
  AgentSoulPayload,
} from "../types";
import { useUnsavedGuard } from "./use-unsaved-guard";

export type AuthoredFileKind = "soul" | "heartbeat";

export type AuthoredFilePayload = AgentSoulPayload | AgentHeartbeatPayload;

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

const CONFLICT_MESSAGE = "This file changed elsewhere. Reload and retry.";

interface LocalBaseline {
  digest: string;
  body: string;
}

interface AuthoredFileEditorState {
  baseline: LocalBaseline;
  conflict: boolean;
  diagnostics: Array<{ message: string; line?: number; source_path?: string }>;
  draft: string;
  payload: AuthoredFilePayload | undefined;
  resourceKey: string;
  saveError: string | null;
  showHistory: boolean;
  sourcePayload: AuthoredFilePayload | undefined;
  validationStatus: string | undefined;
}

/** Missing only for an explicit successful domain payload — never unknown/undefined. */
export function isAuthoredFileMissing(payload: AuthoredFilePayload | undefined): boolean {
  if (!payload) return false;
  return payload.validation_status === "missing" || payload.present === false;
}

export function buildAuthoredFileResourceKey(
  workspaceId: string | null,
  agentName: string,
  kind: AuthoredFileKind
): string {
  return JSON.stringify([workspaceId, agentName, kind]);
}

function readBody(kind: AuthoredFileKind, payload: AuthoredFilePayload): string {
  if (kind === "soul") {
    return serializeAgentSoulSource(payload as AgentSoulPayload);
  }
  return serializeAgentHeartbeatSource(payload as AgentHeartbeatPayload);
}

function readDigest(payload: AuthoredFilePayload): string {
  return payload.digest ?? "";
}

function seedFromPayload(
  kind: AuthoredFileKind,
  payload: AuthoredFilePayload | undefined
): {
  draft: string;
  baseline: LocalBaseline;
  diagnostics: Array<{ message: string; line?: number; source_path?: string }>;
} {
  if (!payload) {
    return { draft: "", baseline: { digest: "", body: "" }, diagnostics: [] };
  }
  if (isAuthoredFileMissing(payload)) {
    return {
      draft: "",
      baseline: { digest: readDigest(payload), body: "" },
      diagnostics: [],
    };
  }
  const body = readBody(kind, payload);
  return {
    draft: body,
    baseline: { digest: readDigest(payload), body },
    diagnostics: payload.diagnostics ?? [],
  };
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
  const initial = seedFromPayload(kind, payload);
  const [storedEditor, setStoredEditor] = useState<AuthoredFileEditorState>(() => ({
    baseline: initial.baseline,
    conflict: false,
    diagnostics: initial.diagnostics,
    draft: initial.draft,
    payload,
    resourceKey,
    saveError: null as string | null,
    showHistory: false,
    sourcePayload: payload,
    validationStatus: payload?.validation_status,
  }));
  const shouldAdoptPayload =
    storedEditor.resourceKey !== resourceKey ||
    (storedEditor.draft === storedEditor.baseline.body && storedEditor.sourcePayload !== payload);
  const editor = shouldAdoptPayload
    ? {
        baseline: initial.baseline,
        conflict: false,
        diagnostics: initial.diagnostics,
        draft: initial.draft,
        payload,
        resourceKey,
        saveError: null,
        showHistory: false,
        sourcePayload: payload,
        validationStatus: payload?.validation_status,
      }
    : storedEditor;
  if (editor !== storedEditor) {
    setStoredEditor(editor);
  }
  const [validating, setValidating] = useState(false);
  const [saving, setSaving] = useState(false);
  const { baseline, conflict, diagnostics, draft, saveError, showHistory } = editor;

  const applyAuthoritativePayload = (authoritative: AuthoredFilePayload | undefined) => {
    const next = seedFromPayload(kind, authoritative);
    setStoredEditor({
      baseline: next.baseline,
      conflict: false,
      diagnostics: next.diagnostics,
      draft: next.draft,
      payload: authoritative,
      resourceKey,
      saveError: null,
      showHistory: false,
      sourcePayload: payload,
      validationStatus: authoritative?.validation_status,
    });
  };

  const dirty = draft !== baseline.body;

  const guard = useUnsavedGuard({ dirty, entityName: fileLabel });

  const applyWriteError = (err: unknown, fallback: string) => {
    if (isAgentDigestConflict(err)) {
      setStoredEditor(current => ({ ...current, conflict: true, saveError: CONFLICT_MESSAGE }));
      return;
    }
    setStoredEditor(current => ({
      ...current,
      conflict: false,
      saveError: err instanceof Error ? err.message : fallback,
    }));
  };

  const handleReload = () => {
    applyAuthoritativePayload(payload);
  };

  const handleValidate = async () => {
    setValidating(true);
    setStoredEditor(current => ({ ...current, conflict: false, saveError: null }));
    try {
      const result = await onValidate(draft);
      setStoredEditor(current => ({
        ...current,
        diagnostics: result.diagnostics ?? [],
        validationStatus: result.validation_status ?? current.validationStatus,
      }));
    } catch (err) {
      setStoredEditor(current => ({
        ...current,
        saveError: err instanceof Error ? err.message : `Couldn't validate ${fileLabel}`,
      }));
    } finally {
      setValidating(false);
    }
  };

  const handleSave = async () => {
    setSaving(true);
    setStoredEditor(current => ({ ...current, conflict: false, saveError: null }));
    try {
      const authoritative = await onSave(draft, baseline.digest);
      applyAuthoritativePayload(authoritative);
      toast.success(`${fileLabel} saved`);
    } catch (err) {
      applyWriteError(err, `Couldn't save ${fileLabel}`);
    } finally {
      setSaving(false);
    }
  };

  const handleCreate = async () => {
    const body = CREATE_BODIES[kind];
    setSaving(true);
    setStoredEditor(current => ({ ...current, conflict: false, saveError: null }));
    try {
      const authoritative = await onSave(body, baseline.digest);
      applyAuthoritativePayload(authoritative);
      toast.success(`${fileLabel} created`);
    } catch (err) {
      applyWriteError(err, `Couldn't create ${fileLabel}`);
    } finally {
      setSaving(false);
    }
  };

  const handleRestore = async (revisionId: string) => {
    setSaving(true);
    setStoredEditor(current => ({ ...current, conflict: false, saveError: null }));
    try {
      const authoritative = await onRestore(revisionId, baseline.digest);
      applyAuthoritativePayload(authoritative);
      toast.success(`${fileLabel} restored`);
    } catch (err) {
      applyWriteError(err, `Couldn't restore ${fileLabel}`);
    } finally {
      setSaving(false);
    }
  };

  const revisions = history && "revisions" in history ? (history.revisions ?? []) : [];

  return {
    fileLabel,
    draft,
    setDraft: (nextDraft: string) => setStoredEditor(current => ({ ...current, draft: nextDraft })),
    diagnostics,
    validating,
    saving,
    saveError,
    conflict,
    showHistory,
    setShowHistory: (update: SetStateAction<boolean>) =>
      setStoredEditor(current => ({
        ...current,
        showHistory: typeof update === "function" ? update(current.showHistory) : update,
      })),
    dirty,
    guardDialog: guard.confirmDialog as ReactNode,
    handleValidate,
    handleSave,
    handleCreate,
    handleRestore,
    handleReload,
    payload: editor.payload,
    isMissing: isAuthoredFileMissing(editor.payload) && draft === "" && baseline.body === "",
    status: editor.validationStatus ?? editor.payload?.validation_status ?? "valid",
    revisions,
  };
}
