import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const routeHookMocks = vi.hoisted(() => ({
  auiState: {
    thread: {
      isRunning: false,
      messages: [] as Array<{ id: string }>,
    },
  },
  transcriptMessages: [] as Array<{ id: string }>,
  resetThread: vi.fn(),
  toastError: vi.fn(),
  toastSuccess: vi.fn(),
  cancelSessionPrompt: vi.fn(),
  clearMutation: {
    isPending: false,
    mutate: vi.fn(),
  },
  deleteMutation: {
    isPending: false,
    mutate: vi.fn(),
  },
  resumeMutation: {
    isPending: false,
    mutate: vi.fn(),
  },
  queuePromptMutation: {
    isPending: false,
    mutateAsync: vi.fn(),
  },
  interruptPromptMutation: {
    isPending: false,
    mutateAsync: vi.fn(),
  },
  steerPromptMutation: {
    isPending: false,
    mutate: vi.fn(),
    mutateAsync: vi.fn(),
  },
  cancelQueuedPromptMutation: {
    isPending: false,
    mutate: vi.fn(),
  },
  stopMutation: {
    isPending: false,
    mutate: vi.fn(),
  },
}));

vi.mock("@assistant-ui/react", () => ({
  useAui: () => ({
    thread: () => ({
      reset: routeHookMocks.resetThread,
    }),
  }),
  useAuiState: (selector: (state: typeof routeHookMocks.auiState) => unknown) =>
    selector(routeHookMocks.auiState),
}));

vi.mock("sonner", () => ({
  toast: {
    error: routeHookMocks.toastError,
    success: routeHookMocks.toastSuccess,
  },
}));

vi.mock("@/systems/session", () => ({
  cancelSessionPrompt: routeHookMocks.cancelSessionPrompt,
  isSessionRunning: (session: {
    state?: string;
    badge?: string;
    activity?: { turn_id?: string };
  }) =>
    session.state !== "stopped" &&
    (Boolean(session.activity?.turn_id) || session.badge === "running"),
  isUserControllableSession: (session: { type?: string }) => (session.type ?? "user") === "user",
  useCancelQueuedSessionPrompt: () => routeHookMocks.cancelQueuedPromptMutation,
  useClearSessionConversation: () => routeHookMocks.clearMutation,
  useDeleteSession: () => routeHookMocks.deleteMutation,
  useInterruptSessionPrompt: () => routeHookMocks.interruptPromptMutation,
  useQueueSessionPrompt: () => routeHookMocks.queuePromptMutation,
  useResumeSession: () => routeHookMocks.resumeMutation,
  useSteerSessionPrompt: () => routeHookMocks.steerPromptMutation,
  useStopSession: () => routeHookMocks.stopMutation,
  useSessionTranscriptThreadMessages: () => routeHookMocks.transcriptMessages,
}));

import { useSessionPageControls } from "../use-session-page-controls";
import type { SessionPayload } from "@/systems/session";

const WORKSPACE_ID = "ws_alpha";

function makeSession(state: SessionPayload["state"]): SessionPayload {
  return {
    id: "sess-1",
    agent_name: "codex-agent",
    provider: "codex",
    workspace_id: WORKSPACE_ID,
    workspace_path: "/workspace",
    state,
    badge: state === "stopped" ? "stopped" : "idle",
    attachable: state !== "stopped",
    available_commands: [],
    type: "user",
    created_at: "2026-04-17T10:00:00Z",
    updated_at: "2026-04-17T10:00:00Z",
  };
}

function renderControls(
  state: SessionPayload["state"],
  options: Parameters<typeof useSessionPageControls>[2] = {}
) {
  return renderHook(() =>
    useSessionPageControls("sess-1", makeSession(state), { workspaceId: WORKSPACE_ID, ...options })
  );
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
  beforeEach(() => {
    routeHookMocks.auiState.thread.isRunning = false;
    routeHookMocks.auiState.thread.messages = [];
    routeHookMocks.transcriptMessages = [];
    routeHookMocks.resetThread.mockReset();
    routeHookMocks.toastError.mockReset();
    routeHookMocks.toastSuccess.mockReset();
    routeHookMocks.cancelSessionPrompt.mockReset();
    routeHookMocks.clearMutation.isPending = false;
    routeHookMocks.clearMutation.mutate.mockReset();
    routeHookMocks.deleteMutation.isPending = false;
    routeHookMocks.deleteMutation.mutate.mockReset();
    routeHookMocks.resumeMutation.isPending = false;
    routeHookMocks.resumeMutation.mutate.mockReset();
    routeHookMocks.queuePromptMutation.isPending = false;
    routeHookMocks.queuePromptMutation.mutateAsync.mockReset();
    routeHookMocks.interruptPromptMutation.isPending = false;
    routeHookMocks.interruptPromptMutation.mutateAsync.mockReset();
    routeHookMocks.steerPromptMutation.isPending = false;
    routeHookMocks.steerPromptMutation.mutate.mockReset();
    routeHookMocks.steerPromptMutation.mutateAsync.mockReset();
    routeHookMocks.cancelQueuedPromptMutation.isPending = false;
    routeHookMocks.cancelQueuedPromptMutation.mutate.mockReset();
    routeHookMocks.stopMutation.isPending = false;
    routeHookMocks.stopMutation.mutate.mockReset();
  });

  it("blocks delete while prompt cancellation is in flight", async () => {
    const cancelPrompt = createDeferredPromise<void>();
    routeHookMocks.auiState.thread.isRunning = true;
    routeHookMocks.cancelSessionPrompt.mockReturnValue(cancelPrompt.promise);

    const { result } = renderControls("active");

    await act(async () => {
      result.current.handleCancelPrompt();
    });

    act(() => {
      result.current.handleDelete();
    });

    expect(routeHookMocks.deleteMutation.mutate).not.toHaveBeenCalled();
    expect(routeHookMocks.cancelSessionPrompt).toHaveBeenCalledWith(WORKSPACE_ID, "sess-1");

    await act(async () => {
      cancelPrompt.resolve();
      await cancelPrompt.promise;
    });
  });

  it("blocks clear while another control mutation is pending", () => {
    routeHookMocks.auiState.thread.messages = [{ id: "message-1" }];
    routeHookMocks.resumeMutation.isPending = true;

    const { result } = renderControls("active");

    act(() => {
      result.current.handleClear();
    });

    expect(routeHookMocks.clearMutation.mutate).not.toHaveBeenCalled();
  });

  it("blocks clear while a prompt is still running", () => {
    routeHookMocks.auiState.thread.isRunning = true;
    routeHookMocks.auiState.thread.messages = [{ id: "message-1" }];

    const { result } = renderControls("active");

    act(() => {
      result.current.handleClear();
    });

    expect(routeHookMocks.clearMutation.mutate).not.toHaveBeenCalled();
  });

  it("allows clear when only the durable transcript has rendered content", () => {
    routeHookMocks.transcriptMessages = [{ id: "transcript-message-1" }];

    const { result } = renderControls("active");

    expect(result.current.canClear).toBe(true);

    act(() => {
      result.current.handleClear();
    });

    expect(routeHookMocks.clearMutation.mutate).toHaveBeenCalledTimes(1);
    expect(routeHookMocks.clearMutation.mutate).toHaveBeenCalledWith(
      "sess-1",
      expect.objectContaining({ onSuccess: expect.any(Function) })
    );
    const [, options] = routeHookMocks.clearMutation.mutate.mock.calls[0] ?? [];
    act(() => {
      options.onSuccess();
    });
    expect(routeHookMocks.resetThread).toHaveBeenCalledTimes(1);
  });

  it("blocks stop while another control action is pending", () => {
    routeHookMocks.auiState.thread.isRunning = true;
    routeHookMocks.clearMutation.isPending = true;

    const { result } = renderControls("active");

    act(() => {
      result.current.handleStop();
    });

    expect(routeHookMocks.cancelSessionPrompt).not.toHaveBeenCalled();
    expect(routeHookMocks.stopMutation.mutate).not.toHaveBeenCalled();
  });

  it("blocks resume while another control action is pending", () => {
    routeHookMocks.deleteMutation.isPending = true;

    const { result } = renderControls("stopped");

    act(() => {
      result.current.handleResume();
    });

    expect(routeHookMocks.resumeMutation.mutate).not.toHaveBeenCalled();
  });

  it("runs delete success side effects when controls are idle", () => {
    const onDeleteSuccess = vi.fn();
    const { result } = renderControls("active", { onDeleteSuccess });

    act(() => {
      result.current.handleDelete();
    });

    expect(routeHookMocks.deleteMutation.mutate).toHaveBeenCalledTimes(1);
    const [, options] = routeHookMocks.deleteMutation.mutate.mock.calls[0] ?? [];
    expect(options).toEqual(
      expect.objectContaining({
        onError: expect.any(Function),
        onSuccess: expect.any(Function),
      })
    );

    act(() => {
      options.onSuccess();
    });

    expect(routeHookMocks.resetThread).toHaveBeenCalledTimes(1);
    expect(routeHookMocks.toastSuccess).toHaveBeenCalledWith("Session deleted.");
    expect(onDeleteSuccess).toHaveBeenCalledTimes(1);
  });

  it("shows the delete error toast from the mutation callback", () => {
    const { result } = renderControls("active");

    act(() => {
      result.current.handleDelete();
    });

    const [, options] = routeHookMocks.deleteMutation.mutate.mock.calls[0] ?? [];

    act(() => {
      options.onError(new Error("delete failed"));
    });

    expect(routeHookMocks.toastError).toHaveBeenCalledWith("delete failed");
  });

  it("captures the daemon queue entry id and exposes it as a queued prompt row", async () => {
    routeHookMocks.auiState.thread.isRunning = true;
    routeHookMocks.queuePromptMutation.mutateAsync.mockResolvedValue({
      queued: true,
      queue_entry_id: "inq-1",
    });

    const { result } = renderControls("active");

    await act(async () => {
      await result.current.handleQueuePrompt("queue me");
    });

    expect(routeHookMocks.queuePromptMutation.mutateAsync).toHaveBeenCalledWith({
      id: "sess-1",
      message: "queue me",
    });
    expect(result.current.queuedPrompts).toEqual([{ id: "inq-1", text: "queue me" }]);
  });

  it("does not show a queued prompt toast when the daemon runs the prompt immediately", async () => {
    routeHookMocks.auiState.thread.isRunning = true;
    routeHookMocks.queuePromptMutation.mutateAsync.mockResolvedValue({
      queued: false,
    });

    const { result } = renderControls("active");

    await act(async () => {
      await result.current.handleQueuePrompt("run now");
    });

    expect(routeHookMocks.toastSuccess).not.toHaveBeenCalled();
    expect(result.current.queuedPrompts).toEqual([]);
  });

  it("removes a queued prompt through the cancel-queued API and drops its row", async () => {
    routeHookMocks.auiState.thread.isRunning = true;
    routeHookMocks.queuePromptMutation.mutateAsync.mockResolvedValue({
      queued: true,
      queue_entry_id: "inq-1",
    });

    const { result } = renderControls("active");

    await act(async () => {
      await result.current.handleQueuePrompt("queue me");
    });

    act(() => {
      result.current.handleRemoveQueuedPrompt("inq-1");
    });

    expect(routeHookMocks.cancelQueuedPromptMutation.mutate).toHaveBeenCalledWith(
      { id: "sess-1", queueEntryId: "inq-1" },
      expect.objectContaining({ onError: expect.any(Function) })
    );
    expect(result.current.queuedPrompts).toEqual([]);
  });

  it("restores a queued prompt when cancel-queued fails", async () => {
    routeHookMocks.auiState.thread.isRunning = true;
    routeHookMocks.queuePromptMutation.mutateAsync.mockResolvedValue({
      queued: true,
      queue_entry_id: "inq-1",
    });

    const { result } = renderControls("active");

    await act(async () => {
      await result.current.handleQueuePrompt("queue me");
    });

    act(() => {
      result.current.handleRemoveQueuedPrompt("inq-1");
    });
    const [, options] = routeHookMocks.cancelQueuedPromptMutation.mutate.mock.calls[0] ?? [];
    act(() => {
      options.onError(new Error("cancel failed"));
    });

    expect(result.current.queuedPrompts).toEqual([{ id: "inq-1", text: "queue me" }]);
    expect(routeHookMocks.toastError).toHaveBeenCalledWith("cancel failed");
  });

  it("steers a queued prompt into the live turn then cancels its durable entry", () => {
    routeHookMocks.auiState.thread.isRunning = true;
    routeHookMocks.steerPromptMutation.mutate.mockImplementation(
      (_params: unknown, options: { onSuccess?: () => void }) => options?.onSuccess?.()
    );
    routeHookMocks.cancelQueuedPromptMutation.mutate.mockImplementation(
      (_params: unknown, options: { onSuccess?: () => void }) => options?.onSuccess?.()
    );

    const { result } = renderControls("active");

    act(() => {
      result.current.handleSteerQueuedPrompt({ id: "inq-9", text: "steer me" });
    });

    expect(routeHookMocks.steerPromptMutation.mutate).toHaveBeenCalledWith(
      { id: "sess-1", message: "steer me" },
      expect.objectContaining({ onSuccess: expect.any(Function), onError: expect.any(Function) })
    );
    expect(routeHookMocks.cancelQueuedPromptMutation.mutate).toHaveBeenCalledWith(
      {
        id: "sess-1",
        queueEntryId: "inq-9",
      },
      expect.objectContaining({ onSuccess: expect.any(Function), onError: expect.any(Function) })
    );
    expect(routeHookMocks.toastSuccess).toHaveBeenCalledWith("Steer staged.");
  });

  it("restores a queued prompt when queued steer fails", async () => {
    routeHookMocks.auiState.thread.isRunning = true;
    routeHookMocks.queuePromptMutation.mutateAsync.mockResolvedValue({
      queued: true,
      queue_entry_id: "inq-1",
    });

    const { result } = renderControls("active");

    await act(async () => {
      await result.current.handleQueuePrompt("queue me");
    });

    act(() => {
      result.current.handleSteerQueuedPrompt({ id: "inq-1", text: "queue me" });
    });
    const [, options] = routeHookMocks.steerPromptMutation.mutate.mock.calls[0] ?? [];
    act(() => {
      options.onError(new Error("steer failed"));
    });

    expect(result.current.queuedPrompts).toEqual([{ id: "inq-1", text: "queue me" }]);
    expect(routeHookMocks.toastError).toHaveBeenCalledWith("steer failed");
  });

  it("restores a queued prompt when steering cannot cancel the durable queue entry", async () => {
    routeHookMocks.auiState.thread.isRunning = true;
    routeHookMocks.queuePromptMutation.mutateAsync.mockResolvedValue({
      queued: true,
      queue_entry_id: "inq-1",
    });
    routeHookMocks.steerPromptMutation.mutate.mockImplementation(
      (_params: unknown, options: { onSuccess?: () => void }) => options?.onSuccess?.()
    );
    routeHookMocks.cancelQueuedPromptMutation.mutate.mockImplementation(
      (_params: unknown, options: { onError?: (error: Error) => void }) =>
        options?.onError?.(new Error("cancel failed"))
    );

    const { result } = renderControls("active");

    await act(async () => {
      await result.current.handleQueuePrompt("queue me");
    });
    act(() => {
      result.current.handleSteerQueuedPrompt({ id: "inq-1", text: "queue me" });
    });

    expect(result.current.queuedPrompts).toEqual([{ id: "inq-1", text: "queue me" }]);
    expect(routeHookMocks.toastError).toHaveBeenCalledWith("cancel failed");
  });

  it("blocks queued steer while another busy-input mutation is pending", () => {
    routeHookMocks.auiState.thread.isRunning = true;
    routeHookMocks.steerPromptMutation.isPending = true;

    const { result } = renderControls("active");

    act(() => {
      result.current.handleSteerQueuedPrompt({ id: "inq-1", text: "queue me" });
    });

    expect(routeHookMocks.steerPromptMutation.mutate).not.toHaveBeenCalled();
  });

  it("clears queued prompts when a prompt is interrupted", async () => {
    routeHookMocks.auiState.thread.isRunning = true;
    routeHookMocks.queuePromptMutation.mutateAsync.mockResolvedValue({
      queued: true,
      queue_entry_id: "inq-1",
    });
    routeHookMocks.interruptPromptMutation.mutateAsync.mockResolvedValue({ queued: false });

    const { result } = renderControls("active");

    await act(async () => {
      await result.current.handleQueuePrompt("queue me");
    });
    expect(result.current.queuedPrompts).toHaveLength(1);

    await act(async () => {
      await result.current.handleInterruptPrompt("stop and do this instead");
    });

    expect(routeHookMocks.interruptPromptMutation.mutateAsync).toHaveBeenCalledWith({
      id: "sess-1",
      message: "stop and do this instead",
    });
    expect(result.current.queuedPrompts).toEqual([]);
  });

  it("drops queued prompts once the running turn settles", async () => {
    routeHookMocks.auiState.thread.isRunning = true;
    routeHookMocks.queuePromptMutation.mutateAsync.mockResolvedValue({
      queued: true,
      queue_entry_id: "inq-1",
    });

    const { result, rerender } = renderControls("active");

    await act(async () => {
      await result.current.handleQueuePrompt("queue me");
    });
    expect(result.current.queuedPrompts).toHaveLength(1);

    routeHookMocks.auiState.thread.isRunning = false;
    act(() => {
      rerender();
    });

    expect(result.current.queuedPrompts).toEqual([]);
  });

  it("drops queued prompts when the daemon advances to a new active turn", async () => {
    routeHookMocks.auiState.thread.isRunning = true;
    routeHookMocks.queuePromptMutation.mutateAsync.mockResolvedValue({
      queued: true,
      queue_entry_id: "inq-1",
    });
    let session = {
      ...makeSession("active"),
      badge: "running",
      activity: { turn_id: "turn-1" } as SessionPayload["activity"],
    };

    const { result, rerender } = renderHook(() =>
      useSessionPageControls("sess-1", session, { workspaceId: WORKSPACE_ID })
    );

    await act(async () => {
      await result.current.handleQueuePrompt("queue me");
    });
    expect(result.current.queuedPrompts).toEqual([{ id: "inq-1", text: "queue me" }]);

    session = {
      ...session,
      activity: { turn_id: "turn-2" } as SessionPayload["activity"],
    };
    act(() => {
      rerender();
    });

    expect(result.current.queuedPrompts).toEqual([]);
  });

  it("keeps the current turn queue when an older turn callback resolves late", async () => {
    routeHookMocks.auiState.thread.isRunning = true;
    routeHookMocks.queuePromptMutation.mutateAsync
      .mockResolvedValueOnce({ queued: true, queue_entry_id: "inq-turn-1" })
      .mockResolvedValueOnce({ queued: true, queue_entry_id: "inq-turn-2" });
    let session = {
      ...makeSession("active"),
      badge: "running",
      activity: { turn_id: "turn-1" } as SessionPayload["activity"],
    };

    const { result, rerender } = renderHook(() =>
      useSessionPageControls("sess-1", session, { workspaceId: WORKSPACE_ID })
    );

    await act(async () => {
      await result.current.handleQueuePrompt("older prompt");
    });
    act(() => {
      result.current.handleRemoveQueuedPrompt("inq-turn-1");
    });
    const [, olderTurnOptions] =
      routeHookMocks.cancelQueuedPromptMutation.mutate.mock.calls[0] ?? [];

    session = {
      ...session,
      activity: { turn_id: "turn-2" } as SessionPayload["activity"],
    };
    act(() => {
      rerender();
    });
    await act(async () => {
      await result.current.handleQueuePrompt("current prompt");
    });

    act(() => {
      olderTurnOptions.onError(new Error("late cancel failure"));
    });

    expect(result.current.queuedPrompts).toEqual([{ id: "inq-turn-2", text: "current prompt" }]);
  });
});
