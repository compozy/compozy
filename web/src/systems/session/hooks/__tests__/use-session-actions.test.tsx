import { QueryClient, QueryClientProvider, type InfiniteData } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { sessionStore } from "../../stores/session-store";
import {
  useClearSessionConversation,
  useArchiveSession,
  useCreateSession,
  useDeleteSession,
  useInterruptSessionPrompt,
  useQueueSessionPrompt,
  useRepairSession,
  useRenameSession,
  useResumeSession,
  useSteerSessionPrompt,
  useUnarchiveSession,
} from "../use-session-actions";
import { useSessionRewind } from "../use-session-rewind";
import {
  useCancelSessionInput,
  usePromoteSessionInput,
  useReplaceSessionInput,
} from "../use-session-inputs";
import { sessionKeys } from "../../lib/query-keys";
import type { SessionTranscriptData } from "../../lib/session-transcript-query";
import type {
  SessionInputPayload,
  SessionInputsResponse,
  SessionMessage,
  SessionPayload,
  SessionsResponse,
} from "../../types";
import { PROFILE_AGGREGATE, resetProfileViews, setProfileView } from "@/systems/profiles";

vi.mock("../../adapters/session-api", async importOriginal => ({
  ...(await importOriginal<typeof import("../../adapters/session-api")>()),
  clearSessionConversation: vi.fn(),
  archiveSession: vi.fn(),
  cancelQueuedSessionPrompt: vi.fn(),
  createSession: vi.fn(),
  deleteSession: vi.fn(),
  repairSession: vi.fn(),
  renameSession: vi.fn(),
  promoteSessionInputToSteer: vi.fn(),
  replaceSessionInput: vi.fn(),
  rewindSession: vi.fn(),
  stopSession: vi.fn(),
  resumeSession: vi.fn(),
  sendSessionPrompt: vi.fn(),
  steerSessionPrompt: vi.fn(),
  unarchiveSession: vi.fn(),
}));

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => ({ activeWorkspaceId: "ws_alpha", runtimeWorkspaceId: "ws_alpha" }),
}));

vi.mock("sonner", () => ({ toast: { error: vi.fn() } }));
vi.mock("@/lib/user-feedback", () => ({ notifyUser: vi.fn() }));

import {
  cancelQueuedSessionPrompt,
  archiveSession,
  clearSessionConversation,
  createSession,
  deleteSession,
  repairSession,
  renameSession,
  resumeSession,
  promoteSessionInputToSteer,
  replaceSessionInput,
  rewindSession,
  sendSessionPrompt,
  steerSessionPrompt,
  unarchiveSession,
} from "../../adapters/session-api";
import { toast } from "sonner";
import { notifyUser } from "@/lib/user-feedback";
import { useSessionLifecycleActions } from "../use-session-lifecycle-actions";
const WORKSPACE_ID = "ws_alpha";

const queuedInput: SessionInputPayload = {
  delivery: "after_turn",
  enqueued_at: "2026-08-03T18:00:00Z",
  id: "input-original",
  mode: "queue",
  queue_generation: 1,
  session_id: "sess-created",
  status: "queued",
  text: "Original queued input",
};

function createWrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

const createdSession: SessionPayload = {
  profile_id: "00000000000000000000000000",
  profile_name: "default",
  id: "sess-created",
  name: "Created session",
  agent_name: "claude-agent",
  runtime: {
    status: "ready",
    transition: "initial_bind",
    effective: { provider: "claude" },
    selection_revision: 0,
  },
  workspace_id: "ws_alpha",
  workspace_path: "/workspace/alpha",
  state: "active",
  badge: "idle",
  attachable: true,
  archived_at: null,
  available_commands: [],
  pending_interactions: [],
  created_at: "2026-04-20T10:00:00Z",
  updated_at: "2026-04-20T10:00:01Z",
};

const staleCreatedSession: SessionPayload = {
  ...createdSession,
  name: "Stale session",
  state: "starting",
};

const existingSession: SessionPayload = {
  ...createdSession,
  id: "sess-existing",
  name: "Existing session",
  updated_at: "2026-04-19T10:00:00Z",
};

const otherWorkspaceSession: SessionPayload = {
  ...createdSession,
  id: "sess-other",
  workspace_id: "ws_beta",
  workspace_path: "/workspace/beta",
  name: "Other workspace session",
};

function sessionListCache(sessions: SessionPayload[]): InfiniteData<SessionsResponse, unknown> {
  return {
    pages: [
      {
        sessions,
        page: { has_more: false, limit: 50, total: sessions.length },
      },
    ],
    pageParams: [undefined],
  };
}

function transcriptCache(messageId = "history-1"): SessionTranscriptData {
  const message: SessionMessage = {
    id: messageId,
    role: "assistant",
    parts: [{ type: "text", text: "existing" }],
  };
  return {
    pages: [
      {
        cursor: 1,
        entries: [{ message, sequence: 1, start_sequence: 1 }],
        epoch: 1,
        generation: 1,
        has_older: false,
        limit: 200,
        max_sequence: 1,
      },
    ],
    pageParams: [undefined],
  };
}

describe("session actions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessionStore.trigger.allDraftsDiscarded();
    sessionStore.trigger.sessionInteractionRemoved({ sessionId: createdSession.id });
  });

  afterEach(() => {
    act(() => resetProfileViews());
    vi.restoreAllMocks();
  });

  it("useCreateSession seeds detail without replacing infinite list caches", async () => {
    vi.mocked(createSession).mockResolvedValue(createdSession);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const allSessions = sessionListCache([staleCreatedSession, existingSession]);
    const workspaceSessions = sessionListCache([existingSession]);
    const otherWorkspaceSessions = sessionListCache([otherWorkspaceSession]);
    queryClient.setQueryData(sessionKeys.list(), allSessions);
    queryClient.setQueryData(sessionKeys.list({ workspace_id: "ws_alpha" }), workspaceSessions);
    queryClient.setQueryData(sessionKeys.list({ workspace_id: "ws_beta" }), otherWorkspaceSessions);
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useCreateSession(), {
      wrapper: createWrapper(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync({
        agent_name: createdSession.agent_name,
        workspace: createdSession.workspace_id,
      });
    });

    // The acting profile rides the create call: omitting it would file every
    // web-created session into `default` regardless of the active profile.
    expect(createSession).toHaveBeenCalledWith(
      {
        agent_name: createdSession.agent_name,
        workspace: createdSession.workspace_id,
      },
      "default"
    );
    expect(queryClient.getQueryData(sessionKeys.detail(WORKSPACE_ID, createdSession.id))).toEqual(
      createdSession
    );
    expect(queryClient.getQueryData(sessionKeys.list())).toEqual(allSessions);
    expect(queryClient.getQueryData(sessionKeys.list({ workspace_id: "ws_alpha" }))).toEqual(
      workspaceSessions
    );
    expect(queryClient.getQueryData(sessionKeys.list({ workspace_id: "ws_beta" }))).toEqual(
      otherWorkspaceSessions
    );
    expect(
      queryClient.getQueryState(sessionKeys.list({ workspace_id: WORKSPACE_ID }))?.isInvalidated
    ).toBe(true);
    expect(
      queryClient.getQueryState(sessionKeys.list({ workspace_id: "ws_beta" }))?.isInvalidated
    ).toBe(false);
    expect(invalidateSpy).toHaveBeenNthCalledWith(1, {
      queryKey: sessionKeys.detail(WORKSPACE_ID, createdSession.id),
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(2, {
      queryKey: sessionKeys.workspaceLists(WORKSPACE_ID),
    });
  });

  it("Should announce the acting profile after aggregate session creation", async () => {
    act(() => setProfileView({ scope: "global" }, { kind: "aggregate" }));
    vi.mocked(createSession).mockResolvedValue(createdSession);
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const { result } = renderHook(() => useCreateSession(), {
      wrapper: createWrapper(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync({ workspace: WORKSPACE_ID });
    });

    expect(createSession).toHaveBeenCalledWith({ workspace: WORKSPACE_ID }, "default");
    expect(notifyUser).toHaveBeenCalledExactlyOnceWith({
      message: "Created in default.",
      tone: "success",
    });
  });

  it("useClearSessionConversation clears transcript caches optimistically without touching drafts", async () => {
    vi.mocked(clearSessionConversation).mockResolvedValue(createdSession);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    queryClient.setQueryData(sessionKeys.detail(WORKSPACE_ID, createdSession.id), createdSession);
    queryClient.setQueryData(
      sessionKeys.transcript(WORKSPACE_ID, createdSession.id),
      transcriptCache()
    );
    queryClient.setQueryData(sessionKeys.history(WORKSPACE_ID, createdSession.id), [
      { id: "turn-1" },
    ]);
    sessionStore.trigger.composerDraftChanged({ sessionId: createdSession.id, text: "keep me" });

    const { result } = renderHook(() => useClearSessionConversation(), {
      wrapper: createWrapper(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync(createdSession.id);
    });

    expect(clearSessionConversation).toHaveBeenCalledWith(WORKSPACE_ID, createdSession.id);
    expect(queryClient.getQueryData(sessionKeys.detail(WORKSPACE_ID, createdSession.id))).toEqual(
      createdSession
    );
    expect(
      queryClient.getQueryData(sessionKeys.transcript(WORKSPACE_ID, createdSession.id))
    ).toBeUndefined();
    expect(queryClient.getQueryData(sessionKeys.history(WORKSPACE_ID, createdSession.id))).toEqual(
      []
    );
    expect(sessionStore.getSnapshot().context.drafts[createdSession.id]).toBe("keep me");
  });

  it("useClearSessionConversation rolls back optimistic cache changes on failure", async () => {
    vi.mocked(clearSessionConversation).mockRejectedValue(new Error("clear failed"));

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    queryClient.setQueryData(sessionKeys.detail(WORKSPACE_ID, createdSession.id), createdSession);

    const transcriptSnapshot = transcriptCache();
    const historySnapshot = [{ id: "turn-1" }];
    queryClient.setQueryData(
      sessionKeys.transcript(WORKSPACE_ID, createdSession.id),
      transcriptSnapshot
    );
    queryClient.setQueryData(sessionKeys.history(WORKSPACE_ID, createdSession.id), historySnapshot);
    sessionStore.trigger.composerDraftChanged({ sessionId: createdSession.id, text: "keep me" });

    const { result } = renderHook(() => useClearSessionConversation(), {
      wrapper: createWrapper(queryClient),
    });

    await act(async () => {
      await expect(result.current.mutateAsync(createdSession.id)).rejects.toThrow("clear failed");
    });

    expect(
      queryClient.getQueryData(sessionKeys.transcript(WORKSPACE_ID, createdSession.id))
    ).toEqual(transcriptSnapshot);
    expect(queryClient.getQueryData(sessionKeys.history(WORKSPACE_ID, createdSession.id))).toEqual(
      historySnapshot
    );
    expect(sessionStore.getSnapshot().context.drafts[createdSession.id]).toBe("keep me");
  });

  it("useSessionRewind sends the current durable fence and refreshes the transcript", async () => {
    vi.mocked(rewindSession).mockResolvedValue({
      rewind: { draft_text: "Return to this request." },
      session: createdSession,
    });
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const freshTranscript = transcriptCache("message-rewound");
    const fetchInfiniteQuery = vi
      .spyOn(queryClient, "fetchInfiniteQuery")
      .mockResolvedValue(freshTranscript);
    const cachedTranscript = transcriptCache("message-001");
    queryClient.setQueryData(sessionKeys.transcript(WORKSPACE_ID, createdSession.id), {
      ...cachedTranscript,
      pages: [
        {
          ...cachedTranscript.pages[0]!,
          epoch: 3,
          generation: 7,
          max_sequence: 19,
        },
      ],
    });

    const { result } = renderHook(() => useSessionRewind(WORKSPACE_ID), {
      wrapper: createWrapper(queryClient),
    });
    const controller = new AbortController();

    await act(async () => {
      await result.current.mutateAsync({
        idempotencyKey: "rewind-idempotency-001",
        messageId: "message-001",
        sessionId: createdSession.id,
        signal: controller.signal,
      });
    });

    expect(rewindSession).toHaveBeenCalledWith(
      WORKSPACE_ID,
      createdSession.id,
      {
        expected_epoch: 3,
        expected_generation: 7,
        expected_max_sequence: 19,
        idempotency_key: "rewind-idempotency-001",
        message_id: "message-001",
      },
      controller.signal
    );
    expect(queryClient.getQueryData(sessionKeys.detail(WORKSPACE_ID, createdSession.id))).toEqual(
      createdSession
    );
    expect(
      queryClient.getQueryData(sessionKeys.transcript(WORKSPACE_ID, createdSession.id))
    ).toBeUndefined();
    expect(fetchInfiniteQuery).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: sessionKeys.transcript(WORKSPACE_ID, createdSession.id),
      })
    );
  });

  it("useDeleteSession removes cached session data and clears the draft", async () => {
    vi.mocked(deleteSession).mockImplementation(async () => {
      expect(sessionStore.getSnapshot().context.liveTailSuppressions[createdSession.id]).toBe(1);
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const cancelSpy = vi.spyOn(queryClient, "cancelQueries");
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    const onDeleteSuccess = vi.fn();
    queryClient.setQueryData(sessionKeys.detail(WORKSPACE_ID, createdSession.id), createdSession);
    queryClient.setQueryData(
      sessionKeys.transcript(WORKSPACE_ID, createdSession.id),
      transcriptCache()
    );
    queryClient.setQueryData(sessionKeys.history(WORKSPACE_ID, createdSession.id), [
      { id: "turn-1" },
    ]);
    queryClient.setQueryData(sessionKeys.events(WORKSPACE_ID, createdSession.id), [
      { id: "event-1" },
    ]);
    queryClient.setQueryData(sessionKeys.byId(createdSession.id, "default"), createdSession);
    queryClient.setQueryData(
      sessionKeys.byId(createdSession.id, PROFILE_AGGREGATE),
      createdSession
    );
    sessionStore.trigger.composerDraftChanged({ sessionId: createdSession.id, text: "remove me" });

    const { result } = renderHook(() => useDeleteSession({ onDeleteSuccess }), {
      wrapper: createWrapper(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync(createdSession.id);
    });

    expect(deleteSession).toHaveBeenCalledWith(WORKSPACE_ID, createdSession.id);
    expect(cancelSpy).toHaveBeenCalledWith({
      queryKey: sessionKeys.detail(WORKSPACE_ID, createdSession.id),
    });
    expect(cancelSpy).toHaveBeenCalledWith({
      queryKey: sessionKeys.byIdRoot(createdSession.id),
    });
    expect(
      queryClient.getQueryData(sessionKeys.detail(WORKSPACE_ID, createdSession.id))
    ).toBeUndefined();
    expect(
      queryClient.getQueryData(sessionKeys.transcript(WORKSPACE_ID, createdSession.id))
    ).toBeUndefined();
    expect(
      queryClient.getQueryData(sessionKeys.history(WORKSPACE_ID, createdSession.id))
    ).toBeUndefined();
    expect(
      queryClient.getQueryData(sessionKeys.events(WORKSPACE_ID, createdSession.id))
    ).toBeUndefined();
    expect(
      queryClient.getQueryData(sessionKeys.byId(createdSession.id, "default"))
    ).toBeUndefined();
    expect(
      queryClient.getQueryData(sessionKeys.byId(createdSession.id, PROFILE_AGGREGATE))
    ).toBeUndefined();
    expect(sessionStore.getSnapshot().context.drafts[createdSession.id]).toBeUndefined();
    expect(
      sessionStore.getSnapshot().context.liveTailSuppressions[createdSession.id]
    ).toBeUndefined();
    expect(onDeleteSuccess).toHaveBeenCalledOnce();
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: sessionKeys.workspaceLists(WORKSPACE_ID),
    });
    expect(onDeleteSuccess.mock.invocationCallOrder[0]).toBeLessThan(
      invalidateSpy.mock.invocationCallOrder[0] ?? Number.POSITIVE_INFINITY
    );
  });

  it("useDeleteSession preserves cached session data and drafts on failure", async () => {
    vi.mocked(deleteSession).mockRejectedValue(new Error("delete failed"));

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    queryClient.setQueryData(sessionKeys.detail(WORKSPACE_ID, createdSession.id), createdSession);

    const transcriptSnapshot = transcriptCache();
    const historySnapshot = [{ id: "turn-1" }];
    const eventsSnapshot = [{ id: "event-1" }];
    queryClient.setQueryData(
      sessionKeys.transcript(WORKSPACE_ID, createdSession.id),
      transcriptSnapshot
    );
    queryClient.setQueryData(sessionKeys.history(WORKSPACE_ID, createdSession.id), historySnapshot);
    queryClient.setQueryData(sessionKeys.events(WORKSPACE_ID, createdSession.id), eventsSnapshot);
    sessionStore.trigger.composerDraftChanged({ sessionId: createdSession.id, text: "keep me" });

    const { result } = renderHook(() => useDeleteSession(), {
      wrapper: createWrapper(queryClient),
    });

    await act(async () => {
      await expect(result.current.mutateAsync(createdSession.id)).rejects.toThrow("delete failed");
    });

    expect(queryClient.getQueryData(sessionKeys.detail(WORKSPACE_ID, createdSession.id))).toEqual(
      createdSession
    );
    expect(
      queryClient.getQueryData(sessionKeys.transcript(WORKSPACE_ID, createdSession.id))
    ).toEqual(transcriptSnapshot);
    expect(queryClient.getQueryData(sessionKeys.history(WORKSPACE_ID, createdSession.id))).toEqual(
      historySnapshot
    );
    expect(queryClient.getQueryData(sessionKeys.events(WORKSPACE_ID, createdSession.id))).toEqual(
      eventsSnapshot
    );
    expect(sessionStore.getSnapshot().context.drafts[createdSession.id]).toBe("keep me");
    expect(
      sessionStore.getSnapshot().context.liveTailSuppressions[createdSession.id]
    ).toBeUndefined();
  });

  it("useResumeSession keeps the live tail suspended through state reconciliation", async () => {
    vi.mocked(resumeSession).mockImplementation(async () => {
      expect(sessionStore.getSnapshot().context.liveTailSuppressions[createdSession.id]).toBe(1);
      return createdSession;
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const cancelSpy = vi.spyOn(queryClient, "cancelQueries");
    let releaseInvalidation!: () => void;
    const invalidation = new Promise<void>(resolve => {
      releaseInvalidation = resolve;
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries").mockReturnValue(invalidation);
    const { result } = renderHook(() => useResumeSession(), {
      wrapper: createWrapper(queryClient),
    });

    let mutation!: Promise<SessionPayload>;
    act(() => {
      mutation = result.current.mutateAsync(createdSession.id);
    });

    await waitFor(() => expect(invalidateSpy).toHaveBeenCalledTimes(3));
    expect(cancelSpy).toHaveBeenCalledWith({
      queryKey: sessionKeys.detail(WORKSPACE_ID, createdSession.id),
    });
    expect(cancelSpy).toHaveBeenCalledWith({
      queryKey: sessionKeys.byIdRoot(createdSession.id),
    });
    expect(sessionStore.getSnapshot().context.liveTailSuppressions[createdSession.id]).toBe(1);

    releaseInvalidation();
    await act(async () => {
      await mutation;
    });

    expect(
      sessionStore.getSnapshot().context.liveTailSuppressions[createdSession.id]
    ).toBeUndefined();
  });

  it.each(["success", "failure"] as const)(
    "Should retain live-tail suppression until overlapping mutation owners settle after delete %s",
    async deleteResult => {
      let resolveResume!: (session: SessionPayload) => void;
      let settleDelete!: () => void;
      const deleteError = new Error("delete failed");
      vi.mocked(resumeSession).mockReturnValue(
        new Promise(resolve => {
          resolveResume = resolve;
        })
      );
      vi.mocked(deleteSession).mockReturnValue(
        new Promise((resolve, reject) => {
          settleDelete = () => {
            if (deleteResult === "success") {
              resolve();
              return;
            }
            reject(deleteError);
          };
        })
      );
      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
      });
      const { result } = renderHook(
        () => ({ remove: useDeleteSession(), resume: useResumeSession() }),
        { wrapper: createWrapper(queryClient) }
      );

      let deleteMutation!: Promise<unknown>;
      let resumeMutation!: Promise<SessionPayload>;
      act(() => {
        deleteMutation = result.current.remove.mutateAsync(createdSession.id).catch(error => error);
        resumeMutation = result.current.resume.mutateAsync(createdSession.id);
      });

      await waitFor(() =>
        expect(sessionStore.getSnapshot().context.liveTailSuppressions[createdSession.id]).toBe(2)
      );

      settleDelete();
      await act(async () => {
        const settledDelete = await deleteMutation;
        expect(settledDelete).toBe(deleteResult === "success" ? undefined : deleteError);
      });
      expect(sessionStore.getSnapshot().context.liveTailSuppressions[createdSession.id]).toBe(1);

      resolveResume(createdSession);
      await act(async () => {
        await resumeMutation;
      });
      expect(
        sessionStore.getSnapshot().context.liveTailSuppressions[createdSession.id]
      ).toBeUndefined();
    }
  );

  it("useRepairSession invalidates the owning session query tree after repair completes", async () => {
    vi.mocked(repairSession).mockResolvedValue({
      session_id: createdSession.id,
      issues: [],
      actions: [
        {
          code: "append_terminal_error",
          turn_id: "turn-1",
          event_id: "ev-repair-1",
          persisted: true,
        },
      ],
      persisted: true,
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useRepairSession(), {
      wrapper: createWrapper(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync({
        id: createdSession.id,
        dry_run: true,
        force: true,
      });
    });

    expect(repairSession).toHaveBeenCalledWith(WORKSPACE_ID, createdSession.id, {
      dry_run: true,
      force: true,
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: sessionKeys.detail(WORKSPACE_ID, createdSession.id),
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: sessionKeys.workspaceLists(WORKSPACE_ID),
    });
  });

  it("archive and unarchive invalidate the owning session query tree", async () => {
    vi.mocked(archiveSession).mockResolvedValue({
      ...createdSession,
      archived_at: "2026-08-04T12:00:00Z",
    });
    vi.mocked(unarchiveSession).mockResolvedValue(createdSession);
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(
      () => ({ archive: useArchiveSession(), unarchive: useUnarchiveSession() }),
      { wrapper: createWrapper(queryClient) }
    );

    await act(async () => {
      await result.current.archive.mutateAsync(createdSession.id);
      await result.current.unarchive.mutateAsync(createdSession.id);
    });

    expect(archiveSession).toHaveBeenCalledWith(
      WORKSPACE_ID,
      createdSession.id,
      expect.any(AbortSignal)
    );
    expect(unarchiveSession).toHaveBeenCalledWith(
      WORKSPACE_ID,
      createdSession.id,
      expect.any(AbortSignal)
    );
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: sessionKeys.detail(WORKSPACE_ID, createdSession.id),
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: sessionKeys.workspaceLists(WORKSPACE_ID),
    });
  });

  it("aborts an in-flight archive request when its hook unmounts", async () => {
    let requestSignal: AbortSignal | undefined;
    vi.mocked(archiveSession).mockImplementation(
      (_workspaceId, _sessionId, signal) =>
        new Promise((_resolve, reject) => {
          requestSignal = signal;
          signal?.addEventListener("abort", () =>
            reject(new DOMException("Aborted", "AbortError"))
          );
        })
    );
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const { result, unmount } = renderHook(() => useArchiveSession(), {
      wrapper: createWrapper(queryClient),
    });

    let mutation!: Promise<SessionPayload>;
    act(() => {
      mutation = result.current.mutateAsync(createdSession.id);
    });
    await waitFor(() => expect(requestSignal).toBeInstanceOf(AbortSignal));
    unmount();

    expect(requestSignal?.aborted).toBe(true);
    await expect(mutation).rejects.toThrow("Aborted");
  });

  it("reports a lifecycle action failure without showing a success notification", async () => {
    vi.mocked(archiveSession).mockRejectedValue(new Error("Archive is unavailable"));
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const { result } = renderHook(() => useSessionLifecycleActions(), {
      wrapper: createWrapper(queryClient),
    });

    act(() => {
      result.current.actions.onArchive(createdSession);
    });

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("Archive is unavailable");
    });
  });

  it("keeps the delete confirmation open until deletion succeeds", async () => {
    let resolveDelete!: () => void;
    vi.mocked(deleteSession).mockImplementation(
      () =>
        new Promise<void>(resolve => {
          resolveDelete = resolve;
        })
    );
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const { result } = renderHook(() => useSessionLifecycleActions(), {
      wrapper: createWrapper(queryClient),
    });

    act(() => {
      result.current.actions.onDelete(createdSession);
    });
    expect(result.current.deleteDialog.open).toBe(true);

    act(() => result.current.deleteDialog.onConfirm());

    await waitFor(() => expect(result.current.deleteDialog.isDeleting).toBe(true));
    expect(result.current.deleteDialog.open).toBe(true);
    expect(result.current.deleteDialog.session).toEqual(createdSession);

    act(() => result.current.deleteDialog.onOpenChange(false));
    expect(result.current.deleteDialog.open).toBe(true);

    await act(async () => resolveDelete());
    await waitFor(() => expect(result.current.deleteDialog.open).toBe(false));
    expect(result.current.deleteDialog.session).toBeNull();
  });

  it("keeps the rename dialog open until the durable rename succeeds", async () => {
    let resolveRename!: (session: SessionPayload) => void;
    vi.mocked(renameSession).mockImplementation(
      () =>
        new Promise<SessionPayload>(resolve => {
          resolveRename = resolve;
        })
    );
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const { result } = renderHook(() => useSessionLifecycleActions(), {
      wrapper: createWrapper(queryClient),
    });

    act(() => result.current.actions.onRename(createdSession));
    expect(result.current.renameDialog.open).toBe(true);

    act(() => result.current.renameDialog.onConfirm("Release review"));
    await waitFor(() => expect(result.current.renameDialog.isRenaming).toBe(true));
    expect(renameSession).toHaveBeenCalledWith(
      WORKSPACE_ID,
      createdSession.id,
      { name: "Release review" },
      expect.any(AbortSignal)
    );

    act(() => result.current.renameDialog.onOpenChange(false));
    expect(result.current.renameDialog.open).toBe(true);

    await act(async () => resolveRename({ ...createdSession, name: "Release review" }));
    await waitFor(() => expect(result.current.renameDialog.open).toBe(false));
    expect(result.current.renameDialog.session).toBeNull();
  });

  it("updates the detail cache and invalidates session views after rename", async () => {
    const renamed = { ...createdSession, name: "Release review" };
    vi.mocked(renameSession).mockResolvedValue(renamed);
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries").mockResolvedValue(undefined);
    const { result } = renderHook(() => useRenameSession(), {
      wrapper: createWrapper(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync({ id: createdSession.id, name: "Release review" });
    });

    expect(queryClient.getQueryData(sessionKeys.detail(WORKSPACE_ID, createdSession.id))).toEqual(
      renamed
    );
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: sessionKeys.detail(WORKSPACE_ID, createdSession.id),
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: sessionKeys.workspaceLists(WORKSPACE_ID),
    });
  });

  it("useQueueSessionPrompt builds the canonical durable request from an action identity", async () => {
    vi.mocked(sendSessionPrompt).mockResolvedValue({
      delivery: "after_turn",
      idempotency_key: "idempotency-001",
      message_id: "message-001",
      replayed: false,
      status: "queued",
    });
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const { result } = renderHook(() => useQueueSessionPrompt(), {
      wrapper: createWrapper(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync({
        id: createdSession.id,
        idempotencyKey: "idempotency-001",
        message: "Queue this durable input.",
        messageId: "message-001",
      });
    });

    expect(sendSessionPrompt).toHaveBeenCalledWith(WORKSPACE_ID, createdSession.id, {
      idempotency_key: "idempotency-001",
      message_id: "message-001",
      messages: [
        {
          id: "message-001",
          parts: [{ text: "Queue this durable input.", type: "text" }],
          role: "user",
        },
      ],
      mode: "queue",
    });
  });

  it.each([
    ["only a message id", { messageId: "message-001" }],
    ["only an idempotency key", { idempotencyKey: "idempotency-001" }],
    ["a blank message id", { idempotencyKey: "idempotency-001", messageId: "   " }],
    ["a blank idempotency key", { idempotencyKey: "   ", messageId: "message-001" }],
  ])("useQueueSessionPrompt rejects %s in an explicit action identity", async (_case, identity) => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const { result } = renderHook(() => useQueueSessionPrompt(), {
      wrapper: createWrapper(queryClient),
    });

    await act(async () => {
      await expect(
        result.current.mutateAsync({
          id: createdSession.id,
          message: "Queue this durable input.",
          ...identity,
        })
      ).rejects.toThrow(
        "A session prompt action requires both non-empty message_id and idempotency_key"
      );
    });

    expect(sendSessionPrompt).not.toHaveBeenCalled();
  });

  it.each([
    ["interrupt", useInterruptSessionPrompt, sendSessionPrompt],
    ["steer", useSteerSessionPrompt, steerSessionPrompt],
  ] as const)(
    "use%sSessionPrompt rejects a missing active-turn fence",
    async (_mode, useAction, request) => {
      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
      });
      const { result } = renderHook(() => useAction(), {
        wrapper: createWrapper(queryClient),
      });

      await act(async () => {
        await expect(
          result.current.mutateAsync({
            expectedTurnId: "   ",
            id: createdSession.id,
            message: "Replace the active work.",
          })
        ).rejects.toThrow("requires a non-empty expected_turn_id");
      });

      expect(request).not.toHaveBeenCalled();
    }
  );

  it("useReplaceSessionInput swaps the original durable id only in the owning queue", async () => {
    const replacement: SessionInputPayload = {
      ...queuedInput,
      id: "input-replacement",
      text: "Replacement queued input",
    };
    vi.mocked(replaceSessionInput).mockResolvedValue(replacement);
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const owningQueue: SessionInputsResponse = { inputs: [queuedInput] };
    const otherSessionQueue: SessionInputsResponse = {
      inputs: [{ ...queuedInput, id: "other-session-input", session_id: "sess-other" }],
    };
    const otherWorkspaceQueue: SessionInputsResponse = {
      inputs: [{ ...queuedInput, id: "other-workspace-input" }],
    };
    queryClient.setQueryData(sessionKeys.inputQueue(WORKSPACE_ID, createdSession.id), owningQueue);
    queryClient.setQueryData(sessionKeys.inputQueue(WORKSPACE_ID, "sess-other"), otherSessionQueue);
    queryClient.setQueryData(
      sessionKeys.inputQueue("ws_beta", createdSession.id),
      otherWorkspaceQueue
    );

    const { result } = renderHook(() => useReplaceSessionInput(WORKSPACE_ID, createdSession.id), {
      wrapper: createWrapper(queryClient),
    });
    await act(async () => {
      await result.current.mutateAsync({
        queueEntryId: queuedInput.id,
        request: {
          idempotency_key: "idem-edit",
          message_id: "message-edit",
          text: replacement.text,
        },
      });
    });

    expect(replaceSessionInput).toHaveBeenCalledWith(
      WORKSPACE_ID,
      createdSession.id,
      queuedInput.id,
      {
        idempotency_key: "idem-edit",
        message_id: "message-edit",
        text: replacement.text,
      }
    );
    expect(
      queryClient.getQueryData(sessionKeys.inputQueue(WORKSPACE_ID, createdSession.id))
    ).toEqual({ inputs: [replacement] });
    expect(queryClient.getQueryData(sessionKeys.inputQueue(WORKSPACE_ID, "sess-other"))).toEqual(
      otherSessionQueue
    );
    expect(queryClient.getQueryData(sessionKeys.inputQueue("ws_beta", createdSession.id))).toEqual(
      otherWorkspaceQueue
    );
  });

  it("queue promote and cancel remove only the acknowledged durable entry", async () => {
    const promptResult = {
      delivery: "interrupt_then_prompt" as const,
      idempotency_key: "idem-mutation",
      message_id: "message-mutation",
      replayed: false,
      status: "steering",
    };
    vi.mocked(promoteSessionInputToSteer).mockResolvedValue(promptResult);
    vi.mocked(cancelQueuedSessionPrompt).mockResolvedValue({
      ...promptResult,
      delivery: "none",
      status: "canceled",
    });
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const retained = { ...queuedInput, id: "input-retained" };
    queryClient.setQueryData<SessionInputsResponse>(
      sessionKeys.inputQueue(WORKSPACE_ID, createdSession.id),
      { inputs: [queuedInput, retained] }
    );

    const promote = renderHook(() => usePromoteSessionInput(WORKSPACE_ID, createdSession.id), {
      wrapper: createWrapper(queryClient),
    });
    await act(async () => {
      await promote.result.current.mutateAsync({
        queueEntryId: queuedInput.id,
        request: {
          expected_turn_id: "turn-active",
          idempotency_key: "idem-mutation",
          message_id: "message-mutation",
          text: queuedInput.text,
        },
      });
    });
    expect(
      queryClient.getQueryData(sessionKeys.inputQueue(WORKSPACE_ID, createdSession.id))
    ).toEqual({ inputs: [retained] });

    const cancel = renderHook(() => useCancelSessionInput(WORKSPACE_ID, createdSession.id), {
      wrapper: createWrapper(queryClient),
    });
    await act(async () => {
      await cancel.result.current.mutateAsync(retained.id);
    });
    expect(
      queryClient.getQueryData(sessionKeys.inputQueue(WORKSPACE_ID, createdSession.id))
    ).toEqual({ inputs: [] });
  });
});
