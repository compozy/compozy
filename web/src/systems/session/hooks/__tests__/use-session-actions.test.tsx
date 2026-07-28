import { QueryClient, QueryClientProvider, type InfiniteData } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useSessionStore } from "../use-session-store";
import {
  useClearSessionConversation,
  useCreateSession,
  useDeleteSession,
  useRepairSession,
} from "../use-session-actions";
import { sessionKeys } from "../../lib/query-keys";
import type { SessionTranscriptData } from "../../lib/session-transcript-query";
import type { SessionMessage, SessionPayload, SessionsResponse } from "../../types";

vi.mock("../../adapters/session-api", () => ({
  clearSessionConversation: vi.fn(),
  createSession: vi.fn(),
  deleteSession: vi.fn(),
  repairSession: vi.fn(),
  stopSession: vi.fn(),
  resumeSession: vi.fn(),
}));

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({ activeWorkspaceId: "ws_alpha" }),
}));

import {
  clearSessionConversation,
  createSession,
  deleteSession,
  repairSession,
} from "../../adapters/session-api";

const WORKSPACE_ID = "ws_alpha";

function createWrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

const createdSession: SessionPayload = {
  id: "sess-created",
  name: "Created session",
  agent_name: "claude-agent",
  provider: "claude",
  workspace_id: "ws_alpha",
  workspace_path: "/workspace/alpha",
  state: "active",
  badge: "idle",
  attachable: true,
  available_commands: [],
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
    useSessionStore.setState({ drafts: {} });
  });

  afterEach(() => {
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
    queryClient.setQueryData(sessionKeys.list({ workspace: "ws_alpha" }), workspaceSessions);
    queryClient.setQueryData(sessionKeys.list({ workspace: "ws_beta" }), otherWorkspaceSessions);
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

    expect(createSession).toHaveBeenCalledWith({
      agent_name: createdSession.agent_name,
      workspace: createdSession.workspace_id,
    });
    expect(queryClient.getQueryData(sessionKeys.detail(WORKSPACE_ID, createdSession.id))).toEqual(
      createdSession
    );
    expect(queryClient.getQueryData(sessionKeys.byId(createdSession.id))).toEqual(createdSession);
    expect(queryClient.getQueryData(sessionKeys.list())).toEqual(allSessions);
    expect(queryClient.getQueryData(sessionKeys.list({ workspace: "ws_alpha" }))).toEqual(
      workspaceSessions
    );
    expect(queryClient.getQueryData(sessionKeys.list({ workspace: "ws_beta" }))).toEqual(
      otherWorkspaceSessions
    );
    expect(
      queryClient.getQueryState(sessionKeys.list({ workspace: WORKSPACE_ID }))?.isInvalidated
    ).toBe(true);
    expect(
      queryClient.getQueryState(sessionKeys.list({ workspace: "ws_beta" }))?.isInvalidated
    ).toBe(false);
    expect(invalidateSpy).toHaveBeenNthCalledWith(1, {
      queryKey: sessionKeys.detail(WORKSPACE_ID, createdSession.id),
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(2, {
      queryKey: sessionKeys.workspaceLists(WORKSPACE_ID),
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
    useSessionStore.getState().setDraft(createdSession.id, { text: "keep me" });

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
    expect(useSessionStore.getState().drafts[createdSession.id]?.text).toBe("keep me");
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
    useSessionStore.getState().setDraft(createdSession.id, { text: "keep me" });

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
    expect(useSessionStore.getState().drafts[createdSession.id]?.text).toBe("keep me");
  });

  it("useDeleteSession removes cached session data and clears the draft", async () => {
    vi.mocked(deleteSession).mockResolvedValue(undefined);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    queryClient.setQueryData(sessionKeys.detail(WORKSPACE_ID, createdSession.id), createdSession);
    queryClient.setQueryData(sessionKeys.byId(createdSession.id), createdSession);
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
    useSessionStore.getState().setDraft(createdSession.id, { text: "remove me" });

    const { result } = renderHook(() => useDeleteSession(), {
      wrapper: createWrapper(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync(createdSession.id);
    });

    expect(deleteSession).toHaveBeenCalledWith(WORKSPACE_ID, createdSession.id);
    expect(
      queryClient.getQueryData(sessionKeys.detail(WORKSPACE_ID, createdSession.id))
    ).toBeUndefined();
    expect(queryClient.getQueryData(sessionKeys.byId(createdSession.id))).toBeUndefined();
    expect(
      queryClient.getQueryData(sessionKeys.transcript(WORKSPACE_ID, createdSession.id))
    ).toBeUndefined();
    expect(
      queryClient.getQueryData(sessionKeys.history(WORKSPACE_ID, createdSession.id))
    ).toBeUndefined();
    expect(
      queryClient.getQueryData(sessionKeys.events(WORKSPACE_ID, createdSession.id))
    ).toBeUndefined();
    expect(useSessionStore.getState().drafts[createdSession.id]).toBeUndefined();
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: sessionKeys.workspaceLists(WORKSPACE_ID),
    });
  });

  it("useDeleteSession preserves cached session data and drafts on failure", async () => {
    vi.mocked(deleteSession).mockRejectedValue(new Error("delete failed"));

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    queryClient.setQueryData(sessionKeys.detail(WORKSPACE_ID, createdSession.id), createdSession);
    queryClient.setQueryData(sessionKeys.byId(createdSession.id), createdSession);

    const transcriptSnapshot = transcriptCache();
    const historySnapshot = [{ id: "turn-1" }];
    const eventsSnapshot = [{ id: "event-1" }];
    queryClient.setQueryData(
      sessionKeys.transcript(WORKSPACE_ID, createdSession.id),
      transcriptSnapshot
    );
    queryClient.setQueryData(sessionKeys.history(WORKSPACE_ID, createdSession.id), historySnapshot);
    queryClient.setQueryData(sessionKeys.events(WORKSPACE_ID, createdSession.id), eventsSnapshot);
    useSessionStore.getState().setDraft(createdSession.id, { text: "keep me" });

    const { result } = renderHook(() => useDeleteSession(), {
      wrapper: createWrapper(queryClient),
    });

    await act(async () => {
      await expect(result.current.mutateAsync(createdSession.id)).rejects.toThrow("delete failed");
    });

    expect(queryClient.getQueryData(sessionKeys.detail(WORKSPACE_ID, createdSession.id))).toEqual(
      createdSession
    );
    expect(queryClient.getQueryData(sessionKeys.byId(createdSession.id))).toEqual(createdSession);
    expect(
      queryClient.getQueryData(sessionKeys.transcript(WORKSPACE_ID, createdSession.id))
    ).toEqual(transcriptSnapshot);
    expect(queryClient.getQueryData(sessionKeys.history(WORKSPACE_ID, createdSession.id))).toEqual(
      historySnapshot
    );
    expect(queryClient.getQueryData(sessionKeys.events(WORKSPACE_ID, createdSession.id))).toEqual(
      eventsSnapshot
    );
    expect(useSessionStore.getState().drafts[createdSession.id]?.text).toBe("keep me");
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
      queryKey: sessionKeys.byId(createdSession.id),
      exact: true,
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: sessionKeys.workspaceLists(WORKSPACE_ID),
    });
  });
});
