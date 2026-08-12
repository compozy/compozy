import { createStoreLogic } from "@xstate/store";

import type { SandboxDraft } from "../lib/sandbox-profile-draft";
import type { SettingsMutationResult, SettingsSandboxEntry } from "@/systems/settings";

export type { SandboxDraft } from "../lib/sandbox-profile-draft";

export type SandboxEditorState =
  | { mode: "closed" }
  | { mode: "create"; draft: SandboxDraft }
  | { mode: "edit"; name: string; draft: SandboxDraft; entry: SettingsSandboxEntry };

export type SandboxDeleteState = { mode: "closed" } | { mode: "open"; entry: SettingsSandboxEntry };

export type SandboxLastAction =
  | { kind: "saved"; name: string; result: SettingsMutationResult }
  | {
      kind: "deleted";
      name: string;
      result: SettingsMutationResult;
      usageCount: number;
    };

type SandboxRequestKind = "delete" | "save";

interface SandboxRequestFence {
  generation: number;
  kind: SandboxRequestKind;
  name: string;
  requestId: number;
}

interface SandboxPageContext {
  deleteTarget: SandboxDeleteState;
  editor: SandboxEditorState;
  generation: number;
  lastAction: SandboxLastAction | null;
  pendingRequest: SandboxRequestFence | null;
  requestCount: number;
  selectedName: string | null;
}

type SandboxPageEvents = {
  deleteClosed: {};
  deleteOpened: { entry: SettingsSandboxEntry };
  deleteRequested: { execute: (name: string) => Promise<SettingsMutationResult> };
  deleteSucceeded: SandboxRequestFence & { result: SettingsMutationResult; usageCount: number };
  draftChanged: { draft: SandboxDraft };
  editorClosed: {};
  editorOpened: { editor: Exclude<SandboxEditorState, { mode: "closed" }> };
  lastActionDismissed: {};
  profileInspectClosed: {};
  profileInspectOpened: { name: string };
  requestFailed: SandboxRequestFence;
  saveRequested: { execute: () => Promise<SettingsMutationResult>; name: string };
  saveSucceeded: SandboxRequestFence & { result: SettingsMutationResult };
};

function initialContext(): SandboxPageContext {
  return {
    deleteTarget: { mode: "closed" },
    editor: { mode: "closed" },
    generation: 0,
    lastAction: null,
    pendingRequest: null,
    requestCount: 0,
    selectedName: null,
  };
}

function hasSandboxRequestFence(
  request: SandboxRequestFence | null,
  event: SandboxRequestFence
): request is SandboxRequestFence {
  return (
    request?.generation === event.generation &&
    request.kind === event.kind &&
    request.name === event.name &&
    request.requestId === event.requestId
  );
}

export const sandboxPageLogic = createStoreLogic<SandboxPageContext, SandboxPageEvents>({
  context: initialContext(),
  on: {
    deleteClosed: context =>
      context.pendingRequest
        ? undefined
        : {
            ...context,
            deleteTarget: { mode: "closed" },
            generation: context.generation + 1,
          },
    deleteOpened: (context, event) =>
      context.pendingRequest
        ? undefined
        : {
            ...context,
            deleteTarget: { entry: event.entry, mode: "open" },
            generation: context.generation + 1,
            selectedName: null,
          },
    deleteRequested: (context, event, enqueue) => {
      if (context.deleteTarget.mode !== "open" || context.pendingRequest) return;
      const requestId = context.requestCount + 1;
      const request: SandboxRequestFence = {
        generation: context.generation,
        kind: "delete",
        name: context.deleteTarget.entry.name,
        requestId,
      };
      const usageCount = context.deleteTarget.entry.workspace_usage_count;
      enqueue.effect(async ({ trigger }) => {
        try {
          trigger.deleteSucceeded({
            ...request,
            result: await event.execute(request.name),
            usageCount,
          });
        } catch {
          trigger.requestFailed(request);
        }
      });
      return {
        ...context,
        pendingRequest: request,
        requestCount: requestId,
      };
    },
    deleteSucceeded: (context, event) => {
      if (!hasSandboxRequestFence(context.pendingRequest, event)) return;
      return {
        ...context,
        deleteTarget: { mode: "closed" },
        generation: context.generation + 1,
        lastAction: {
          kind: "deleted",
          name: event.name,
          result: event.result,
          usageCount: event.usageCount,
        },
        pendingRequest: null,
      };
    },
    draftChanged: (context, event) => {
      if (context.editor.mode === "closed" || context.pendingRequest) return;
      return { ...context, editor: { ...context.editor, draft: event.draft } };
    },
    editorClosed: context =>
      context.pendingRequest
        ? undefined
        : {
            ...context,
            editor: { mode: "closed" },
            generation: context.generation + 1,
          },
    editorOpened: (context, event) =>
      context.pendingRequest
        ? undefined
        : {
            ...context,
            editor: event.editor,
            generation: context.generation + 1,
            selectedName: null,
          },
    lastActionDismissed: context => ({ ...context, lastAction: null }),
    profileInspectClosed: context =>
      context.pendingRequest ? undefined : { ...context, selectedName: null },
    profileInspectOpened: (context, event) =>
      context.pendingRequest ? undefined : { ...context, selectedName: event.name },
    requestFailed: (context, event) => {
      if (!hasSandboxRequestFence(context.pendingRequest, event)) return;
      return { ...context, pendingRequest: null };
    },
    saveRequested: (context, event, enqueue) => {
      if (context.editor.mode === "closed" || context.pendingRequest) return;
      const requestId = context.requestCount + 1;
      const request: SandboxRequestFence = {
        generation: context.generation,
        kind: "save",
        name: event.name,
        requestId,
      };
      enqueue.effect(async ({ trigger }) => {
        try {
          trigger.saveSucceeded({ ...request, result: await event.execute() });
        } catch {
          trigger.requestFailed(request);
        }
      });
      return {
        ...context,
        pendingRequest: request,
        requestCount: requestId,
      };
    },
    saveSucceeded: (context, event) => {
      if (!hasSandboxRequestFence(context.pendingRequest, event)) return;
      return {
        ...context,
        editor: { mode: "closed" },
        generation: context.generation + 1,
        lastAction: { kind: "saved", name: event.name, result: event.result },
        pendingRequest: null,
      };
    },
  },
});
