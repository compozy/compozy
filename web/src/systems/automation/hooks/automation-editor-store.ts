import { createStoreLogic } from "@xstate/store";

import { notifyUser } from "@/lib/user-feedback";

export interface WorkspaceEditorInput {
  workspaceId: string | null | undefined;
}

export interface WorkspaceEditorValue {
  mode: "create" | "edit";
}

export interface EditorRequestFence {
  generation: number;
  requestId: number;
  workspaceId: string | null | undefined;
}

export interface WorkspaceEditorFailure {
  submitError: string;
  toastError: string;
}

export interface WorkspaceEditorContext<T> extends WorkspaceEditorInput {
  editor: T | null;
  generation: number;
  pendingRequest: EditorRequestFence | null;
  requestCount: number;
  submitError: string | null;
}

type WorkspaceEditorEvents<T, TResult> = {
  draftChanged: { draft: T };
  editorClosed: {};
  editorOpened: { editor: T; workspaceId: string | null | undefined };
  lifecycleDisposed: {};
  submissionFailed: EditorRequestFence & { failure: WorkspaceEditorFailure };
  submissionRequested: {
    describeFailure: (error: unknown) => WorkspaceEditorFailure;
    execute: () => Promise<TResult>;
    successMessage: (mode: WorkspaceEditorValue["mode"], result: TResult) => string;
    workspaceId: string | null | undefined;
  };
  submissionSucceeded: EditorRequestFence & {
    mode: WorkspaceEditorValue["mode"];
    result: TResult;
    successMessage: string;
  };
};

type WorkspaceEditorEmitted<TResult> = {
  saveSucceeded: { result: TResult };
};

export function hasEditorRequestFence(
  request: EditorRequestFence | null,
  event: EditorRequestFence
): request is EditorRequestFence {
  return (
    request?.generation === event.generation &&
    request.requestId === event.requestId &&
    request.workspaceId === event.workspaceId
  );
}

export function createWorkspaceEditorLogic<T extends WorkspaceEditorValue, TResult>() {
  return createStoreLogic<
    WorkspaceEditorContext<T>,
    WorkspaceEditorEvents<T, TResult>,
    WorkspaceEditorEmitted<TResult>,
    WorkspaceEditorInput
  >({
    context: input => ({
      editor: null,
      generation: 0,
      pendingRequest: null,
      requestCount: 0,
      submitError: null,
      workspaceId: input.workspaceId,
    }),
    on: {
      draftChanged: (context, event) => {
        if (!context.editor || context.pendingRequest) return;
        return { ...context, editor: event.draft, submitError: null };
      },
      editorClosed: context => invalidateEditor(context),
      lifecycleDisposed: context => invalidateEditor(context),
      editorOpened: (context, event) => ({
        ...context,
        editor: event.editor,
        generation: context.generation + 1,
        pendingRequest: null,
        submitError: null,
        workspaceId: event.workspaceId,
      }),
      submissionRequested: (context, event, enqueue) => {
        if (
          !context.editor ||
          context.pendingRequest ||
          context.workspaceId !== event.workspaceId
        ) {
          return;
        }
        const requestId = context.requestCount + 1;
        const request: EditorRequestFence = {
          generation: context.generation,
          requestId,
          workspaceId: event.workspaceId,
        };
        const mode = context.editor.mode;
        enqueue.effect(async ({ trigger }) => {
          try {
            const result = await event.execute();
            trigger.submissionSucceeded({
              ...request,
              mode,
              result,
              successMessage: event.successMessage(mode, result),
            });
          } catch (error) {
            trigger.submissionFailed({ ...request, failure: event.describeFailure(error) });
          }
        });
        return {
          ...context,
          pendingRequest: request,
          requestCount: requestId,
          submitError: null,
        };
      },
      submissionFailed: (context, event, enqueue) => {
        if (!hasEditorRequestFence(context.pendingRequest, event)) return;
        enqueue.effect(() => notifyUser({ message: event.failure.toastError, tone: "error" }));
        return { ...context, pendingRequest: null, submitError: event.failure.submitError };
      },
      submissionSucceeded: (context, event, enqueue) => {
        if (!hasEditorRequestFence(context.pendingRequest, event)) return;
        enqueue.effect(() => notifyUser({ message: event.successMessage, tone: "success" }));
        enqueue.emit.saveSucceeded({ result: event.result });
        return invalidateEditor(context);
      },
    },
  });
}

function invalidateEditor<T>(context: WorkspaceEditorContext<T>): WorkspaceEditorContext<T> {
  return {
    ...context,
    editor: null,
    generation: context.generation + 1,
    pendingRequest: null,
    submitError: null,
  };
}
