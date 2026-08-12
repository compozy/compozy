import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const routeHookMocks = vi.hoisted(() => ({
  auiState: { thread: { isRunning: false, messages: [] as Array<{ id: string }> } },
  transcriptMessages: [] as Array<{ id: string }>,
  resetThread: vi.fn(),
  toastError: vi.fn(),
  toastSuccess: vi.fn(),
  cancelSessionPrompt: vi.fn(),
  clearMutation: { isPending: false, mutate: vi.fn() },
  deleteMutation: { isPending: false, mutate: vi.fn() },
  renameMutation: { isPending: false, mutateAsync: vi.fn() },
  resumeMutation: { isPending: false, mutateAsync: vi.fn() },
  unarchiveMutation: { isPending: false, mutate: vi.fn() },
  queuePromptMutation: { isPending: false, mutateAsync: vi.fn() },
  interruptPromptMutation: { isPending: false, mutateAsync: vi.fn() },
  steerPromptMutation: { isPending: false, mutateAsync: vi.fn() },
  stopMutation: { isPending: false, mutate: vi.fn(), mutateAsync: vi.fn() },
  sessionInputsQuery: { data: { inputs: [] as Array<Record<string, unknown>> } },
  cancelInputMutation: { isPending: false, mutate: vi.fn() },
  replaceInputMutation: { isPending: false, mutateAsync: vi.fn() },
  promoteInputMutation: { isPending: false, mutate: vi.fn() },
}));

vi.mock("@assistant-ui/react", () => ({
  useAui: () => ({ thread: { reset: routeHookMocks.resetThread } }),
  useAuiState: (selector: (state: typeof routeHookMocks.auiState) => unknown) =>
    selector(routeHookMocks.auiState),
}));

vi.mock("sonner", () => ({
  toast: { error: routeHookMocks.toastError, success: routeHookMocks.toastSuccess },
}));

vi.mock("@/systems/session", async () => {
  const { canPromptSession } = await import("@/systems/session/lib/session-running");
  return {
    canPromptSession,
    cancelSessionPrompt: routeHookMocks.cancelSessionPrompt,
    isSessionRunning: (session: {
      state?: string;
      badge?: string;
      activity?: { turn_id?: string };
    }) =>
      session.state !== "stopped" &&
      (Boolean(session.activity?.turn_id) || session.badge === "running"),
    isUserControllableSession: (session: { type?: string }) => (session.type ?? "user") === "user",
    useCancelSessionInput: () => routeHookMocks.cancelInputMutation,
    useClearSessionConversation: () => routeHookMocks.clearMutation,
    useDeleteSession: () => routeHookMocks.deleteMutation,
    useInterruptSessionPrompt: () => routeHookMocks.interruptPromptMutation,
    usePromoteSessionInput: () => routeHookMocks.promoteInputMutation,
    useQueueSessionPrompt: () => routeHookMocks.queuePromptMutation,
    useReplaceSessionInput: () => routeHookMocks.replaceInputMutation,
    useRenameSession: () => routeHookMocks.renameMutation,
    useResumeSession: () => routeHookMocks.resumeMutation,
    useUnarchiveSession: () => routeHookMocks.unarchiveMutation,
    useSessionInputs: () => routeHookMocks.sessionInputsQuery,
    useSessionTranscriptThreadMessages: () => routeHookMocks.transcriptMessages,
    useSteerSessionPrompt: () => routeHookMocks.steerPromptMutation,
    useStopSession: () => routeHookMocks.stopMutation,
  };
});

import type { SessionPayload } from "@/systems/session";
import { useSessionPageControls } from "../use-session-page-controls";

const WORKSPACE_ID = "ws_alpha";

function makeSession(state: SessionPayload["state"], turnId?: string): SessionPayload {
  return {
    id: "sess-1",
    agent_name: "codex-agent",
    runtime: {
      status: state === "stopped" ? "unbound" : "ready",
      effective: { provider: "codex" },
      selection_revision: 0,
    },
    workspace_id: WORKSPACE_ID,
    workspace_path: "/workspace",
    state,
    badge: turnId ? "running" : state === "stopped" ? "stopped" : "idle",
    activity: turnId
      ? {
          elapsed_ms: 0,
          elapsed_seconds: 0,
          idle_seconds: 0,
          iteration_current: 0,
          iteration_max: 0,
          last_activity_at: "2026-04-17T10:00:00Z",
          turn_id: turnId,
        }
      : undefined,
    attachable: state !== "stopped",
    archived_at: null,
    available_commands: [],
    type: "user",
    created_at: "2026-04-17T10:00:00Z",
    updated_at: "2026-04-17T10:00:00Z",
  };
}

function renderControls(session = makeSession("active")) {
  return renderHook(() => useSessionPageControls("sess-1", session, { workspaceId: WORKSPACE_ID }));
}

function createDeferredPromise<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, reject, resolve };
}

describe("useSessionPageControls", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  beforeEach(() => {
    routeHookMocks.auiState.thread.isRunning = false;
    routeHookMocks.auiState.thread.messages = [];
    routeHookMocks.transcriptMessages = [];
    routeHookMocks.sessionInputsQuery.data = { inputs: [] };
    routeHookMocks.resetThread.mockReset();
    routeHookMocks.toastError.mockReset();
    routeHookMocks.toastSuccess.mockReset();
    routeHookMocks.cancelSessionPrompt.mockReset();
    for (const mutation of [
      routeHookMocks.clearMutation,
      routeHookMocks.deleteMutation,
      routeHookMocks.cancelInputMutation,
      routeHookMocks.promoteInputMutation,
    ]) {
      mutation.isPending = false;
      mutation.mutate.mockReset();
    }
    routeHookMocks.resumeMutation.isPending = false;
    routeHookMocks.resumeMutation.mutateAsync.mockReset();
    routeHookMocks.renameMutation.isPending = false;
    routeHookMocks.renameMutation.mutateAsync.mockReset();
    routeHookMocks.unarchiveMutation.isPending = false;
    routeHookMocks.unarchiveMutation.mutate.mockReset();
    routeHookMocks.queuePromptMutation.isPending = false;
    routeHookMocks.queuePromptMutation.mutateAsync.mockReset();
    routeHookMocks.interruptPromptMutation.isPending = false;
    routeHookMocks.interruptPromptMutation.mutateAsync.mockReset();
    routeHookMocks.steerPromptMutation.isPending = false;
    routeHookMocks.steerPromptMutation.mutateAsync.mockReset();
    routeHookMocks.replaceInputMutation.isPending = false;
    routeHookMocks.replaceInputMutation.mutateAsync.mockReset();
    routeHookMocks.stopMutation.isPending = false;
    routeHookMocks.stopMutation.mutate.mockReset();
    routeHookMocks.stopMutation.mutateAsync.mockReset();
  });

  it("Should serialize prompt cancellation and block destructive controls", async () => {
    const cancellation = createDeferredPromise<void>();
    routeHookMocks.auiState.thread.isRunning = true;
    routeHookMocks.cancelSessionPrompt.mockReturnValue(cancellation.promise);
    const { result } = renderControls();

    act(() => {
      result.current.handleCancelPrompt();
      result.current.handleCancelPrompt();
    });
    await waitFor(() => expect(result.current.isStopping).toBe(true));
    act(() => {
      result.current.handleDelete();
    });

    expect(routeHookMocks.cancelSessionPrompt).toHaveBeenCalledOnce();
    expect(routeHookMocks.deleteMutation.mutate).not.toHaveBeenCalled();
    await act(async () => {
      cancellation.resolve();
      await cancellation.promise;
    });
  });

  it("Should allow a normal prompt to resume a stopped user session", () => {
    const { result } = renderControls(makeSession("stopped"));

    expect(result.current.canPrompt).toBe(true);
    expect(result.current.isSessionRunning).toBe(false);
  });

  it("Should block clear while work is running and allow durable transcript content when idle", () => {
    routeHookMocks.auiState.thread.isRunning = true;
    routeHookMocks.transcriptMessages = [{ id: "message-1" }];
    const running = renderControls();
    act(() => running.result.current.handleClear());
    expect(routeHookMocks.clearMutation.mutate).not.toHaveBeenCalled();
    running.unmount();

    routeHookMocks.auiState.thread.isRunning = false;
    const idle = renderControls();
    expect(idle.result.current.canClear).toBe(true);
    act(() => idle.result.current.handleClear());
    expect(routeHookMocks.clearMutation.mutate).toHaveBeenCalledOnce();
  });

  it("Should expose only the daemon queue projection across turn changes", () => {
    routeHookMocks.sessionInputsQuery.data = {
      inputs: [
        { id: "inq-1", mode: "queue", status: "queued", text: "Keep this", delivery: "after_turn" },
      ],
    };
    let session = makeSession("active", "turn-a");
    const { result, rerender } = renderHook(() =>
      useSessionPageControls("sess-1", session, { workspaceId: WORKSPACE_ID })
    );

    expect(result.current.queuedPrompts).toEqual([
      { id: "inq-1", mode: "queue", status: "queued", text: "Keep this" },
    ]);
    session = makeSession("active", "turn-b");
    rerender();
    expect(result.current.queuedPrompts).toEqual([
      { id: "inq-1", mode: "queue", status: "queued", text: "Keep this" },
    ]);
  });

  it("Should keep a queue draft pending until acknowledgement and emit no success toast", async () => {
    routeHookMocks.auiState.thread.isRunning = true;
    const admission = createDeferredPromise<unknown>();
    routeHookMocks.queuePromptMutation.mutateAsync.mockReturnValue(admission.promise);
    const { result } = renderControls();

    let request!: Promise<void>;
    act(() => {
      request = result.current.handleQueuePrompt("queue me")!;
    });
    expect(result.current.isBusyInputPending).toBe(true);
    expect(routeHookMocks.queuePromptMutation.mutateAsync).toHaveBeenCalledWith({
      id: "sess-1",
      message: "queue me",
    });

    await act(async () => {
      admission.resolve({ delivery: "after_turn", queue_entry_id: "inq-1", status: "queued" });
      await request;
    });
    expect(routeHookMocks.toastSuccess).not.toHaveBeenCalled();
  });

  it("Should fence direct steer and interrupt with the active turn", async () => {
    routeHookMocks.steerPromptMutation.mutateAsync.mockResolvedValue({ status: "steering" });
    routeHookMocks.interruptPromptMutation.mutateAsync.mockResolvedValue({
      status: "interrupting",
    });
    const { result } = renderControls(makeSession("active", "turn-live"));

    await act(async () => {
      await result.current.handleSteerPrompt("new constraint");
      await result.current.handleInterruptPrompt("replace the work");
    });

    expect(routeHookMocks.steerPromptMutation.mutateAsync).toHaveBeenCalledWith({
      expectedTurnId: "turn-live",
      id: "sess-1",
      message: "new constraint",
    });
    expect(routeHookMocks.interruptPromptMutation.mutateAsync).toHaveBeenCalledWith({
      expectedTurnId: "turn-live",
      id: "sess-1",
      message: "replace the work",
    });
  });

  it("Should block steer and interrupt when no active turn fence exists", async () => {
    routeHookMocks.auiState.thread.isRunning = true;
    const { result } = renderControls(makeSession("active"));

    await act(async () => {
      await result.current.handleSteerPrompt("new constraint");
      await result.current.handleInterruptPrompt("replace the work");
    });

    expect(routeHookMocks.steerPromptMutation.mutateAsync).not.toHaveBeenCalled();
    expect(routeHookMocks.interruptPromptMutation.mutateAsync).not.toHaveBeenCalled();
  });

  it("Should promote a durable queue entry with one atomic request", () => {
    const { result } = renderControls(makeSession("active", "turn-live"));
    act(() => result.current.handleSteerQueuedPrompt({ id: "inq-1", text: "steer me" }));

    expect(routeHookMocks.promoteInputMutation.mutate).toHaveBeenCalledOnce();
    expect(routeHookMocks.promoteInputMutation.mutate).toHaveBeenCalledWith(
      {
        queueEntryId: "inq-1",
        request: {
          expected_turn_id: "turn-live",
          idempotency_key: expect.any(String),
          message_id: expect.any(String),
          text: "steer me",
        },
      },
      expect.objectContaining({ onError: expect.any(Function) })
    );
  });

  it("Should replace a durable queue entry with one atomic request", async () => {
    routeHookMocks.replaceInputMutation.mutateAsync.mockResolvedValue({ id: "inq-2" });
    const { result } = renderControls(makeSession("active", "turn-live"));

    await act(async () => {
      await result.current.handleReplaceQueuedPrompt(
        { id: "inq-1", text: "old" },
        "new queued text"
      );
    });

    expect(routeHookMocks.replaceInputMutation.mutateAsync).toHaveBeenCalledWith({
      queueEntryId: "inq-1",
      request: {
        idempotency_key: expect.any(String),
        message_id: expect.any(String),
        text: "new queued text",
      },
    });
  });

  it("Should generate durable mutation identities without randomUUID", async () => {
    let fill = 1;
    const getRandomValues = vi.fn((bytes: Uint8Array) => {
      bytes.fill(fill);
      fill += 1;
      return bytes;
    });
    vi.stubGlobal("crypto", { getRandomValues });
    routeHookMocks.replaceInputMutation.mutateAsync.mockResolvedValue({ id: "inq-2" });
    const { result } = renderControls(makeSession("active", "turn-live"));

    act(() => result.current.handleSteerQueuedPrompt({ id: "inq-1", text: "steer me" }));
    await act(async () => {
      await result.current.handleReplaceQueuedPrompt(
        { id: "inq-1", text: "old" },
        "new queued text"
      );
    });

    expect(getRandomValues).toHaveBeenCalledTimes(4);
    expect(routeHookMocks.promoteInputMutation.mutate).toHaveBeenCalledWith(
      expect.objectContaining({
        request: expect.objectContaining({
          idempotency_key: expect.any(String),
          message_id: expect.any(String),
        }),
      }),
      expect.any(Object)
    );
    expect(routeHookMocks.replaceInputMutation.mutateAsync).toHaveBeenCalledWith(
      expect.objectContaining({
        request: expect.objectContaining({
          idempotency_key: expect.any(String),
          message_id: expect.any(String),
        }),
      })
    );
  });

  it("Should report a durable queue mutation failure without removing server state", () => {
    routeHookMocks.sessionInputsQuery.data = {
      inputs: [{ id: "inq-1", mode: "queue", status: "queued", text: "Keep me" }],
    };
    const { result } = renderControls(makeSession("active", "turn-live"));
    act(() => result.current.handleRemoveQueuedPrompt("inq-1"));
    const [, options] = routeHookMocks.cancelInputMutation.mutate.mock.calls[0] ?? [];
    act(() => options.onError(new Error("cancel failed")));

    expect(result.current.queuedPrompts).toEqual([
      { id: "inq-1", mode: "queue", status: "queued", text: "Keep me" },
    ]);
    expect(routeHookMocks.toastError).toHaveBeenCalledWith("cancel failed");
  });

  it("Should release the busy state after a failed acknowledgement", async () => {
    routeHookMocks.auiState.thread.isRunning = true;
    routeHookMocks.queuePromptMutation.mutateAsync.mockRejectedValue(new Error("queue failed"));
    const { result } = renderControls();

    await act(async () => {
      await expect(result.current.handleQueuePrompt("keep draft")).rejects.toThrow("queue failed");
    });
    await waitFor(() => expect(result.current.isBusyInputPending).toBe(false));
  });
});
