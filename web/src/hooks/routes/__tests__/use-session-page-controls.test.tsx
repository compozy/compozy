// Suite: Web session-page prompt and lifecycle controls
// Invariant: public session types share prompt controls while lifecycle controls remain user-only.
// Boundary IN: useSessionPageControls and the real session prompt eligibility policy.
// Boundary OUT: rendered composer/topbar wiring and daemon transport contracts.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const routeHookMocks = vi.hoisted(() => ({
  auiState: { thread: { isRunning: false, messages: [] as Array<{ id: string }> } },
  transcriptMessages: [] as Array<{ id: string }>,
  resetThread: vi.fn(),
  toastError: vi.fn(),
  toastSuccess: vi.fn(),
  cancelSessionPrompt: vi.fn(),
  invalidateSessionMutationQueries: vi.fn(async () => undefined),
  clearMutation: { isPending: false, mutate: vi.fn() },
  deleteOptions: { current: undefined as { onDeleteSuccess?: () => void } | undefined },
  deleteMutation: { isPending: false, mutate: vi.fn() },
  renameMutation: { isPending: false, mutateAsync: vi.fn() },
  resumeMutation: { isPending: false, mutateAsync: vi.fn() },
  unarchiveMutation: { isPending: false, mutate: vi.fn() },
  sendPromptMutation: { isPending: false, mutateAsync: vi.fn() },
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
  const { queuedPromptAttachmentSummary } = await import("@/systems/session/lib/queued-prompt");
  const busyInput = await import("@/systems/session/lib/session-busy-input");
  const refusal = await import("@/systems/session/lib/session-busy-input-refusal");
  const outcome = await import("@/systems/session/lib/session-send-outcome");
  const stopAttention = await import("@/systems/session/lib/session-stop-attention");
  return {
    ...busyInput,
    ...refusal,
    ...outcome,
    ...stopAttention,
    canPromptSession,
    cancelSessionPrompt: routeHookMocks.cancelSessionPrompt,
    invalidateSessionMutationQueries: routeHookMocks.invalidateSessionMutationQueries,
    isSessionRunning: (session: {
      state?: string;
      badge?: string;
      activity?: { turn_id?: string };
    }) =>
      session.state !== "stopped" &&
      (Boolean(session.activity?.turn_id) || session.badge === "running"),
    isUserControllableSession: (session: { type?: string }) => (session.type ?? "user") === "user",
    queuedPromptAttachmentSummary,
    useCancelSessionInput: () => routeHookMocks.cancelInputMutation,
    useClearSessionConversation: () => routeHookMocks.clearMutation,
    useDeleteSession: (options: { onDeleteSuccess?: () => void }) => {
      routeHookMocks.deleteOptions.current = options;
      return routeHookMocks.deleteMutation;
    },
    usePromoteSessionInput: () => routeHookMocks.promoteInputMutation,
    useSendSessionPrompt: () => routeHookMocks.sendPromptMutation,
    useReplaceSessionInput: () => routeHookMocks.replaceInputMutation,
    useRenameSession: () => routeHookMocks.renameMutation,
    useResumeSession: () => routeHookMocks.resumeMutation,
    useUnarchiveSession: () => routeHookMocks.unarchiveMutation,
    useSessionInputs: () => routeHookMocks.sessionInputsQuery,
    useSessionTranscriptThreadMessages: () => routeHookMocks.transcriptMessages,
    useStopSession: () => routeHookMocks.stopMutation,
  };
});

import type { SessionPayload } from "@/systems/session";
import { useSessionPageControls } from "../use-session-page-controls";

const WORKSPACE_ID = "ws_alpha";

function makeSession(
  state: SessionPayload["state"],
  turnId?: string,
  sessionType: NonNullable<SessionPayload["type"]> = "user"
): SessionPayload {
  return {
    profile_id: "00000000000000000000000000",
    profile_name: "default",
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
    pending_interactions: [],
    type: sessionType,
    created_at: "2026-04-17T10:00:00Z",
    updated_at: "2026-04-17T10:00:00Z",
  };
}

function createWrapper() {
  const queryClient = new QueryClient();
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
  };
}

function renderControls(session = makeSession("active"), onDeleteSuccess?: () => void) {
  return renderHook(
    () => useSessionPageControls("sess-1", session, { workspaceId: WORKSPACE_ID, onDeleteSuccess }),
    { wrapper: createWrapper() }
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
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  beforeEach(() => {
    routeHookMocks.auiState.thread.isRunning = false;
    routeHookMocks.auiState.thread.messages = [];
    routeHookMocks.transcriptMessages = [];
    routeHookMocks.deleteOptions.current = undefined;
    routeHookMocks.sessionInputsQuery.data = { inputs: [] };
    routeHookMocks.resetThread.mockReset();
    routeHookMocks.toastError.mockReset();
    routeHookMocks.toastSuccess.mockReset();
    routeHookMocks.cancelSessionPrompt.mockReset();
    routeHookMocks.invalidateSessionMutationQueries.mockClear();
    for (const mutation of [
      routeHookMocks.clearMutation,
      routeHookMocks.cancelInputMutation,
      routeHookMocks.promoteInputMutation,
    ]) {
      mutation.isPending = false;
      mutation.mutate.mockReset();
    }
    routeHookMocks.deleteMutation.isPending = false;
    routeHookMocks.deleteMutation.mutate.mockReset();
    routeHookMocks.resumeMutation.isPending = false;
    routeHookMocks.resumeMutation.mutateAsync.mockReset();
    routeHookMocks.renameMutation.isPending = false;
    routeHookMocks.renameMutation.mutateAsync.mockReset();
    routeHookMocks.unarchiveMutation.isPending = false;
    routeHookMocks.unarchiveMutation.mutate.mockReset();
    routeHookMocks.sendPromptMutation.isPending = false;
    routeHookMocks.sendPromptMutation.mutateAsync.mockReset();
    routeHookMocks.replaceInputMutation.isPending = false;
    routeHookMocks.replaceInputMutation.mutateAsync.mockReset();
    routeHookMocks.stopMutation.isPending = false;
    routeHookMocks.stopMutation.mutate.mockReset();
    routeHookMocks.stopMutation.mutateAsync.mockReset();
  });

  it("Should serialize managed prompt cancellation and block destructive controls", async () => {
    const cancellation = createDeferredPromise<void>();
    routeHookMocks.auiState.thread.isRunning = true;
    routeHookMocks.cancelSessionPrompt.mockReturnValue(cancellation.promise);
    const { result } = renderControls(makeSession("active", "turn-managed", "system"));

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

  // Invariant (US-009.AC-1/AC-3): the page reads stopping from the first
  // activation until the session payload stops reporting the turn; the cancel
  // acknowledgement alone never flips it back. Owning layer: page-controls
  // hook over the real store. Canonical suite: this file.
  it("Should keep reading stopping after the cancel is accepted until the session drops the turn", async () => {
    routeHookMocks.auiState.thread.isRunning = true;
    routeHookMocks.cancelSessionPrompt.mockResolvedValue(undefined);
    const { result, rerender } = renderHook(
      (session: SessionPayload) =>
        useSessionPageControls("sess-1", session, { workspaceId: WORKSPACE_ID }),
      { initialProps: makeSession("active", "turn-1"), wrapper: createWrapper() }
    );

    act(() => result.current.handleCancelPrompt());
    expect(result.current.isStopping).toBe(true);
    await waitFor(() => expect(routeHookMocks.cancelSessionPrompt).toHaveBeenCalledOnce());
    // The request settled, the daemon still reports turn-1: still stopping, still guarded.
    await act(async () => {
      await Promise.resolve();
    });
    expect(result.current.isStopping).toBe(true);
    act(() => result.current.handleCancelPrompt());
    expect(routeHookMocks.cancelSessionPrompt).toHaveBeenCalledOnce();

    routeHookMocks.auiState.thread.isRunning = false;
    rerender(makeSession("active"));
    await waitFor(() => expect(result.current.isStopping).toBe(false));
  });

  it("Should keep an accepted cancel stopping when the session reread fails", async () => {
    routeHookMocks.auiState.thread.isRunning = true;
    routeHookMocks.cancelSessionPrompt.mockResolvedValue(undefined);
    routeHookMocks.invalidateSessionMutationQueries.mockRejectedValueOnce(
      new Error("session reread failed")
    );
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => undefined);
    const { result } = renderControls(makeSession("active", "turn-1"));

    await act(async () => {
      result.current.handleCancelPrompt();
    });
    await waitFor(() =>
      expect(routeHookMocks.invalidateSessionMutationQueries).toHaveBeenCalledOnce()
    );
    await act(async () => {
      await Promise.resolve();
    });

    // Acceptance is authoritative: the reread failure is logged, not a failed stop.
    expect(result.current.isStopping).toBe(true);
    expect(routeHookMocks.toastError).not.toHaveBeenCalled();
    expect(consoleError).toHaveBeenCalledWith(
      "Failed to reread the session after cancelling its prompt",
      expect.any(Error)
    );
    act(() => result.current.handleCancelPrompt());
    expect(routeHookMocks.cancelSessionPrompt).toHaveBeenCalledOnce();
    consoleError.mockRestore();
  });

  it("Should return Stop to the operator when the cancel request fails", async () => {
    routeHookMocks.auiState.thread.isRunning = true;
    routeHookMocks.cancelSessionPrompt.mockRejectedValueOnce(new Error("daemon disconnected"));
    const { result } = renderControls(makeSession("active", "turn-1"));

    act(() => result.current.handleCancelPrompt());
    expect(result.current.isStopping).toBe(true);
    await waitFor(() => expect(result.current.isStopping).toBe(false));
    expect(routeHookMocks.toastError).toHaveBeenCalledWith("Failed to stop the current prompt.");

    routeHookMocks.cancelSessionPrompt.mockResolvedValue(undefined);
    await act(async () => {
      result.current.handleCancelPrompt();
    });
    expect(routeHookMocks.cancelSessionPrompt).toHaveBeenCalledTimes(2);
    // Accepted this time, and the daemon still reports turn-1: stopping holds.
    await waitFor(() => expect(result.current.isStopping).toBe(true));
  });

  it("Should read stopping from the daemon and still let the operator retry the stop", async () => {
    routeHookMocks.stopMutation.mutateAsync.mockResolvedValue(undefined);
    const { result } = renderControls(makeSession("stopping"));

    expect(result.current.isStopping).toBe(true);
    act(() => result.current.handleStop());
    // No attention on the session: an ordinary stop, accepted without waiting.
    await waitFor(() =>
      expect(routeHookMocks.stopMutation.mutateAsync).toHaveBeenCalledWith({
        id: "sess-1",
        wait: false,
      })
    );
  });

  // Invariant (US-009.AC-3, ADR-004 invariant 3): a stop the daemon could not
  // verify surfaces as durable attention on a session that still reads
  // stopping; Retry is the same session stop, waited on (`wait: true`) so the
  // guard holds until that request settles — not on a reread, not on an
  // unrelated metadata change — and only the read model clears the warning.
  // Owning layer: page-controls hook over the real store. Canonical suite: this file.
  it("Should surface an unverified stop and hold its waited retry until the daemon's answer settles", async () => {
    const settled = createDeferredPromise<{ status: string; verified: boolean }>();
    routeHookMocks.stopMutation.mutateAsync.mockReturnValue(settled.promise);
    const unverified: SessionPayload = {
      ...makeSession("stopping"),
      attention: "stop_verification_failed",
      badge: "needs-attention",
      escalated: true,
      updated_at: "2026-09-05T10:00:00Z",
    };
    const { result, rerender } = renderHook(
      (session: SessionPayload) =>
        useSessionPageControls("sess-1", session, { workspaceId: WORKSPACE_ID }),
      { initialProps: unverified, wrapper: createWrapper() }
    );

    expect(result.current.stopAttention).toBe("stop_verification_failed");
    expect(result.current.isStopping).toBe(true);
    expect(result.current.isStopRetrying).toBe(false);
    expect(result.current.canRetryStop).toBe(true);

    act(() => result.current.handleStop());
    expect(routeHookMocks.stopMutation.mutateAsync).toHaveBeenCalledWith({
      id: "sess-1",
      wait: true,
    });
    expect(result.current.isStopRetrying).toBe(true);

    // A rename (or any unrelated metadata stamp) rereads the session while the
    // retry is still waiting: the guard holds, the attention stands, no duplicate.
    rerender({ ...unverified, name: "Renamed while stopping", updated_at: "2026-09-05T10:00:05Z" });
    expect(result.current.isStopRetrying).toBe(true);
    expect(result.current.stopAttention).toBe("stop_verification_failed");
    act(() => result.current.handleStop());
    expect(routeHookMocks.stopMutation.mutateAsync).toHaveBeenCalledOnce();

    // The daemon settled this retry unverified: the guard releases, the read
    // model still carries the attention, and Retry is open again.
    await act(async () => {
      settled.resolve({ status: "stopping", verified: false });
      await settled.promise;
    });
    expect(result.current.isStopRetrying).toBe(false);
    expect(result.current.stopAttention).toBe("stop_verification_failed");
    expect(result.current.isStopping).toBe(true);

    // Only the read model clears the warning: a verified `stopped` from the daemon.
    rerender({ ...makeSession("stopped"), updated_at: "2026-09-05T10:00:40Z" });
    expect(result.current.stopAttention).toBeNull();
    expect(result.current.isStopping).toBe(false);
  });

  it("Should keep the attention and reopen Retry when the retry request itself fails", async () => {
    routeHookMocks.stopMutation.mutateAsync.mockRejectedValueOnce(new Error("daemon disconnected"));
    const { result } = renderControls({
      ...makeSession("stopping"),
      attention: "stop_verification_failed",
      badge: "needs-attention",
      updated_at: "2026-09-05T10:00:00Z",
    });

    // The rejection settles inside the same act as the request.
    await act(async () => {
      result.current.handleStop();
    });
    expect(result.current.isStopRetrying).toBe(false);
    expect(routeHookMocks.toastError).toHaveBeenCalledWith("daemon disconnected");
    expect(result.current.stopAttention).toBe("stop_verification_failed");
    expect(result.current.isStopping).toBe(true);

    routeHookMocks.stopMutation.mutateAsync.mockReturnValue(new Promise(() => undefined));
    act(() => result.current.handleStop());
    expect(routeHookMocks.stopMutation.mutateAsync).toHaveBeenCalledTimes(2);
    expect(routeHookMocks.stopMutation.mutateAsync).toHaveBeenLastCalledWith({
      id: "sess-1",
      wait: true,
    });
    expect(result.current.isStopRetrying).toBe(true);
  });

  it("Should read an unverified stop on a managed session without offering the user-only retry", () => {
    const { result } = renderControls({
      ...makeSession("stopping", undefined, "system"),
      attention: "stop_verification_failed",
      badge: "needs-attention",
      updated_at: "2026-09-05T10:00:00Z",
    });

    expect(result.current.stopAttention).toBe("stop_verification_failed");
    expect(result.current.canRetryStop).toBe(false);
    act(() => result.current.handleStop());
    expect(routeHookMocks.stopMutation.mutateAsync).not.toHaveBeenCalled();
  });

  it("Should stop a user session without canceling its active prompt through the prompt path", async () => {
    routeHookMocks.auiState.thread.isRunning = true;
    routeHookMocks.stopMutation.mutateAsync.mockResolvedValue(undefined);
    const { result } = renderControls(makeSession("active", "turn-user"));

    act(() => result.current.handleStop());

    await waitFor(() =>
      expect(routeHookMocks.stopMutation.mutateAsync).toHaveBeenCalledWith({
        id: "sess-1",
        wait: false,
      })
    );
    expect(routeHookMocks.cancelSessionPrompt).not.toHaveBeenCalled();
  });

  it("Should own delete success in the mutation lifecycle before the route unmounts", () => {
    const onDeleteSuccess = vi.fn();
    const { result } = renderControls(makeSession("stopped"), onDeleteSuccess);

    act(() => result.current.handleDelete());
    act(() => routeHookMocks.deleteOptions.current?.onDeleteSuccess?.());

    expect(routeHookMocks.deleteMutation.mutate).toHaveBeenCalledWith("sess-1", {
      onError: expect.any(Function),
    });
    expect(routeHookMocks.resetThread).toHaveBeenCalledOnce();
    expect(routeHookMocks.toastSuccess).toHaveBeenCalledWith("Session deleted.");
    expect(onDeleteSuccess).toHaveBeenCalledOnce();
  });

  it.each(["user", "system", "coordinator", "spawned"] as const)(
    "Should allow normal prompts for active and stopped %s sessions",
    sessionType => {
      const active = renderControls(makeSession("active", undefined, sessionType));
      expect(active.result.current.canPrompt).toBe(true);
      expect(active.result.current.allowBusyInput).toBe(true);
      active.unmount();

      const stopped = renderControls(makeSession("stopped", undefined, sessionType));
      expect(stopped.result.current.canPrompt).toBe(true);
      expect(stopped.result.current.isSessionRunning).toBe(false);
    }
  );

  it.each([
    { label: "dream", session: makeSession("active", undefined, "dream") },
    { label: "missing-type", session: { ...makeSession("active"), type: undefined } },
    { label: "starting", session: makeSession("starting", undefined, "system") },
    { label: "stopping", session: makeSession("stopping", undefined, "system") },
    {
      label: "archived",
      session: {
        ...makeSession("stopped", undefined, "system"),
        archived_at: "2026-04-17T11:00:00Z",
      },
    },
  ])("Should keep $label sessions read-only", ({ session }) => {
    const { result } = renderControls(session);

    expect(result.current.canPrompt).toBe(false);
    expect(result.current.allowBusyInput).toBe(false);
  });

  it("Should keep a dead stopped session readable without allowing another prompt", () => {
    const { result } = renderControls({
      ...makeSession("stopped", undefined, "system"),
      failure: { kind: "process_exit", summary: "Codex exited" },
      health: {
        active_prompt: false,
        agent_name: "codex-agent",
        attachable: false,
        eligible_for_wake: false,
        health: "dead",
        session_id: "sess-1",
        state: "stopped",
        updated_at: "2026-04-17T10:00:00Z",
        workspace_id: WORKSPACE_ID,
      },
    });

    expect(result.current.canPrompt).toBe(false);
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

  it("Should reject lifecycle mutations for a managed session", async () => {
    routeHookMocks.transcriptMessages = [{ id: "message-1" }];
    const { result } = renderControls(makeSession("active", "turn-managed", "system"));

    expect(result.current.canClear).toBe(false);
    act(() => {
      result.current.handleStop();
      result.current.handleDelete();
      result.current.handleClear();
      result.current.handleResume();
      result.current.handleUnarchive();
    });
    await expect(result.current.handleRename("Managed title")).resolves.toBeUndefined();

    expect(routeHookMocks.stopMutation.mutateAsync).not.toHaveBeenCalled();
    expect(routeHookMocks.deleteMutation.mutate).not.toHaveBeenCalled();
    expect(routeHookMocks.clearMutation.mutate).not.toHaveBeenCalled();
    expect(routeHookMocks.resumeMutation.mutateAsync).not.toHaveBeenCalled();
    expect(routeHookMocks.unarchiveMutation.mutate).not.toHaveBeenCalled();
    expect(routeHookMocks.renameMutation.mutateAsync).not.toHaveBeenCalled();
    expect(routeHookMocks.cancelSessionPrompt).not.toHaveBeenCalled();
  });

  it("Should expose only the daemon queue projection across turn changes", () => {
    routeHookMocks.sessionInputsQuery.data = {
      inputs: [
        { id: "inq-1", mode: "queue", status: "queued", text: "Keep this", delivery: "after_turn" },
      ],
    };
    let session = makeSession("active", "turn-a");
    const { result, rerender } = renderHook(
      () => useSessionPageControls("sess-1", session, { workspaceId: WORKSPACE_ID }),
      { wrapper: createWrapper() }
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

  it("Should keep a queue draft pending until acknowledgement and resolve its disposition", async () => {
    routeHookMocks.auiState.thread.isRunning = true;
    const admission = createDeferredPromise<unknown>();
    routeHookMocks.sendPromptMutation.mutateAsync.mockReturnValue(admission.promise);
    const { result } = renderControls(makeSession("active", "turn-managed", "system"));

    let request!: Promise<unknown>;
    act(() => {
      request = result.current.handleQueuePrompt({ message: "queue me", attachments: [] })!;
    });
    expect(result.current.isBusyInputPending).toBe(true);
    expect(routeHookMocks.sendPromptMutation.mutateAsync).toHaveBeenCalledWith({
      expectedTurnId: "turn-managed",
      id: "sess-1",
      message: "queue me",
      mode: "queue",
    });

    let outcome: unknown;
    await act(async () => {
      admission.resolve({
        delivery: "after_turn",
        disposition: "queued",
        entry_id: "inq-1",
        idempotency_key: "idk-1",
        message_id: "msg-1",
        queue_position: 2,
        replayed: false,
        status: "queued",
        turn_id: "turn-managed",
      });
      outcome = await request;
    });
    expect(outcome).toEqual({
      disposition: "queued",
      entryId: "inq-1",
      idempotencyKey: "idk-1",
      messageId: "msg-1",
      queuePosition: 2,
      replayed: false,
      steerDelivery: null,
      turnId: "turn-managed",
    });
    expect(routeHookMocks.toastSuccess).not.toHaveBeenCalled();
  });

  it("Should expose the daemon follow-up default and steer delivery from the session resource", () => {
    const { result } = renderControls({
      ...makeSession("active", "turn-live"),
      busy_input: {
        default_mode: "queue",
        steer_capability: "none",
        steer_delivery: "interrupt_fallback",
      },
    });

    expect(result.current.busyInputDefaultMode).toBe("queue");
    expect(result.current.busyInputSteerDelivery).toBe("interrupt_fallback");
    // No report yet: the shipped daemon default (steer) and an unknown delivery.
    const { result: bare } = renderControls(makeSession("active", "turn-live"));
    expect(bare.current.busyInputDefaultMode).toBe("steer");
    expect(bare.current.busyInputSteerDelivery).toBeNull();
  });

  it("Should reject gated busy sends with their reason instead of a silent no-op", async () => {
    routeHookMocks.auiState.thread.isRunning = true;
    const stopped = { ...makeSession("stopped"), archived_at: "2026-04-17T10:00:00Z" };
    const { result: gated } = renderControls(stopped);
    await expect(
      gated.current.handleQueuePrompt({ message: "after archive", attachments: [] })
    ).rejects.toMatchObject({ refusal: { code: "session_not_promptable" } });

    const { result } = renderControls(makeSession("active", "turn-live"));
    await expect(
      result.current.handleSteerPrompt({
        message: "steer with a file",
        attachments: [
          {
            bytes: 4,
            height: 10,
            id: `att_${"b".repeat(64)}`,
            kind: "image",
            mime_type: "image/png",
            name: "steer.png",
            sha256: "b".repeat(64),
            width: 10,
          },
        ],
      })
    ).rejects.toMatchObject({
      refusal: { attachmentCount: 1, code: "steer_attachments_unsupported" },
    });

    const admission = createDeferredPromise<unknown>();
    routeHookMocks.sendPromptMutation.mutateAsync.mockReturnValue(admission.promise);
    let first!: Promise<unknown>;
    act(() => {
      first = result.current.handleSteerPrompt({ message: "first", attachments: [] })!;
    });
    await expect(
      result.current.handleQueuePrompt({ message: "second", attachments: [] })
    ).rejects.toMatchObject({ refusal: { code: "send_in_flight" } });
    admission.resolve({
      delivery: "direct",
      disposition: "steering",
      idempotency_key: "idk-1",
      message_id: "msg-1",
      queue_position: 0,
      replayed: false,
      status: "steering",
      steer_delivery: "injected",
      turn_id: "turn-live",
    });
    await act(async () => {
      await first;
    });
    expect(routeHookMocks.sendPromptMutation.mutateAsync).toHaveBeenCalledOnce();
  });

  it("Should fence direct steer and interrupt with the active turn", async () => {
    routeHookMocks.sendPromptMutation.mutateAsync.mockResolvedValue({
      delivery: "direct",
      idempotency_key: "idk",
      message_id: "msg",
      queue_position: 0,
      replayed: false,
      status: "steering",
    });
    const { result } = renderControls(makeSession("active", "turn-live", "system"));

    await act(async () => {
      await result.current.handleSteerPrompt({ message: "new constraint", attachments: [] });
      await result.current.handleInterruptPrompt({
        message: "replace the work",
        attachments: [
          {
            bytes: 4,
            height: 10,
            id: `att_${"a".repeat(64)}`,
            kind: "image",
            mime_type: "image/png",
            name: "interrupt.png",
            sha256: "a".repeat(64),
            width: 10,
          },
        ],
      });
    });

    expect(routeHookMocks.sendPromptMutation.mutateAsync).toHaveBeenCalledWith({
      expectedTurnId: "turn-live",
      id: "sess-1",
      message: "new constraint",
      mode: "steer",
    });
    expect(routeHookMocks.sendPromptMutation.mutateAsync).toHaveBeenCalledWith({
      expectedTurnId: "turn-live",
      id: "sess-1",
      message: "replace the work",
      mode: "interrupt",
      attachments: [
        {
          bytes: 4,
          height: 10,
          id: `att_${"a".repeat(64)}`,
          kind: "image",
          mime_type: "image/png",
          name: "interrupt.png",
          sha256: "a".repeat(64),
          width: 10,
        },
      ],
    });
  });

  it("Should let the daemon resolve the fence when no active turn id is known yet", async () => {
    // Invariant 6: an omitted fence resolves the live turn at admission; the
    // browser no longer blocks steer and interrupt behind its own poll.
    routeHookMocks.auiState.thread.isRunning = true;
    routeHookMocks.sendPromptMutation.mutateAsync.mockResolvedValue({
      delivery: "direct",
      idempotency_key: "idk",
      message_id: "msg",
      queue_position: 0,
      replayed: false,
      status: "steering",
    });
    const { result } = renderControls(makeSession("active"));

    await act(async () => {
      await result.current.handleSteerPrompt({ message: "new constraint", attachments: [] });
      await result.current.handleInterruptPrompt({ message: "replace the work", attachments: [] });
    });

    expect(routeHookMocks.sendPromptMutation.mutateAsync).toHaveBeenNthCalledWith(1, {
      id: "sess-1",
      message: "new constraint",
      mode: "steer",
    });
    expect(routeHookMocks.sendPromptMutation.mutateAsync).toHaveBeenNthCalledWith(2, {
      id: "sess-1",
      message: "replace the work",
      mode: "interrupt",
    });
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
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => undefined);
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
    expect(routeHookMocks.toastError).toHaveBeenCalledWith("Couldn't remove queued prompt.");
    expect(consoleError).toHaveBeenCalledWith("Failed to remove a queued prompt");
    expect(consoleError.mock.calls.flat()).not.toContainEqual(expect.any(Error));
    consoleError.mockRestore();
  });

  it("Should release the busy state after a failed acknowledgement", async () => {
    routeHookMocks.auiState.thread.isRunning = true;
    routeHookMocks.sendPromptMutation.mutateAsync.mockRejectedValue(new Error("queue failed"));
    const { result } = renderControls();

    await act(async () => {
      await expect(
        result.current.handleQueuePrompt({ message: "keep draft", attachments: [] })
      ).rejects.toThrow("queue failed");
    });
    await waitFor(() => expect(result.current.isBusyInputPending).toBe(false));
  });
});
