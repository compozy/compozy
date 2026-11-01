import { QueryClient, QueryClientProvider, type InfiniteData } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  composerDraftAttachmentFiles,
  recordComposerDraftAttachments,
  retainComposerDraftUpload,
  retainedComposerDraftUpload,
} from "../../lib/session-composer-attachment-drafts";
import { sessionStore } from "../../stores/session-store";
import {
  useClearSessionConversation,
  useArchiveSession,
  useCreateSession,
  useDeleteSession,
  useSessionDeletedByOperator,
  useSessionDeletionPending,
  useRepairSession,
  useRenameSession,
  useStopSession,
  useUnarchiveSession,
} from "../use-session-actions";
import { useSendSessionPrompt } from "../use-session-prompt-actions";
import { useSessionRewind } from "../use-session-rewind";
import {
  useSessionInputs,
  useCancelSessionInput,
  useClearSessionInputs,
  usePromoteSessionInput,
  useReplaceSessionInput,
} from "../use-session-inputs";
import { sessionScopedDetailOptions } from "../../lib/query-options";
import { sessionKeys } from "../../lib/query-keys";
import { listSessionDescendants } from "../../adapters/session-archive-api";
import type { SessionTranscriptData } from "../../lib/session-transcript-query";
import type {
  SessionGoalCommandResult,
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
  clearSessionInputs: vi.fn(),
  createSession: vi.fn(),
  deleteSession: vi.fn(),
  fetchSession: vi.fn(),
  fetchSessionInputs: vi.fn(),
  repairSession: vi.fn(),
  renameSession: vi.fn(),
  setSessionParent: vi.fn(),
  clearSessionParent: vi.fn(),
  promoteSessionInputToSteer: vi.fn(),
  replaceSessionInput: vi.fn(),
  rewindSession: vi.fn(),
  stopSession: vi.fn(),
  sendSessionPrompt: vi.fn(),
  unarchiveSession: vi.fn(),
}));

vi.mock("../../adapters/session-archive-api", async importOriginal => ({
  ...(await importOriginal<typeof import("../../adapters/session-archive-api")>()),
  listSessionDescendants: vi.fn(),
}));

vi.mock("../../adapters/session-later-api", () => ({
  moveSessionToLater: vi.fn(),
  resumeSessionFromLater: vi.fn(),
}));
import { moveSessionToLater, resumeSessionFromLater } from "../../adapters/session-later-api";

vi.mock("../../adapters/session-owner-api", async importOriginal => ({
  ...(await importOriginal<typeof import("../../adapters/session-owner-api")>()),
  fetchSessionById: vi.fn(),
}));
import { fetchSessionById } from "../../adapters/session-owner-api";

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => ({
    activeWorkspaceId: "ws_alpha",
    runtimeWorkspaceId: "ws_alpha",
    breadthMode: "focused",
    registeredWorkspaces: [{ id: "ws_alpha", name: "alpha" }],
  }),
}));

vi.mock("@/lib/user-feedback", () => ({ notifyUser: vi.fn() }));

import {
  cancelQueuedSessionPrompt,
  archiveSession,
  clearSessionConversation,
  clearSessionInputs,
  createSession,
  deleteSession,
  fetchSession,
  fetchSessionInputs,
  repairSession,
  renameSession,
  setSessionParent,
  clearSessionParent,
  promoteSessionInputToSteer,
  replaceSessionInput,
  rewindSession,
  sendSessionPrompt,
  stopSession,
  unarchiveSession,
} from "../../adapters/session-api";
import { SessionApiError, SessionGoalCommandError } from "../../adapters/session-api-errors";
import { toast } from "@compozy/ui";
import { notifyUser } from "@/lib/user-feedback";
import { withSessionProcessSummary } from "../../mocks/process-fixtures";
import { useSessionLifecycleActions } from "../use-session-lifecycle-actions";

beforeEach(() => {
  vi.spyOn(toast, "error").mockReturnValue("toast");
  vi.spyOn(toast, "success").mockReturnValue("toast");
  vi.spyOn(toast, "info").mockReturnValue("toast");
});
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
  board_column: "done",
  pull_request: null,
  pull_request_lookup: null,
  supervision: null,
  profile_id: "00000000000000000000000000",
  profile_name: "default",
  scope: "workspace",
  effective_workspace_access: "",
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
    vi.mocked(listSessionDescendants).mockResolvedValue([]);
    vi.clearAllMocks();
    sessionStore.trigger.allDraftsDiscarded();
    sessionStore.trigger.sessionInteractionRemoved({ sessionId: createdSession.id });
    vi.mocked(fetchSessionById).mockImplementation(async id => ({ ...createdSession, id }));
  });

  afterEach(() => {
    act(() => resetProfileViews());
    vi.restoreAllMocks();
  });

  it("useCreateSession seeds detail without replacing infinite list caches", async () => {
    act(() => setProfileView({ kind: "profile", profile: "open-design" }));
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
      "open-design"
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

  it("Should announce the project and profile after aggregate session creation", async () => {
    act(() => setProfileView({ kind: "aggregate" }));
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
      message: "Session started in alpha · default",
      tone: "success",
    });
  });

  it("Should file the session into the picked profile instead of the acting one", async () => {
    vi.mocked(createSession).mockResolvedValue(createdSession);
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const { result } = renderHook(() => useCreateSession(), {
      wrapper: createWrapper(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync({ workspace: WORKSPACE_ID, profile: "marketing" });
    });

    expect(createSession).toHaveBeenCalledWith({ workspace: WORKSPACE_ID }, "marketing");
  });

  it("useClearSessionConversation publishes an empty transcript after commit without touching drafts", async () => {
    let resolveClear!: (session: typeof createdSession) => void;
    const clearResult = new Promise<typeof createdSession>(resolve => {
      resolveClear = resolve;
    });
    vi.mocked(clearSessionConversation).mockReturnValue(clearResult);

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

    let mutation: Promise<typeof createdSession>;
    await act(async () => {
      mutation = result.current.mutateAsync(createdSession.id);
    });
    await waitFor(() => expect(clearSessionConversation).toHaveBeenCalled());
    expect(
      queryClient.getQueryData(sessionKeys.transcript(WORKSPACE_ID, createdSession.id))
    ).toEqual(transcriptCache());
    expect(queryClient.getQueryData(sessionKeys.history(WORKSPACE_ID, createdSession.id))).toEqual([
      { id: "turn-1" },
    ]);
    await act(async () => {
      resolveClear(createdSession);
      await mutation;
    });

    expect(clearSessionConversation).toHaveBeenCalledWith(WORKSPACE_ID, createdSession.id);
    expect(queryClient.getQueryData(sessionKeys.detail(WORKSPACE_ID, createdSession.id))).toEqual(
      createdSession
    );
    expect(
      queryClient.getQueryData(sessionKeys.transcript(WORKSPACE_ID, createdSession.id))
    ).toEqual({ pages: [], pageParams: [] });
    expect(queryClient.getQueryData(sessionKeys.history(WORKSPACE_ID, createdSession.id))).toEqual(
      []
    );
    expect(sessionStore.getSnapshot().context.drafts[createdSession.id]).toBe("keep me");
    expect(
      queryClient.getQueryState(sessionKeys.detail(WORKSPACE_ID, createdSession.id))?.isInvalidated
    ).toBe(true);
  });

  it("useClearSessionConversation preserves cached conversation on failure", async () => {
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

  it.each([false, true])(
    "useSessionRewind preserves its owner and fence across refetch failure (%s)",
    async failRefetch => {
      const owner = { ...createdSession, profile_name: "design-renamed" };
      vi.mocked(fetchSession).mockResolvedValue(owner);
      vi.mocked(rewindSession).mockResolvedValue({
        rewind: { draft_text: "Return to this request." },
        session: owner,
      });
      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
      });
      queryClient.setQueryData(sessionKeys.detail(WORKSPACE_ID, createdSession.id), createdSession);
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
      if (failRefetch) {
        fetchInfiniteQuery.mockRejectedValueOnce(new Error("refetch failed"));
        await act(async () => {
          await expect(
            result.current.mutateAsync({
              idempotencyKey: "rewind-idempotency-001",
              messageId: "message-001",
              sessionId: createdSession.id,
              signal: controller.signal,
            })
          ).rejects.toThrow("refetch failed");
        });
        expect(
          queryClient.getQueryData(sessionKeys.transcript(WORKSPACE_ID, createdSession.id))
        ).toBeUndefined();
      }

      await act(async () => {
        await result.current.mutateAsync({
          idempotencyKey: "rewind-idempotency-001",
          messageId: "message-001",
          sessionId: createdSession.id,
          signal: controller.signal,
        });
      });

      if (failRefetch)
        expect(vi.mocked(rewindSession).mock.calls[1]?.[3]).toEqual(
          vi.mocked(rewindSession).mock.calls[0]?.[3]
        );
      expect(rewindSession).toHaveBeenCalledWith(
        WORKSPACE_ID,
        createdSession.id,
        owner.profile_name,
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
        owner
      );
      expect(
        queryClient.getQueryData(sessionKeys.transcript(WORKSPACE_ID, createdSession.id))
      ).toBeUndefined();
      expect(fetchInfiniteQuery).toHaveBeenCalledWith(
        expect.objectContaining({
          queryKey: sessionKeys.transcript(WORKSPACE_ID, createdSession.id),
        })
      );
    }
  );

  // Invariant: a definite rewind refusal releases its stale fence, so the retry reads the current
  // transcript; an unknown outcome replays the exact request.
  it.each([
    [409, true],
    [503, false],
  ])(
    "useSessionRewind retried after a %s answer rereads the fence: %s",
    async (status, rereads) => {
      vi.mocked(fetchSession).mockResolvedValue(createdSession);
      vi.mocked(rewindSession)
        .mockRejectedValueOnce(new SessionApiError("rewind failed", status, createdSession.id))
        .mockResolvedValueOnce({
          rewind: { draft_text: "Return to this request." },
          session: createdSession,
        });
      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
      });
      vi.spyOn(queryClient, "fetchInfiniteQuery").mockResolvedValue(
        transcriptCache("message-rewound")
      );
      const setFence = (max_sequence: number) => {
        const cached = transcriptCache("message-001");
        queryClient.setQueryData(sessionKeys.transcript(WORKSPACE_ID, createdSession.id), {
          ...cached,
          pages: [{ ...cached.pages[0]!, epoch: 3, generation: 7, max_sequence }],
        });
      };
      setFence(19);
      const { result } = renderHook(() => useSessionRewind(WORKSPACE_ID), {
        wrapper: createWrapper(queryClient),
      });
      const variables = {
        idempotencyKey: "rewind-idempotency-001",
        messageId: "message-001",
        sessionId: createdSession.id,
        signal: new AbortController().signal,
      };
      await act(async () => {
        await expect(result.current.mutateAsync(variables)).rejects.toThrow("rewind failed");
      });
      setFence(23);
      await act(async () => {
        await result.current.mutateAsync(variables);
      });

      expect(vi.mocked(rewindSession).mock.calls[1]?.[3]).toMatchObject({
        expected_max_sequence: rereads ? 23 : 19,
        idempotency_key: "rewind-idempotency-001",
      });
    }
  );

  // Invariant: deletion ownership spans async host reconciliation, scoped to the target ID, and
  // a settled deletion stays attributed to the operator so its window retires quietly (#952).
  // Owner: session mutation cache; canonical suite: session actions.
  it.each([null, undefined, ""])(
    "deletes a User-owned session (%s) without touching project caches",
    async workspaceId => {
      const target = { ...createdSession, workspace_id: workspaceId } as SessionPayload;
      vi.mocked(deleteSession).mockResolvedValue(undefined);
      const client = new QueryClient();
      const userKey = sessionKeys.detail("@user", target.id);
      const projectKey = sessionKeys.detail(WORKSPACE_ID, target.id);
      const projectListKey = sessionKeys.workspaceLists(WORKSPACE_ID);
      client.setQueryData(projectListKey, { sessions: [createdSession] });
      client.setQueryData(userKey, target);
      client.setQueryData(projectKey, createdSession);
      const { result } = renderHook(() => useDeleteSession(), { wrapper: createWrapper(client) });
      await act(async () => {
        await result.current.mutateAsync(target);
      });
      expect(deleteSession).toHaveBeenCalledWith("@user", target.id, target.profile_name);
      expect(client.getQueryData(userKey)).toBeUndefined();
      expect(client.getQueryData(projectKey)).toEqual(createdSession);
      expect(client.getQueryState(projectKey)?.isInvalidated).toBe(false);
      expect(client.getQueryState(projectListKey)?.isInvalidated).toBe(false);
    }
  );

  it.each([null, undefined, ""])(
    "updates User lifecycle actions (%s) and refreshes their owner",
    async workspaceId => {
      const target = { ...createdSession, workspace_id: workspaceId } as SessionPayload;
      vi.mocked(renameSession).mockResolvedValue(target);
      vi.mocked(stopSession).mockResolvedValue({
        session_id: target.id,
        status: "stopped",
        state: "stopped",
        verified: true,
        escalated: false,
      });
      vi.mocked(archiveSession).mockResolvedValue(target);
      vi.mocked(unarchiveSession).mockResolvedValue(target);
      const client = new QueryClient();
      const userKey = sessionKeys.workspaceLists("@user");
      const aggregateKey = sessionKeys.workspaceLists("");
      const otherKey = sessionKeys.workspaceLists(WORKSPACE_ID);
      const { result } = renderHook(
        () => ({
          rename: useRenameSession(),
          stop: useStopSession(),
          archive: useArchiveSession(),
          restore: useUnarchiveSession(),
        }),
        { wrapper: createWrapper(client) }
      );
      for (const action of ["rename", "stop", "archive", "restore"] as const) {
        client.setQueryData(userKey, { sessions: [target] });
        client.setQueryData(aggregateKey, { sessions: [target] });
        client.setQueryData(otherKey, { sessions: [createdSession] });
        await act(async () => {
          await result.current[action].mutateAsync({ ...target, name: "Updated" });
        });
        expect(client.getQueryState(userKey)?.isInvalidated).toBe(true);
        expect(client.getQueryState(aggregateKey)?.isInvalidated).toBe(true);
        expect(client.getQueryState(otherKey)?.isInvalidated).toBe(false);
      }
      for (const adapter of [renameSession, stopSession, archiveSession, unarchiveSession]) {
        expect(vi.mocked(adapter).mock.calls[0]?.slice(0, 3)).toEqual([
          "@user",
          target.id,
          target.profile_name,
        ]);
      }
    }
  );

  it("tracks deletion until the host finishes retiring its windows", async () => {
    let retire!: () => void;
    const onDeleteSuccess = () =>
      new Promise<void>(resolve => {
        retire = resolve;
      });
    vi.mocked(deleteSession).mockResolvedValue(undefined);
    const client = new QueryClient();
    const { result } = renderHook(
      () => ({
        remove: useDeleteSession({ onDeleteSuccess }),
        pending: useSessionDeletionPending(createdSession.id),
        otherPending: useSessionDeletionPending("another-session"),
        deleted: useSessionDeletedByOperator(createdSession.id),
        otherDeleted: useSessionDeletedByOperator("another-session"),
      }),
      { wrapper: createWrapper(client) }
    );
    act(() => result.current.remove.mutate(createdSession));
    await waitFor(() => expect(retire).toBeTypeOf("function"));
    expect(result.current.pending).toBe(true);
    expect(result.current.otherPending).toBe(false);
    expect(result.current.deleted).toBe(false);
    act(() => retire());
    await waitFor(() => expect(result.current.pending).toBe(false));
    expect(result.current.deleted).toBe(true);
    expect(result.current.otherDeleted).toBe(false);
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
    const globalKey = sessionKeys.list({ all_workspaces: true });
    const otherWorkspaceKey = sessionKeys.list({ workspace_id: "other-workspace" });
    for (const key of [globalKey, otherWorkspaceKey, sessionKeys.attentionSummary()]) {
      queryClient.setQueryData(key, { sessions: [createdSession] });
    }

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
    const file = new File(["draft"], "draft.txt");
    recordComposerDraftAttachments(createdSession.id, [file]);
    retainComposerDraftUpload(WORKSPACE_ID, createdSession.id, file, new Promise(() => {}));

    const { result } = renderHook(() => useDeleteSession({ onDeleteSuccess }), {
      wrapper: createWrapper(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync(createdSession);
    });

    expect(composerDraftAttachmentFiles(createdSession.id)).toEqual([]);
    expect(retainedComposerDraftUpload(WORKSPACE_ID, createdSession.id, file)).toBeUndefined();
    expect(deleteSession).toHaveBeenCalledWith(
      WORKSPACE_ID,
      createdSession.id,
      createdSession.profile_name
    );
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
    expect(queryClient.getQueryState(globalKey)?.isInvalidated).toBe(true);
    expect(queryClient.getQueryState(sessionKeys.attentionSummary())?.isInvalidated).toBe(true);
    expect(queryClient.getQueryState(otherWorkspaceKey)?.isInvalidated).toBe(false);

    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: sessionKeys.workspaceLists(WORKSPACE_ID),
    });
    expect(onDeleteSuccess.mock.invocationCallOrder[0]).toBeLessThan(
      invalidateSpy.mock.invocationCallOrder[0] ?? Number.POSITIVE_INFINITY
    );
  });

  // Invariant: deletion aborts concurrent queue reads; failed deletion re-reads durable state.
  // Owner: session mutation/query integration; HTTP transport is the unit I/O boundary.
  it("Should fence an in-flight queue read during deletion and reconcile after failure", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    let queueSignal: AbortSignal | undefined;
    let finishStaleRead!: (value: SessionInputsResponse) => void;
    let failDelete!: (error: Error) => void;
    vi.mocked(fetchSessionInputs)
      .mockImplementationOnce((_workspace, _id, signal) => {
        queueSignal = signal;
        return new Promise(resolve => {
          finishStaleRead = resolve;
        });
      })
      .mockResolvedValue({ inputs: [] });
    vi.mocked(deleteSession).mockImplementation(
      () =>
        new Promise((_resolve, reject) => {
          failDelete = reject;
        })
    );
    const { result } = renderHook(
      () => ({
        queue: useSessionInputs(WORKSPACE_ID, createdSession.id, { sessionState: "active" }),
        deletion: useDeleteSession(),
      }),
      { wrapper: createWrapper(queryClient) }
    );
    await waitFor(() => expect(queueSignal).toBeDefined());
    act(() => result.current.deletion.mutate(createdSession));
    await waitFor(() => expect(deleteSession).toHaveBeenCalled());
    expect(queueSignal?.aborted).toBe(true);
    await act(async () => finishStaleRead({ inputs: [queuedInput] }));
    expect(result.current.queue.data?.inputs).not.toEqual([queuedInput]);
    expect(fetchSessionInputs).toHaveBeenCalledTimes(1);
    act(() => failDelete(new Error("Goal settlement failed")));
    await waitFor(() => expect(result.current.deletion.isError).toBe(true));
    await waitFor(() => expect(result.current.queue.data?.inputs).toEqual([]));
    expect(fetchSessionInputs).toHaveBeenCalledTimes(2);
    expect(
      sessionStore.getSnapshot().context.liveTailSuppressions[createdSession.id]
    ).toBeUndefined();
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
      await expect(result.current.mutateAsync(createdSession)).rejects.toThrow("delete failed");
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

  it("useDeleteSession resolves before catalog reconciliation settles", async () => {
    vi.mocked(deleteSession).mockResolvedValue(undefined);
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    let releaseInvalidation!: () => void;
    const invalidation = new Promise<void>(resolve => {
      releaseInvalidation = resolve;
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries").mockReturnValue(invalidation);
    const { result } = renderHook(() => useDeleteSession(), {
      wrapper: createWrapper(queryClient),
    });

    let settled = false;
    let mutation!: Promise<void>;
    act(() => {
      mutation = result.current.mutateAsync(createdSession).then(() => {
        settled = true;
      });
    });

    await waitFor(() => expect(invalidateSpy).toHaveBeenCalled());
    await waitFor(() => expect(settled).toBe(true));
    releaseInvalidation();
    await act(async () => {
      await mutation;
    });
  });

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
    const detailKey = sessionKeys.detail(WORKSPACE_ID, createdSession.id);
    const catalogKey = sessionKeys.workspaceLists(WORKSPACE_ID);
    queryClient.setQueryData(detailKey, createdSession);
    queryClient.setQueryData(catalogKey, { sessions: [createdSession] });

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
    expect(queryClient.getQueryState(detailKey)?.isInvalidated).toBe(true);
    expect(queryClient.getQueryState(catalogKey)?.isInvalidated).toBe(true);
  });

  it("archive and unarchive use the row owner across workspace and profile selection", async () => {
    const target = { ...createdSession, workspace_id: "ws-other", profile_name: "design" };
    vi.mocked(archiveSession).mockResolvedValue({
      ...target,
      archived_at: "2026-08-04T12:00:00Z",
    });
    vi.mocked(unarchiveSession).mockResolvedValue(target);
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const detailKey = sessionKeys.detail(target.workspace_id, target.id);
    const catalogKey = sessionKeys.workspaceLists(target.workspace_id);
    const selectedWorkspaceKey = sessionKeys.workspaceLists(WORKSPACE_ID);
    queryClient.setQueryData(selectedWorkspaceKey, { sessions: [createdSession] });
    const { result } = renderHook(
      () => ({ archive: useArchiveSession(), unarchive: useUnarchiveSession() }),
      { wrapper: createWrapper(queryClient) }
    );

    for (const action of [result.current.archive, result.current.unarchive]) {
      queryClient.setQueryData(detailKey, target);
      queryClient.setQueryData(catalogKey, { sessions: [target] });
      await act(async () => {
        await action.mutateAsync(target);
      });
      expect(queryClient.getQueryState(detailKey)?.isInvalidated).toBe(true);
      expect(queryClient.getQueryState(catalogKey)?.isInvalidated).toBe(true);
      expect(queryClient.getQueryState(selectedWorkspaceKey)?.isInvalidated).toBe(false);
    }

    expect(archiveSession).toHaveBeenCalledWith(
      target.workspace_id,
      target.id,
      target.profile_name,
      expect.any(AbortSignal),
      false
    );
    expect(unarchiveSession).toHaveBeenCalledWith(
      target.workspace_id,
      target.id,
      target.profile_name,
      expect.any(AbortSignal)
    );
  });

  it("aborts an in-flight archive request when its hook unmounts", async () => {
    let requestSignal: AbortSignal | undefined;
    vi.mocked(archiveSession).mockImplementation(
      (_workspaceId, _sessionId, _profile, signal) =>
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
      mutation = result.current.mutateAsync(createdSession);
    });
    await waitFor(() => expect(requestSignal).toBeInstanceOf(AbortSignal));
    unmount();

    expect(requestSignal?.aborted).toBe(true);
    await expect(mutation).rejects.toThrow("Aborted");
  });

  // Invariant: children are a daemon-counted, explicit choice; no children keeps one click,
  // and a selected child is never processed twice.
  // Owner: lifecycle confirmation hook; canonical suite: use-session-actions.
  describe("child sessions", () => {
    const child = { ...createdSession, id: "sess-child", name: "Child" };
    const treeClient = () => new QueryClient({ defaultOptions: { queries: { retry: false } } });

    beforeEach(() => {
      vi.mocked(fetchSessionById).mockResolvedValue(createdSession);
      vi.mocked(archiveSession).mockResolvedValue(createdSession);
      vi.mocked(deleteSession).mockResolvedValue(undefined);
    });

    it("archives in one click without children", async () => {
      vi.mocked(listSessionDescendants).mockResolvedValue([]);
      const { result } = renderHook(() => useSessionLifecycleActions(), {
        wrapper: createWrapper(treeClient()),
      });
      act(() => result.current.actions.onArchive(createdSession));
      await waitFor(() => expect(archiveSession).toHaveBeenCalledTimes(1));
      expect(result.current.processDialog.open).toBe(false);
      expect(vi.mocked(archiveSession).mock.calls[0]).toHaveLength(5);
    });

    it("asks with children checked and sends the answer", async () => {
      vi.mocked(listSessionDescendants).mockResolvedValue([child]);
      const { result } = renderHook(() => useSessionLifecycleActions(), {
        wrapper: createWrapper(treeClient()),
      });
      act(() => result.current.actions.onArchive(createdSession));
      await waitFor(() => expect(result.current.processDialog.open).toBe(true));
      expect(archiveSession).not.toHaveBeenCalled();
      expect(result.current.processDialog.childSessions).toMatchObject({
        count: 1,
        included: true,
      });
      act(() => result.current.processDialog.onChoose());
      await waitFor(() => expect(archiveSession).toHaveBeenCalledTimes(1));
      expect(vi.mocked(archiveSession).mock.calls[0]?.[6]).toBe(true);
    });

    it("leaves children out when the operator unchecks them", async () => {
      vi.mocked(listSessionDescendants).mockResolvedValue([child]);
      const { result } = renderHook(() => useSessionLifecycleActions(), {
        wrapper: createWrapper(treeClient()),
      });
      act(() => result.current.actions.onArchive(createdSession));
      await waitFor(() => expect(result.current.processDialog.open).toBe(true));
      act(() => result.current.processDialog.childSessions?.onIncludedChange(false));
      act(() => result.current.processDialog.onChoose());
      await waitFor(() => expect(archiveSession).toHaveBeenCalledTimes(1));
      expect(vi.mocked(archiveSession).mock.calls[0]).toHaveLength(5);
    });

    it("starts delete unchecked and covers a selected child through its parent", async () => {
      vi.mocked(listSessionDescendants).mockImplementation(async (_workspace, id) =>
        id === createdSession.id ? [child] : []
      );
      vi.mocked(fetchSessionById).mockImplementation(async (id: string) =>
        id === child.id ? child : createdSession
      );
      const { result } = renderHook(() => useSessionLifecycleActions(), {
        wrapper: createWrapper(treeClient()),
      });
      act(() => result.current.actions.onDeleteMany?.([child, createdSession]));
      await waitFor(() =>
        expect(listSessionDescendants).toHaveBeenCalledWith(
          expect.anything(),
          createdSession.id,
          expect.anything(),
          expect.anything()
        )
      );
      // Both targets are selected, so no child is left to ask about.
      await waitFor(() => expect(result.current.deleteDialog.open).toBe(true));
      expect(result.current.deleteDialog.childSessions).toBeUndefined();
      act(() => result.current.deleteDialog.onConfirm());
      await waitFor(() => expect(result.current.deleteDialog.open).toBe(false));
      expect(vi.mocked(deleteSession).mock.calls.map(call => call[1]).sort()).toEqual(
        [child.id, createdSession.id].sort()
      );
    });

    it("deletes a tree once when the opt-in covers a selected child", async () => {
      const grandchild = { ...createdSession, id: "sess-grandchild" };
      vi.mocked(listSessionDescendants).mockImplementation(async (_workspace, id) =>
        id === createdSession.id ? [child, grandchild] : []
      );
      vi.mocked(fetchSessionById).mockImplementation(async (id: string) =>
        id === child.id ? child : createdSession
      );
      const { result } = renderHook(() => useSessionLifecycleActions(), {
        wrapper: createWrapper(treeClient()),
      });
      act(() => result.current.actions.onDeleteMany?.([child, createdSession]));
      await waitFor(() => expect(result.current.deleteDialog.childSessions?.count).toBe(1));
      expect(result.current.deleteDialog.childSessions?.included).toBe(false);
      act(() => result.current.deleteDialog.childSessions?.onIncludedChange(true));
      act(() => result.current.deleteDialog.onConfirm());
      await waitFor(() => expect(result.current.deleteDialog.open).toBe(false));
      expect(deleteSession).toHaveBeenCalledTimes(1);
      expect(vi.mocked(deleteSession).mock.calls[0]?.[1]).toBe(createdSession.id);
      expect(vi.mocked(deleteSession).mock.calls[0]?.[5]).toBe(true);
    });
  });

  // Invariant: deletion choices follow newer detail evidence, not a frozen catalog row.
  // Owner: lifecycle confirmation hook; canonical suite: use-session-actions.
  it("updates an open delete confirmation when the detail cache reports running processes", async () => {
    vi.mocked(fetchSessionById).mockImplementation(() => new Promise(() => {}));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { result } = renderHook(() => useSessionLifecycleActions(), {
      wrapper: createWrapper(client),
    });
    act(() => result.current.actions.onDelete(createdSession));
    const current = withSessionProcessSummary(createdSession);
    act(() =>
      client.setQueryData(
        sessionScopedDetailOptions(createdSession.id, { profile: createdSession.profile_name })
          .queryKey,
        current
      )
    );
    await waitFor(() =>
      expect(result.current.deleteDialog.session?.process_summary).toEqual(current.process_summary)
    );
    expect(result.current.deleteDialog.sessions?.[0]?.process_summary).toEqual(
      current.process_summary
    );
    client.clear();
  });

  // Invariant: old zero-process details cannot suppress choices from a newer list/read.
  // Owner: lifecycle confirmation hook; canonical suite: use-session-actions.
  it("refreshes old details before archiving and preserves the newer delete row while reading", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const key = sessionScopedDetailOptions(createdSession.id, {
      profile: createdSession.profile_name,
    }).queryKey;
    const running = withSessionProcessSummary(createdSession);
    client.setQueryData(key, createdSession, { updatedAt: 1 });
    let resolveRead!: (session: SessionPayload) => void;
    vi.mocked(fetchSessionById).mockImplementation(
      () =>
        new Promise(resolve => {
          resolveRead = resolve;
        })
    );
    const { result, unmount } = renderHook(() => useSessionLifecycleActions(), {
      wrapper: createWrapper(client),
    });
    act(() => result.current.actions.onDelete(running));
    expect(result.current.deleteDialog.session?.process_summary).toEqual(running.process_summary);
    await act(async () => resolveRead(running));
    act(() => result.current.deleteDialog.onOpenChange(false));
    client.setQueryData(key, createdSession, { updatedAt: 1 });
    act(() => {
      void result.current.actions.onArchive(running);
    });
    expect(archiveSession).not.toHaveBeenCalled();
    await act(async () => resolveRead(running));
    await waitFor(() => expect(result.current.processDialog.processCount).toBe(3));
    expect(archiveSession).not.toHaveBeenCalled();
    unmount();
    client.clear();
  });

  it("does not archive when the current process detail cannot be read", async () => {
    vi.mocked(fetchSessionById).mockRejectedValue(new Error("Detail unavailable"));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { result, unmount } = renderHook(() => useSessionLifecycleActions(), {
      wrapper: createWrapper(client),
    });
    await act(async () => {
      await result.current.actions.onArchive(createdSession);
    });
    expect(archiveSession).not.toHaveBeenCalled();
    expect(toast.error).toHaveBeenCalledWith("Detail unavailable");
    unmount();
    client.clear();
  });

  it("archives a live session in one action by asking the daemon to stop it first", async () => {
    vi.mocked(archiveSession).mockImplementation(async (_workspaceId, id) => ({
      ...createdSession,
      id,
      state: "stopped",
      archived_at: "2026-08-04T12:00:00Z",
    }));
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const { result } = renderHook(() => useSessionLifecycleActions(), {
      wrapper: createWrapper(queryClient),
    });

    act(() => result.current.actions.onArchive(createdSession));

    await waitFor(() =>
      expect(archiveSession).toHaveBeenCalledWith(
        createdSession.workspace_id,
        createdSession.id,
        createdSession.profile_name,
        expect.any(AbortSignal),
        true
      )
    );
  });

  // Invariant: archive asks what happens to running processes only while some run, and sends
  // the answer; a single choice covers every session of a bulk archive.
  it("asks before archiving sessions that left processes running and sends the answer", async () => {
    vi.mocked(archiveSession).mockImplementation(async (_workspaceId, id) => ({
      ...createdSession,
      id,
      state: "stopped",
      archived_at: "2026-08-04T12:00:00Z",
    }));
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const { result } = renderHook(() => useSessionLifecycleActions(), {
      wrapper: createWrapper(queryClient),
    });
    const running = withSessionProcessSummary(createdSession);
    vi.mocked(fetchSessionById).mockImplementation(async id => ({ ...running, id }));

    queryClient.setQueryData(
      sessionScopedDetailOptions(createdSession.id, { profile: createdSession.profile_name })
        .queryKey,
      running
    );
    act(() => result.current.actions.onArchive(createdSession));
    expect(archiveSession).not.toHaveBeenCalled();
    await waitFor(() =>
      expect(result.current.processDialog).toMatchObject({
        open: true,
        action: "archive",
        sessionCount: 1,
        processCount: 3,
        ports: [5173, 8080, 9090],
      })
    );

    const readsBeforeSecondAction = vi.mocked(fetchSessionById).mock.calls.length;
    await act(async () => {
      result.current.actions.onArchive({ ...running, id: "sess_other" });
    });
    expect(fetchSessionById).toHaveBeenCalledTimes(readsBeforeSecondAction);
    expect(archiveSession).not.toHaveBeenCalled();
    act(() => result.current.processDialog.onChoose("terminate"));
    expect(result.current.processDialog.open).toBe(false);
    await waitFor(() =>
      expect(archiveSession).toHaveBeenCalledWith(
        createdSession.workspace_id,
        createdSession.id,
        createdSession.profile_name,
        expect.any(AbortSignal),
        true,
        "terminate"
      )
    );

    vi.mocked(archiveSession).mockClear();
    const second = { ...running, id: "sess-second" };
    act(() => result.current.actions.onArchiveMany?.([running, second]));
    await waitFor(() =>
      expect(result.current.processDialog).toMatchObject({ sessionCount: 2, processCount: 6 })
    );
    act(() => result.current.processDialog.onChoose("keep"));
    await waitFor(() => expect(archiveSession).toHaveBeenCalledTimes(2));
    for (const call of vi.mocked(archiveSession).mock.calls) expect(call[5]).toBe("keep");
  });

  // Invariant (#1052): a host hears every archive the daemon confirmed — single and batched —
  // and never a failed one, so the board drawer closes only on a real archive.
  it("reports each confirmed archive, single and batched, through onArchiveSuccess", async () => {
    vi.mocked(archiveSession).mockImplementation(async (_workspaceId, id) => {
      if (id === "sess-refused") throw new Error("Archive is unavailable");
      return { ...createdSession, id, state: "stopped", archived_at: "2026-08-04T12:00:00Z" };
    });
    const onArchiveSuccess = vi.fn();
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const { result } = renderHook(() => useSessionLifecycleActions({ onArchiveSuccess }), {
      wrapper: createWrapper(queryClient),
    });

    act(() => result.current.actions.onArchive(createdSession));
    await waitFor(() => expect(onArchiveSuccess).toHaveBeenCalledExactlyOnceWith(createdSession));

    const batch = [
      { ...createdSession, id: "sess-batch" },
      { ...createdSession, id: "sess-refused" },
    ];
    await act(async () => result.current.actions.onArchiveMany?.(batch));
    await waitFor(() => expect(onArchiveSuccess).toHaveBeenCalledTimes(2));
    expect(onArchiveSuccess).toHaveBeenLastCalledWith(batch[0]);
  });

  // Invariant: Move to Later always asks first and sends only what the operator typed; the
  // confirmed move is reported like an archive. Owning layer: lifecycle actions (this suite).
  it("moves a session to Later through its dialog and reports each confirmed move", async () => {
    vi.mocked(moveSessionToLater).mockImplementation(async (_workspaceId, id) => ({
      ...createdSession,
      id,
      state: "stopped",
      later: { parked_at: "2026-08-04T12:00:00Z", pending_messages: 0 },
    }));
    const onLaterSuccess = vi.fn();
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const { result } = renderHook(() => useSessionLifecycleActions({ onLaterSuccess }), {
      wrapper: createWrapper(queryClient),
    });

    act(() => result.current.actions.onMoveToLater?.(createdSession));
    expect(moveSessionToLater).not.toHaveBeenCalled();
    expect(result.current.laterDialog).toMatchObject({ open: true, sessions: [createdSession] });

    act(() =>
      result.current.laterDialog.onConfirm({
        reason: "After the release",
        return_at: "2026-08-10T09:00:00Z",
      })
    );
    await waitFor(() => expect(onLaterSuccess).toHaveBeenCalledExactlyOnceWith(createdSession));
    expect(moveSessionToLater).toHaveBeenCalledWith(
      WORKSPACE_ID,
      createdSession.id,
      "default",
      { reason: "After the release", return_at: "2026-08-10T09:00:00Z" },
      expect.any(AbortSignal)
    );
    expect(result.current.laterDialog.open).toBe(false);
  });

  it("skips parked and subagent sessions in a bulk Move to Later and reports the batch once", async () => {
    vi.mocked(moveSessionToLater).mockImplementation(async (_workspaceId, id) => ({
      ...createdSession,
      id,
      state: "stopped",
      later: { parked_at: "2026-08-04T12:00:00Z", pending_messages: 0 },
    }));
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const { result } = renderHook(() => useSessionLifecycleActions(), {
      wrapper: createWrapper(queryClient),
    });
    const parked = {
      ...createdSession,
      id: "sess-parked",
      later: { parked_at: "2026-08-01T12:00:00Z", pending_messages: 1 },
    };
    const child = {
      ...createdSession,
      id: "sess-child",
      lineage: {
        kind: "spawn" as const,
        spawn_role: "subagent" as const,
        parent_session_id: createdSession.id,
        root_session_id: createdSession.id,
        spawn_depth: 1,
        auto_stop_on_parent: false,
        notify_creator: false,
        spawn_budget: { max_children: 0, max_depth: 0, ttl_seconds: 0 },
        permission_policy: { tools: [], skills: [], mcp_servers: [], workspace_paths: [] },
      },
    };

    act(() => result.current.actions.onMoveToLaterMany?.([createdSession, parked, child]));
    expect(result.current.laterDialog.sessions).toEqual([createdSession]);
    act(() => result.current.laterDialog.onConfirm({}));

    await waitFor(() =>
      expect(toast.success).toHaveBeenCalledWith("1 session moved to Later", {
        description: "2 skipped: 1 already in Later, 1 child session follows its parent.",
      })
    );
    expect(moveSessionToLater).toHaveBeenCalledTimes(1);
    expect(result.current.laterDialog.open).toBe(false);
  });

  // Invariant: a partly failed bulk Move to Later keeps its dialog open on the sessions
  // still to move, so a retry reuses the same form; it closes once every move landed.
  it("keeps the Move to Later dialog on the failed sessions after a partial bulk failure", async () => {
    const second = { ...createdSession, id: "sess-second" };
    vi.mocked(moveSessionToLater).mockImplementation(async (_workspaceId, id) => {
      if (id === second.id) throw new Error("stop timed out");
      return {
        ...createdSession,
        id,
        state: "stopped",
        later: { parked_at: "2026-08-04T12:00:00Z", pending_messages: 0 },
      };
    });
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const { result } = renderHook(() => useSessionLifecycleActions(), {
      wrapper: createWrapper(queryClient),
    });

    act(() => result.current.actions.onMoveToLaterMany?.([createdSession, second]));
    act(() => result.current.laterDialog.onConfirm({ reason: "After the release" }));

    await waitFor(() => expect(toast.error).toHaveBeenCalled());
    await waitFor(() => expect(result.current.laterDialog.isMoving).toBe(false));
    expect(result.current.laterDialog).toMatchObject({ open: true, sessions: [second] });

    vi.mocked(moveSessionToLater).mockClear();
    vi.mocked(moveSessionToLater).mockImplementation(async (_workspaceId, id) => ({
      ...second,
      id,
      state: "stopped",
      later: { parked_at: "2026-08-04T12:05:00Z", pending_messages: 0 },
    }));
    act(() => result.current.laterDialog.onConfirm({ reason: "After the release" }));

    await waitFor(() => expect(result.current.laterDialog.open).toBe(false));
    expect(moveSessionToLater).toHaveBeenCalledExactlyOnceWith(
      WORKSPACE_ID,
      second.id,
      "default",
      { reason: "After the release" },
      expect.any(AbortSignal)
    );
  });

  it("brings a parked session back from Later at once, without a dialog", async () => {
    const parked = {
      ...createdSession,
      state: "stopped" as const,
      later: { parked_at: "2026-08-01T12:00:00Z", pending_messages: 0 },
    };
    vi.mocked(resumeSessionFromLater).mockResolvedValue({ ...parked, later: null });
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const { result } = renderHook(() => useSessionLifecycleActions(), {
      wrapper: createWrapper(queryClient),
    });

    act(() => result.current.actions.onBackFromLater?.(parked));
    await waitFor(() =>
      expect(resumeSessionFromLater).toHaveBeenCalledWith(
        WORKSPACE_ID,
        parked.id,
        "default",
        {},
        expect.any(AbortSignal)
      )
    );
    expect(result.current.laterDialog.open).toBe(false);
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

  // Invariant: an unreadable detail cannot authorize deletion from a stale catalog row.
  // Owning layer: lifecycle actions; canonical suite: this shared action-hook suite.
  it("Should keep deletion blocked when the current detail cannot be read", async () => {
    vi.mocked(fetchSessionById).mockRejectedValue(new Error("Detail unavailable"));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { result, unmount } = renderHook(() => useSessionLifecycleActions(), {
      wrapper: createWrapper(client),
    });
    act(() => result.current.actions.onDelete(createdSession));
    act(() => result.current.deleteDialog.onConfirm());
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("Detail unavailable"));
    expect(deleteSession).not.toHaveBeenCalled();
    expect(result.current.deleteDialog.open).toBe(true);
    unmount();
    client.clear();
  });

  // Invariant: dismissal or replacement revokes an in-flight delete confirmation.
  // Owning layer/canonical suite: lifecycle actions; only detail adapter I/O is deferred.
  it.each([false, true])(
    "Should discard a pending delete read after dismissal with replacement=%s",
    async replace => {
      let resolveDetail!: (session: SessionPayload) => void;
      const detail = new Promise<SessionPayload>(resolve => {
        resolveDetail = resolve;
      });
      vi.mocked(fetchSessionById).mockReturnValue(detail);
      const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
      const { result, unmount } = renderHook(() => useSessionLifecycleActions(), {
        wrapper: createWrapper(client),
      });
      act(() => result.current.actions.onDelete(createdSession));
      act(() => result.current.deleteDialog.onConfirm());
      await waitFor(() => expect(fetchSessionById).toHaveBeenCalled());
      act(() => result.current.deleteDialog.onOpenChange(false));
      if (replace)
        act(() => result.current.actions.onDelete({ ...createdSession, id: "replacement" }));
      await act(async () => {
        resolveDetail(createdSession);
        await detail;
      });
      expect(deleteSession).not.toHaveBeenCalled();
      expect(result.current.deleteDialog.open).toBe(replace);
      unmount();
      client.clear();
    }
  );

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
      createdSession.profile_name,
      { name: "Release review" },
      expect.any(AbortSignal)
    );

    act(() => result.current.renameDialog.onOpenChange(false));
    expect(result.current.renameDialog.open).toBe(true);

    await act(async () => resolveRename({ ...createdSession, name: "Release review" }));
    await waitFor(() => expect(result.current.renameDialog.open).toBe(false));
    expect(result.current.renameDialog.session).toBeNull();
  });

  // Invariant: a drop adopts the row's session under its new parent through the
  // session's own profile, and the confirmation's Undo makes it top-level again.
  it("adopts a session under a parent and undoes it from the toast", async () => {
    const parent = { ...createdSession, id: "sess-parent", name: "Parent" };
    const owned = { ...createdSession, profile_name: "work" };
    vi.mocked(setSessionParent).mockResolvedValue(owned);
    vi.mocked(clearSessionParent).mockResolvedValue(owned);
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries").mockResolvedValue(undefined);
    const { result } = renderHook(() => useSessionLifecycleActions(), {
      wrapper: createWrapper(queryClient),
    });

    act(() => result.current.actions.onAdopt?.(owned, parent));
    await waitFor(() =>
      expect(setSessionParent).toHaveBeenCalledWith(WORKSPACE_ID, owned.id, "work", "sess-parent")
    );
    await waitFor(() => expect(toast.success).toHaveBeenCalled());
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: sessionKeys.workspaceLists(WORKSPACE_ID),
    });
    const options = vi.mocked(toast.success).mock.calls.at(-1)?.[1] as {
      action: { label: string; onClick: () => void };
    };
    expect(options.action.label).toBe("Undo");
    act(() => options.action.onClick());
    await waitFor(() =>
      expect(clearSessionParent).toHaveBeenCalledWith(WORKSPACE_ID, owned.id, "work")
    );
  });

  it("updates the detail cache and invalidates session views after rename", async () => {
    const renamed = { ...createdSession, name: "Release review" };
    vi.mocked(renameSession).mockResolvedValue(renamed);
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const catalogKey = sessionKeys.workspaceLists(WORKSPACE_ID);
    queryClient.setQueryData(catalogKey, { sessions: [createdSession] });
    const { result } = renderHook(() => useRenameSession(), {
      wrapper: createWrapper(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync({ ...createdSession, name: "Release review" });
    });

    expect(queryClient.getQueryData(sessionKeys.detail(WORKSPACE_ID, createdSession.id))).toEqual(
      renamed
    );
    expect(
      queryClient.getQueryState(sessionKeys.detail(WORKSPACE_ID, createdSession.id))?.isInvalidated
    ).toBe(true);
    expect(queryClient.getQueryState(catalogKey)?.isInvalidated).toBe(true);
  });

  it.each(["started", "error"] as const)(
    "publishes scoped Goal feedback for a busy %s result while retaining rejection semantics",
    async outcome => {
      const goal: SessionGoalCommandResult = {
        outcome,
        reason_code: outcome === "error" ? "goal_replace_required" : null,
        replaced_run_id: null,
        snapshot: {
          run_id: "run-current",
          node_id: "goal",
          objective: "Keep current work",
          origin_session_id: createdSession.id,
          bound_session_id: createdSession.id,
          status: "active",
          run_status: "running",
          turns_used: 1,
          turn_limit: 20,
          live: true,
          cause: null,
          contract_summary: "Complete the objective",
          last_verdict: null,
          context: {
            state: "unknown",
            used: null,
            size: null,
            ratio: null,
            nudge_ratio: 0.8,
            reported_at: null,
          },
        },
      };
      const failure = new SessionGoalCommandError(goal, 409, createdSession.id);
      if (outcome === "error") vi.mocked(sendSessionPrompt).mockRejectedValue(failure);
      else vi.mocked(sendSessionPrompt).mockResolvedValue(goal);
      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
      });
      const foreignGoal = { goal: { ...goal.snapshot, run_id: "foreign-run" } };
      queryClient.setQueryData(sessionKeys.goal("ws_beta", createdSession.id), foreignGoal);
      const { result } = renderHook(() => useSendSessionPrompt(), {
        wrapper: createWrapper(queryClient),
      });
      const message = "/goal New objective";

      await act(async () => {
        const pending = result.current.mutateAsync({
          id: createdSession.id,
          message,
          mode: "steer",
        });
        if (outcome === "error") await expect(pending).rejects.toBe(failure);
        else await expect(pending).resolves.toBe(goal);
      });

      expect(sessionStore.getSnapshot().context.goalFeedback[createdSession.id]).toMatchObject({
        command: message,
        result: goal,
        errorVisible: outcome === "error",
      });
      expect(queryClient.getQueryData(sessionKeys.goal(WORKSPACE_ID, createdSession.id))).toEqual({
        goal: goal.snapshot,
      });
      expect(queryClient.getQueryData(sessionKeys.goal("ws_beta", createdSession.id))).toEqual(
        foreignGoal
      );
    }
  );

  it("useSendSessionPrompt builds the canonical durable request with the resolved busy verb", async () => {
    vi.mocked(sendSessionPrompt).mockResolvedValue({
      delivery: "after_turn",
      disposition: "queued",
      entry_id: "inp_4d8",
      idempotency_key: "idempotency-001",
      message_id: "message-001",
      queue_position: 1,
      replayed: false,
      status: "queued",
    });
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const { result } = renderHook(() => useSendSessionPrompt(), {
      wrapper: createWrapper(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync({
        id: createdSession.id,
        idempotencyKey: "idempotency-001",
        message: "Queue this durable input.",
        messageId: "message-001",
        mode: "queue",
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
  ])("useSendSessionPrompt rejects %s in an explicit action identity", async (_case, identity) => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const { result } = renderHook(() => useSendSessionPrompt(), {
      wrapper: createWrapper(queryClient),
    });

    await act(async () => {
      await expect(
        result.current.mutateAsync({
          id: createdSession.id,
          message: "Queue this durable input.",
          mode: "queue",
          ...identity,
        })
      ).rejects.toThrow(
        "A session prompt action requires both non-empty message_id and idempotency_key"
      );
    });

    expect(sendSessionPrompt).not.toHaveBeenCalled();
  });

  it.each(["steer", "interrupt"] as const)(
    "useSendSessionPrompt sends %s with a strict fence when the active turn is known and without one otherwise",
    async mode => {
      vi.mocked(sendSessionPrompt).mockResolvedValue({
        delivery: "direct",
        disposition: mode === "steer" ? "steering" : "interrupting",
        idempotency_key: "idempotency-001",
        message_id: "message-001",
        queue_position: 0,
        replayed: false,
        status: mode === "steer" ? "steering" : "interrupting",
        turn_id: "turn-001",
      });
      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
      });
      const { result } = renderHook(() => useSendSessionPrompt(), {
        wrapper: createWrapper(queryClient),
      });

      await act(async () => {
        await result.current.mutateAsync({
          expectedTurnId: "turn-001",
          id: createdSession.id,
          idempotencyKey: "idempotency-001",
          message: "Replace the active work.",
          messageId: "message-001",
          mode,
        });
        await result.current.mutateAsync({
          id: createdSession.id,
          idempotencyKey: "idempotency-002",
          message: "Let the daemon pick the turn.",
          messageId: "message-002",
          mode,
        });
      });

      expect(sendSessionPrompt).toHaveBeenNthCalledWith(
        1,
        WORKSPACE_ID,
        createdSession.id,
        expect.objectContaining({ expected_turn_id: "turn-001", mode })
      );
      expect(sendSessionPrompt).toHaveBeenNthCalledWith(
        2,
        WORKSPACE_ID,
        createdSession.id,
        expect.not.objectContaining({ expected_turn_id: expect.anything() })
      );
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
    const owningQueue: SessionInputsResponse = {
      inputs: [queuedInput],
      queue: { cap: 10, entries: 1 },
    };
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
    // The daemon's queue summary survives a list-only write; the reread refreshes it.
    expect(
      queryClient.getQueryData(sessionKeys.inputQueue(WORKSPACE_ID, createdSession.id))
    ).toEqual({ inputs: [replacement], queue: { cap: 10, entries: 1 } });
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
      queue_position: 0,
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
      { inputs: [queuedInput, retained], queue: { cap: 10, entries: 2 } }
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
    ).toEqual({ inputs: [retained], queue: { cap: 10, entries: 2 } });

    const cancel = renderHook(() => useCancelSessionInput(WORKSPACE_ID, createdSession.id), {
      wrapper: createWrapper(queryClient),
    });
    await act(async () => {
      await cancel.result.current.mutateAsync(retained.id);
    });
    expect(
      queryClient.getQueryData(sessionKeys.inputQueue(WORKSPACE_ID, createdSession.id))
    ).toEqual({ inputs: [], queue: { cap: 10, entries: 2 } });
  });

  it("useClearSessionInputs keeps what the daemon did not cancel and its queue summary", async () => {
    const dispatching: SessionInputPayload = {
      ...queuedInput,
      id: "input-dispatching",
      status: "dispatching",
    };
    vi.mocked(clearSessionInputs).mockResolvedValue({
      cleared_count: 1,
      inputs: [{ ...queuedInput, status: "canceled" }, dispatching],
      queue_generation: 2,
    });
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    queryClient.setQueryData<SessionInputsResponse>(
      sessionKeys.inputQueue(WORKSPACE_ID, createdSession.id),
      { inputs: [queuedInput, dispatching], queue: { cap: 10, entries: 2 } }
    );

    const { result } = renderHook(() => useClearSessionInputs(WORKSPACE_ID, createdSession.id), {
      wrapper: createWrapper(queryClient),
    });
    await act(async () => {
      await result.current.mutateAsync();
    });

    expect(clearSessionInputs).toHaveBeenCalledWith(WORKSPACE_ID, createdSession.id);
    // A dispatching entry is never canceled implicitly: it stays visible until it leaves on its own.
    expect(
      queryClient.getQueryData(sessionKeys.inputQueue(WORKSPACE_ID, createdSession.id))
    ).toEqual({ inputs: [dispatching], queue: { cap: 10, entries: 2 } });
  });
});

// Batch invariant: the lifecycle owner serializes existing mutations, preserves per-id results,
// and retries only failures. Adapter I/O remains this suite's existing mock boundary.
describe("session lifecycle batches", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(fetchSessionById).mockImplementation(async id => ({ ...createdSession, id }));
  });
  function lifecycle(options: Parameters<typeof useSessionLifecycleActions>[0] = {}) {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    return renderHook(() => useSessionLifecycleActions(options), {
      wrapper: createWrapper(client),
    });
  }

  // Invariant: a failed current-detail read fails only its own target, without mutating stale data.
  // Owner/canonical suite: session lifecycle batches, mocked only at adapter I/O.
  it.each([false, true])(
    "archives readable targets and reports unreadable targets with dialog=%s",
    async dialog => {
      const unavailable = { ...createdSession, id: "unavailable" };
      vi.mocked(fetchSessionById).mockImplementation(async id => {
        if (id === unavailable.id) throw new Error("Detail unavailable");
        return { ...(dialog ? withSessionProcessSummary(createdSession) : createdSession), id };
      });
      vi.mocked(archiveSession).mockResolvedValue({ ...createdSession, archived_at: "2026-10-30" });
      const { result } = lifecycle();
      act(() => result.current.actions.onArchiveMany?.([unavailable, createdSession]));
      if (dialog) {
        await waitFor(() => expect(result.current.processDialog.open).toBe(true));
        expect(archiveSession).not.toHaveBeenCalled();
        act(() => result.current.processDialog.onChoose("keep"));
      }
      await waitFor(() =>
        expect(result.current.batchResults).toEqual([
          { id: unavailable.id, status: "failed", error: "Detail unavailable" },
          { id: createdSession.id, status: "done" },
        ])
      );
      expect(archiveSession).toHaveBeenCalledTimes(1);
      if (dialog) expect(vi.mocked(archiveSession).mock.calls[0]?.[5]).toBe("keep");
      expect(vi.mocked(archiveSession).mock.calls[0]?.[1]).toBe(createdSession.id);
      expect(toast.error).toHaveBeenCalledWith(
        "1 session archived, 1 failed",
        expect.objectContaining({ description: expect.stringContaining("Detail unavailable") })
      );
    }
  );

  // Invariant: both list flows notify their host only after successful owner-scoped deletion.
  // Owner: session lifecycle mutation; canonical suite: session lifecycle batches.
  it.each([false, true])("reconciles only successful deletes with bulk=%s", async bulk => {
    const onDeleteSuccess = vi.fn();
    vi.mocked(deleteSession).mockRejectedValueOnce(new Error("Busy"));
    const { result } = lifecycle({ onDeleteSuccess });
    act(() => {
      if (bulk) result.current.actions.onDeleteMany?.([createdSession]);
      else result.current.actions.onDelete(createdSession);
    });
    act(() => result.current.deleteDialog.onConfirm());
    await waitFor(() => expect(result.current.deleteDialog.isDeleting).toBe(false));
    expect(onDeleteSuccess).not.toHaveBeenCalled();
    vi.mocked(deleteSession).mockResolvedValue(undefined);
    act(() => result.current.deleteDialog.onRetry?.());
    await waitFor(() => expect(result.current.deleteDialog.open).toBe(false));
    expect(onDeleteSuccess).toHaveBeenCalledExactlyOnceWith(createdSession);
  });

  it("deletes each target in its owning profile and workspace", async () => {
    const targets = [
      { ...createdSession, profile_name: "work", workspace_id: "ws_work" },
      {
        ...createdSession,
        id: "personal-session",
        profile_name: "personal",
        workspace_id: "ws_personal",
      },
    ];
    vi.mocked(deleteSession).mockResolvedValue(undefined);
    const { result } = lifecycle();
    act(() => result.current.actions.onDeleteMany?.(targets));
    act(() => result.current.deleteDialog.onConfirm());
    await waitFor(() => expect(result.current.deleteDialog.open).toBe(false));
    expect(vi.mocked(deleteSession).mock.calls).toEqual([
      ["ws_work", createdSession.id, "work"],
      ["ws_personal", "personal-session", "personal"],
    ]);
  });

  // Invariant: unreadable bulk targets fail individually while readable targets proceed.
  // Owner/canonical suite: session lifecycle batches, adapter I/O boundary only.
  it.each(["keep", "terminate"] as const)(
    "Should recover unreadable delete targets with a %s process choice",
    async processes => {
      const unavailable = { ...createdSession, id: "unavailable" };
      vi.mocked(fetchSessionById).mockImplementation(async id => {
        if (id === unavailable.id) throw new Error("Detail unavailable");
        return { ...createdSession, id };
      });
      vi.mocked(deleteSession).mockResolvedValue(undefined);
      const { result } = lifecycle();
      act(() => result.current.actions.onDeleteMany?.([unavailable, createdSession]));
      act(() => result.current.deleteDialog.onConfirm());
      await waitFor(() =>
        expect(result.current.batchResults).toEqual([
          { id: unavailable.id, status: "failed", error: "Detail unavailable" },
          { id: createdSession.id, status: "done" },
        ])
      );
      expect(deleteSession).toHaveBeenCalledTimes(1);
      expect(vi.mocked(deleteSession).mock.calls[0]?.[1]).toBe(createdSession.id);
      const readsBeforeRetry = vi
        .mocked(fetchSessionById)
        .mock.calls.filter(([id]) => id === unavailable.id).length;
      act(() => result.current.deleteDialog.onRetry?.());
      await waitFor(() =>
        expect(
          vi.mocked(fetchSessionById).mock.calls.filter(([id]) => id === unavailable.id).length
        ).toBeGreaterThan(readsBeforeRetry)
      );
      await waitFor(() =>
        expect(result.current.batchResults[0]).toEqual({
          id: unavailable.id,
          status: "failed",
          error: "Detail unavailable",
        })
      );
      expect(deleteSession).toHaveBeenCalledTimes(1);
      const recovered = withSessionProcessSummary(unavailable);
      vi.mocked(fetchSessionById).mockResolvedValue(recovered);
      act(() => result.current.deleteDialog.onRetry?.());
      await waitFor(() => expect(result.current.batchResults).toEqual([]));
      expect(result.current.deleteDialog.sessions?.map(session => session.id)).toEqual([
        unavailable.id,
      ]);
      expect(result.current.deleteDialog.session?.process_summary).toEqual(
        recovered.process_summary
      );
      expect(deleteSession).toHaveBeenCalledTimes(1);
      act(() => result.current.deleteDialog.onConfirm(processes));
      await waitFor(() => expect(deleteSession).toHaveBeenCalledTimes(2));
      expect(vi.mocked(deleteSession).mock.calls[1]?.[1]).toBe(unavailable.id);
      expect(vi.mocked(deleteSession).mock.calls[1]?.[4]).toBe(processes);
    }
  );

  it("deletes sequentially, keeps failures selected, and retries only their ids", async () => {
    const targets = [
      createdSession,
      { ...createdSession, id: "second" },
      { ...createdSession, id: "third" },
    ];
    let releaseFirst!: () => void;
    vi.mocked(deleteSession)
      .mockImplementationOnce(
        () =>
          new Promise<void>(resolve => {
            releaseFirst = resolve;
          })
      )
      .mockRejectedValueOnce(new Error("Session is locked"))
      .mockResolvedValueOnce(undefined)
      .mockResolvedValueOnce(undefined);
    const selectionChanged = vi.fn();
    const { result } = lifecycle();
    act(() => result.current.actions.onDeleteMany?.(targets, selectionChanged));
    act(() => {
      result.current.deleteDialog.onConfirm();
      result.current.deleteDialog.onConfirm();
    });
    await waitFor(() => expect(deleteSession).toHaveBeenCalledTimes(1));
    expect(result.current.deleteDialog.results?.map(row => row.status)).toEqual([
      "running",
      "pending",
      "pending",
    ]);
    expect(result.current.actions.pendingAction).toBe("delete");
    act(() => result.current.deleteDialog.onOpenChange(false));
    expect(result.current.deleteDialog.open).toBe(true);
    await act(async () => releaseFirst());
    await waitFor(() => expect(result.current.deleteDialog.isDeleting).toBe(false));
    expect(vi.mocked(deleteSession).mock.calls.map(call => call[1])).toEqual([
      createdSession.id,
      "second",
      "third",
    ]);
    expect(result.current.deleteDialog.results).toEqual([
      { id: createdSession.id, status: "done" },
      { id: "second", status: "failed", error: "Session is locked" },
      { id: "third", status: "done" },
    ]);
    expect(result.current.deleteDialog.open).toBe(true);
    expect(selectionChanged).toHaveBeenLastCalledWith(["second"]);
    expect(toast.success).not.toHaveBeenCalled();
    expect(toast.error).not.toHaveBeenCalled();
    // Completed targets are gone: retry must read only the remaining failed ID.
    vi.mocked(fetchSessionById).mockImplementation(async id => {
      if (id !== "second") throw new Error("Session not found");
      return { ...createdSession, id };
    });
    act(() => result.current.deleteDialog.onRetry?.());
    await waitFor(() => expect(result.current.deleteDialog.open).toBe(false));
    expect(vi.mocked(deleteSession).mock.calls.map(call => call[1])).toEqual([
      createdSession.id,
      "second",
      "third",
      "second",
    ]);
    expect(selectionChanged).toHaveBeenLastCalledWith([]);
    expect(toast.success).toHaveBeenCalledExactlyOnceWith("3 sessions deleted");
    expect(toast.error).not.toHaveBeenCalled();
    act(() => result.current.deleteDialog.onOpenChange(false));
    expect(toast.success).toHaveBeenCalledTimes(1);
  });

  // Feedback invariant: one lifetime-total toast on Close; row errors remain in the dialog.
  // Owner: lifecycle batch coordinator, this existing suite at the adapter I/O boundary.
  it.each([0, 1, 2])(
    "announces %i deleted sessions only when closing a partial result",
    async deletedCount => {
      const deleted = Array.from({ length: deletedCount }, (_, index) => ({
        ...createdSession,
        id: `deleted-${index}`,
      }));
      for (const _session of deleted) vi.mocked(deleteSession).mockResolvedValueOnce(undefined);
      vi.mocked(deleteSession).mockRejectedValueOnce(new Error("Busy"));
      const selectionChanged = vi.fn();
      const { result } = lifecycle();
      act(() =>
        result.current.actions.onDeleteMany?.(
          [...deleted, { ...createdSession, id: "failed" }],
          selectionChanged
        )
      );
      act(() => result.current.deleteDialog.onConfirm());
      await waitFor(() => expect(result.current.deleteDialog.isDeleting).toBe(false));
      expect(result.current.deleteDialog.results).toEqual([
        ...deleted.map(session => ({ id: session.id, status: "done" })),
        { id: "failed", status: "failed", error: "Busy" },
      ]);
      expect(result.current.deleteDialog.open).toBe(true);
      expect(toast.success).not.toHaveBeenCalled();
      expect(toast.error).not.toHaveBeenCalled();
      act(() => result.current.deleteDialog.onOpenChange(false));
      expect(result.current.deleteDialog.open).toBe(false);
      expect(selectionChanged).toHaveBeenLastCalledWith(["failed"]);
      if (deletedCount === 0) expect(toast.success).not.toHaveBeenCalled();
      else
        expect(toast.success).toHaveBeenCalledExactlyOnceWith(
          `${deletedCount} ${deletedCount === 1 ? "session" : "sessions"} deleted`
        );
      expect(toast.error).not.toHaveBeenCalled();
      act(() => result.current.deleteDialog.onOpenChange(false));
      expect(toast.success).toHaveBeenCalledTimes(deletedCount > 0 ? 1 : 0);
    }
  );

  it("stops only eligible sessions sequentially and announces verified completion once", async () => {
    const stopped = { ...createdSession, id: "stopped", state: "stopped" as const };
    const starting = {
      ...createdSession,
      id: "starting",
      workspace_id: "ws_beta",
      profile_name: "studio",
      state: "starting" as const,
    };
    let release!: (value: Awaited<ReturnType<typeof stopSession>>) => void;
    vi.mocked(stopSession)
      .mockImplementationOnce(
        () =>
          new Promise(resolve => {
            release = resolve;
          })
      )
      .mockResolvedValueOnce({
        session_id: "starting",
        status: "stopped",
        state: "stopped",
        verified: true,
        escalated: false,
        stop_cause: "user_requested",
      });
    const { result } = lifecycle();
    act(() =>
      result.current.actions.onStopMany?.([
        createdSession,
        stopped,
        starting,
        { ...stopped, id: "archived", archived_at: "2026-09-11" },
      ])
    );
    await waitFor(() => expect(stopSession).toHaveBeenCalledTimes(1));
    expect(stopSession).toHaveBeenCalledWith(WORKSPACE_ID, createdSession.id, "default", {
      wait: true,
    });
    await act(async () =>
      release({
        session_id: createdSession.id,
        status: "stopped",
        state: "stopped",
        verified: true,
        escalated: false,
        stop_cause: "user_requested",
      })
    );
    await waitFor(() =>
      // The sessions the verb does not apply to are named with why, not dropped silently.
      expect(toast.success).toHaveBeenCalledExactlyOnceWith("2 sessions stopped", {
        description: "2 skipped: 2 not running.",
      })
    );
    expect(vi.mocked(stopSession).mock.calls).toEqual([
      [WORKSPACE_ID, createdSession.id, "default", { wait: true }],
      ["ws_beta", "starting", "studio", { wait: true }],
    ]);
  });

  it("explains a batch that applies to none of the selection, by reason", async () => {
    const stopped = { ...createdSession, id: "stopped", state: "stopped" as const };
    const archivedLive = { ...createdSession, id: "archived-live", archived_at: "2026-09-11" };
    const { result } = lifecycle();
    act(() => result.current.actions.onStopMany?.([stopped, archivedLive]));
    await waitFor(() =>
      expect(toast.info).toHaveBeenCalledExactlyOnceWith("No sessions stopped", {
        description: "2 skipped: 1 not running, 1 archived.",
      })
    );
    expect(stopSession).not.toHaveBeenCalled();
    expect(toast.success).not.toHaveBeenCalled();
  });

  it("does not announce an unverified stop as stopped", async () => {
    vi.mocked(stopSession).mockResolvedValue({
      session_id: createdSession.id,
      status: "stopping",
      state: "stopping",
      verified: false,
      escalated: false,
      attention: "Stop is still pending",
      stop_cause: "user_requested",
    });
    const { result } = lifecycle();
    act(() => result.current.actions.onStopMany?.([createdSession]));
    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith("Couldn't stop 1 session", {
        description: "Created session: Stop is still pending",
      })
    );
    expect(toast.success).not.toHaveBeenCalled();
    expect(result.current.batchResults[0]?.status).toBe("failed");
  });

  it("archives and unarchives only eligible rows with one toast per batch", async () => {
    const stopped = { ...createdSession, id: "stopped", state: "stopped" as const };
    const archived = { ...stopped, id: "archived", archived_at: "2026-09-11" };
    vi.mocked(archiveSession).mockResolvedValue(archived);
    vi.mocked(unarchiveSession).mockResolvedValue(stopped);
    const { result } = lifecycle();
    const child = {
      ...stopped,
      id: "child",
      lineage: {
        kind: "spawn" as const,
        spawn_role: "subagent" as const,
        parent_session_id: "stopped",
        root_session_id: "stopped",
        spawn_depth: 1,
        auto_stop_on_parent: false,
        notify_creator: false,
        spawn_budget: { max_children: 0, max_depth: 0, ttl_seconds: 0 },
        permission_policy: { tools: [], skills: [], mcp_servers: [], workspace_paths: [] },
      },
    };
    act(() => result.current.actions.onArchiveMany?.([createdSession, stopped, archived, child]));
    await waitFor(() =>
      expect(toast.success).toHaveBeenCalledWith("2 sessions archived", {
        description: "2 skipped: 1 already archived, 1 child session follows its parent.",
      })
    );
    // Every row archives through the stop-first operation, so a stopped parent with a
    // still-live subagent session archives too.
    expect(archiveSession).toHaveBeenCalledTimes(2);
    expect(archiveSession).toHaveBeenNthCalledWith(
      1,
      WORKSPACE_ID,
      createdSession.id,
      createdSession.profile_name,
      expect.any(AbortSignal),
      true
    );
    expect(archiveSession).toHaveBeenNthCalledWith(
      2,
      WORKSPACE_ID,
      "stopped",
      createdSession.profile_name,
      expect.any(AbortSignal),
      true
    );
    act(() => result.current.actions.onUnarchiveMany?.([createdSession, stopped, archived]));
    await waitFor(() =>
      expect(toast.success).toHaveBeenCalledWith("1 session unarchived", {
        description: "2 skipped: 2 not archived.",
      })
    );
    expect(unarchiveSession).toHaveBeenCalledExactlyOnceWith(
      WORKSPACE_ID,
      "archived",
      createdSession.profile_name,
      expect.any(AbortSignal)
    );
  });

  it("archives a mixed selection with per-session outcomes when one stop fails", async () => {
    const live = { ...createdSession, id: "sess-live", state: "active" as const };
    const stopped = { ...createdSession, id: "sess-stopped", state: "stopped" as const };
    const failing = {
      ...createdSession,
      id: "sess-failing",
      name: "Failing session",
      state: "active" as const,
    };
    vi.mocked(archiveSession).mockImplementation(async (_workspaceId, id) => {
      if (id === failing.id) throw new Error("stop before archiving failed");
      return { ...createdSession, id, state: "stopped", archived_at: "2026-08-04T12:00:00Z" };
    });
    const { result } = lifecycle();

    act(() => result.current.actions.onArchiveMany?.([live, failing, stopped]));

    await waitFor(() =>
      expect(result.current.batchResults).toEqual([
        { id: live.id, status: "done" },
        { id: failing.id, status: "failed", error: "stop before archiving failed" },
        { id: stopped.id, status: "done" },
      ])
    );
    const stopFlags = vi
      .mocked(archiveSession)
      .mock.calls.map(([, id, , , stop]) => [id, stop] as const);
    expect(stopFlags).toEqual([
      [live.id, true],
      [failing.id, true],
      [stopped.id, true],
    ]);
    // Successes are counted and each failure names its session.
    expect(toast.error).toHaveBeenCalledExactlyOnceWith("2 sessions archived, 1 failed", {
      description: "Failing session: stop before archiving failed",
    });
    expect(toast.success).not.toHaveBeenCalled();
  });
});
