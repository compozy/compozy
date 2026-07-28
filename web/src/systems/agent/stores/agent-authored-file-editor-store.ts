import { createStoreLogic, type EnqueueObject } from "@xstate/store";

import { notifyUser } from "@/lib/user-feedback";

import { isAgentDigestConflict } from "../adapters/agent-api";
import {
  serializeAgentHeartbeatSource,
  serializeAgentSoulSource,
} from "../lib/agent-authored-file-source";
import type { AgentHeartbeatPayload, AgentSoulPayload } from "../types";

export type AuthoredFileKind = "soul" | "heartbeat";
type AuthoredFilePayloadByKind = {
  soul: AgentSoulPayload;
  heartbeat: AgentHeartbeatPayload;
};

export type AuthoredFilePayload<K extends AuthoredFileKind = AuthoredFileKind> =
  AuthoredFilePayloadByKind[K];

export interface AuthoredFileDiagnostic {
  message: string;
  line?: number;
  source_path?: string;
}

export interface AuthoredFileValidationResult {
  diagnostics?: AuthoredFileDiagnostic[];
  validation_status?: string;
}

export type AuthoredFileValidate = (body: string) => Promise<AuthoredFileValidationResult>;
export type AuthoredFileSave<K extends AuthoredFileKind = AuthoredFileKind> = (
  body: string,
  expectedDigest: string
) => Promise<AuthoredFilePayload<K>>;
export type AuthoredFileRestore<K extends AuthoredFileKind = AuthoredFileKind> = (
  revisionId: string,
  expectedDigest: string
) => Promise<AuthoredFilePayload<K>>;

interface LocalBaseline {
  digest: string;
  body: string;
}

interface AuthoredFileEditorCommon {
  resourceKey: string;
  observedSourceFingerprint: string;
  requestId: number;
  baseline: LocalBaseline;
  draft: string;
  diagnostics: AuthoredFileDiagnostic[];
  validationStatus: string | undefined;
  showHistory: boolean;
}

type AuthoredFileEditorBase = AuthoredFileEditorCommon & AuthoredFileEditorInput;

type AuthoredFileEditorReady = AuthoredFileEditorBase & { phase: "ready" };

type AuthoredFileEditorValidating = AuthoredFileEditorBase & { phase: "validating" };

type AuthoredFileEditorSaving = AuthoredFileEditorBase & {
  phase: "saving";
  operation: "save" | "create" | "restore";
};

type AuthoredFileEditorConflict = AuthoredFileEditorBase & { phase: "conflict"; error: string };

type AuthoredFileEditorFailed = AuthoredFileEditorBase & {
  phase: "failed";
  operation: "validate" | "save" | "create" | "restore";
  error: string;
};

export type AuthoredFileEditorState =
  | AuthoredFileEditorReady
  | AuthoredFileEditorValidating
  | AuthoredFileEditorSaving
  | AuthoredFileEditorConflict
  | AuthoredFileEditorFailed;

export type AuthoredFileEditorInput<K extends AuthoredFileKind = AuthoredFileKind> =
  K extends AuthoredFileKind
    ? {
        resourceKey: string;
        kind: K;
        payload: AuthoredFilePayload<K> | undefined;
      }
    : never;

type AuthoredFileEditorEventPayloadMap = {
  draftChanged: { draft: string };
  historyVisibilityChanged: { visible: boolean };
  validationRequested: { validate: AuthoredFileValidate };
  saveRequested: { save: AuthoredFileSave };
  createRequested: { body: string; save: AuthoredFileSave };
  restoreRequested: { restore: AuthoredFileRestore; revisionId: string };
  validationSucceeded: {
    resourceKey: string;
    requestId: number;
    diagnostics: AuthoredFileDiagnostic[];
    validationStatus: string | undefined;
  };
  validationFailed: { resourceKey: string; requestId: number; error: string };
  writeSucceeded: {
    resourceKey: string;
    requestId: number;
    expectedDigest: string;
    payload: AuthoredFilePayload;
  };
  writeFailed: {
    resourceKey: string;
    requestId: number;
    expectedDigest: string;
    conflict: boolean;
    error: string;
    operation: "save" | "create" | "restore";
  };
};

type AuthoredFileEditorEnqueue = EnqueueObject<
  AuthoredFileEditorState,
  never,
  AuthoredFileEditorEventPayloadMap
>;

interface AuthoredFileEditorTrigger {
  writeSucceeded: (event: {
    resourceKey: string;
    requestId: number;
    expectedDigest: string;
    payload: AuthoredFilePayload;
  }) => void;
  writeFailed: (event: {
    resourceKey: string;
    requestId: number;
    expectedDigest: string;
    conflict: boolean;
    error: string;
    operation: "save" | "create" | "restore";
  }) => void;
}

export function createAuthoredFileEditorLogic() {
  return createStoreLogic<
    AuthoredFileEditorState,
    AuthoredFileEditorEventPayloadMap,
    Record<never, never>,
    AuthoredFileEditorInput
  >({
    context: input => seedState(input, 0),
    on: {
      draftChanged: (context, event) => {
        if (context.phase === "saving") return;
        if (context.phase === "conflict") return { ...context, draft: event.draft };
        return { ...toReady(context), draft: event.draft };
      },
      historyVisibilityChanged: (context, event) => ({ ...context, showHistory: event.visible }),
      validationRequested: (context, event, enqueue) => {
        if (context.phase === "saving" || context.phase === "validating") return;
        const requestId = context.requestId + 1;
        const resourceKey = context.resourceKey;
        const body = context.draft;
        enqueue.effect(async ({ trigger }) => {
          try {
            const result = await event.validate(body);
            trigger.validationSucceeded({
              resourceKey,
              requestId,
              diagnostics: result.diagnostics ?? [],
              validationStatus: result.validation_status,
            });
          } catch (error) {
            trigger.validationFailed({
              resourceKey,
              requestId,
              error: errorMessage(error, operationFailureFallback(context.kind, "validate")),
            });
          }
        });
        return { ...toReady(context), phase: "validating", requestId };
      },
      saveRequested: (context, event, enqueue) => {
        if (
          context.phase === "saving" ||
          context.phase === "validating" ||
          context.draft === context.baseline.body
        )
          return;
        return startWrite(context, "save", context.draft, event.save, enqueue);
      },
      createRequested: (context, event, enqueue) => {
        if (context.phase === "saving" || context.phase === "validating") return;
        return startWrite(context, "create", event.body, event.save, enqueue);
      },
      restoreRequested: (context, event, enqueue) => {
        if (context.phase === "saving" || context.phase === "validating" || event.revisionId === "")
          return;
        const requestId = context.requestId + 1;
        const resourceKey = context.resourceKey;
        const expectedDigest = context.baseline.digest;
        const revisionId = event.revisionId;
        enqueue.effect(async ({ trigger }) => {
          try {
            trigger.writeSucceeded({
              resourceKey,
              requestId,
              expectedDigest,
              payload: await event.restore(revisionId, expectedDigest),
            });
          } catch (error) {
            triggerWriteFailure(
              trigger,
              error,
              resourceKey,
              requestId,
              expectedDigest,
              context.kind,
              "restore"
            );
          }
        });
        return { ...toReady(context), phase: "saving", operation: "restore", requestId };
      },
      validationSucceeded: (context, event) => {
        if (!matchesRequest(context, event.resourceKey, event.requestId, "validating")) return;
        return {
          ...context,
          phase: "ready",
          diagnostics: event.diagnostics,
          validationStatus: event.validationStatus ?? context.validationStatus,
        };
      },
      validationFailed: (context, event) => {
        if (!matchesRequest(context, event.resourceKey, event.requestId, "validating")) return;
        return { ...context, phase: "failed", operation: "validate", error: event.error };
      },
      writeSucceeded: (context, event, enqueue) => {
        if (
          context.phase !== "saving" ||
          context.resourceKey !== event.resourceKey ||
          context.requestId !== event.requestId ||
          context.baseline.digest !== event.expectedDigest
        ) {
          return;
        }
        const action =
          context.operation === "save"
            ? "saved"
            : context.operation === "create"
              ? "created"
              : "restored";
        enqueue.effect(() =>
          notifyUser({
            message: `${authoredFileLabel(context.kind)} ${action}`,
            tone: "success",
          })
        );
        return seedState(
          toAuthoredFileEditorInput(context.resourceKey, context.kind, event.payload),
          context.requestId,
          context.observedSourceFingerprint
        );
      },
      writeFailed: (context, event) => {
        if (
          context.phase !== "saving" ||
          context.resourceKey !== event.resourceKey ||
          context.requestId !== event.requestId ||
          context.baseline.digest !== event.expectedDigest ||
          context.operation !== event.operation
        ) {
          return;
        }
        if (event.conflict) return { ...context, phase: "conflict", error: event.error };
        return { ...context, phase: "failed", operation: event.operation, error: event.error };
      },
    },
  });
}

function startWrite(
  context: AuthoredFileEditorState,
  operation: "save" | "create",
  body: string,
  save: AuthoredFileSave,
  enqueue: AuthoredFileEditorEnqueue
): AuthoredFileEditorSaving {
  const requestId = context.requestId + 1;
  const resourceKey = context.resourceKey;
  const expectedDigest = context.baseline.digest;
  enqueue.effect(async ({ trigger }) => {
    try {
      trigger.writeSucceeded({
        resourceKey,
        requestId,
        expectedDigest,
        payload: await save(body, expectedDigest),
      });
    } catch (error) {
      triggerWriteFailure(
        trigger,
        error,
        resourceKey,
        requestId,
        expectedDigest,
        context.kind,
        operation
      );
    }
  });
  return { ...toReady(context), phase: "saving", operation, requestId };
}

function triggerWriteFailure(
  trigger: AuthoredFileEditorTrigger,
  error: unknown,
  resourceKey: string,
  requestId: number,
  expectedDigest: string,
  kind: AuthoredFileKind,
  operation: "save" | "create" | "restore"
): void {
  const conflict = isAgentDigestConflict(error);
  trigger.writeFailed({
    resourceKey,
    requestId,
    expectedDigest,
    operation,
    conflict,
    error: conflict
      ? "This file changed elsewhere. Reload and retry."
      : errorMessage(error, operationFailureFallback(kind, operation)),
  });
}

function matchesRequest(
  context: AuthoredFileEditorState,
  resourceKey: string,
  requestId: number,
  phase: "validating"
): boolean {
  return (
    context.phase === phase &&
    context.resourceKey === resourceKey &&
    context.requestId === requestId
  );
}

function seedState(
  input: AuthoredFileEditorInput,
  requestId: number,
  observedSourceFingerprint = sourceFingerprint(input.payload)
): AuthoredFileEditorReady {
  const seeded = seedFromPayload(input);
  return {
    ...input,
    phase: "ready",
    observedSourceFingerprint,
    requestId,
    baseline: seeded.baseline,
    draft: seeded.draft,
    diagnostics: seeded.diagnostics,
    validationStatus: input.payload?.validation_status,
    showHistory: false,
  };
}

function toAuthoredFileEditorInput(
  resourceKey: string,
  kind: AuthoredFileKind,
  payload: AuthoredFilePayload
): AuthoredFileEditorInput {
  if (kind === "soul" && !isAgentHeartbeatPayload(payload)) {
    return { resourceKey, kind, payload };
  }
  if (kind === "heartbeat" && isAgentHeartbeatPayload(payload)) {
    return { resourceKey, kind, payload };
  }
  throw new TypeError("Authored-file response kind did not match the editor kind.");
}

function isAgentHeartbeatPayload(payload: AuthoredFilePayload): payload is AgentHeartbeatPayload {
  return "preferences" in payload;
}

export function shouldAdoptAuthoredFileSource(
  context: AuthoredFileEditorState,
  input: AuthoredFileEditorInput
): boolean {
  if (context.resourceKey !== input.resourceKey || context.kind !== input.kind) return true;
  if (!input.payload || context.phase === "saving" || context.draft !== context.baseline.body)
    return false;
  return context.observedSourceFingerprint !== sourceFingerprint(input.payload);
}

function sourceFingerprint(payload: AuthoredFilePayload | undefined): string {
  return JSON.stringify([
    payload?.digest ?? null,
    payload?.present ?? null,
    payload?.validation_status ?? null,
    payload?.diagnostics ?? [],
  ]);
}

function operationFailureFallback(
  kind: AuthoredFileKind,
  operation: "validate" | "save" | "create" | "restore"
): string {
  return `Couldn't ${operation} ${authoredFileLabel(kind)}`;
}

function authoredFileLabel(kind: AuthoredFileKind): string {
  return kind === "soul" ? "SOUL.md" : "HEARTBEAT.md";
}

function seedFromPayload(input: AuthoredFileEditorInput) {
  return input.kind === "soul"
    ? seedSoulPayload(input.payload)
    : seedHeartbeatPayload(input.payload);
}

function seedSoulPayload(payload: AgentSoulPayload | undefined) {
  if (!payload) return { draft: "", baseline: { digest: "", body: "" }, diagnostics: [] };
  if (isAuthoredFileMissing(payload)) {
    return { draft: "", baseline: { digest: readDigest(payload), body: "" }, diagnostics: [] };
  }
  const body = serializeAgentSoulSource(payload);
  return {
    draft: body,
    baseline: { digest: readDigest(payload), body },
    diagnostics: payload.diagnostics ?? [],
  };
}

function seedHeartbeatPayload(payload: AgentHeartbeatPayload | undefined) {
  if (!payload) return { draft: "", baseline: { digest: "", body: "" }, diagnostics: [] };
  if (isAuthoredFileMissing(payload)) {
    return { draft: "", baseline: { digest: readDigest(payload), body: "" }, diagnostics: [] };
  }
  const body = serializeAgentHeartbeatSource(payload);
  return {
    draft: body,
    baseline: { digest: readDigest(payload), body },
    diagnostics: payload.diagnostics ?? [],
  };
}

function toReady(context: AuthoredFileEditorState): AuthoredFileEditorReady {
  const {
    phase: _phase,
    error: _error,
    operation: _operation,
    ...ready
  } = context as AuthoredFileEditorState & {
    error?: string;
    operation?: "validate" | "save" | "create" | "restore";
  };
  return { ...ready, phase: "ready" };
}

function readDigest(payload: AuthoredFilePayload): string {
  return payload.digest ?? "";
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message.trim() !== "" ? error.message : fallback;
}

export function isAuthoredFileMissing(payload: AuthoredFilePayload | undefined): boolean {
  return Boolean(payload && (payload.validation_status === "missing" || payload.present === false));
}
