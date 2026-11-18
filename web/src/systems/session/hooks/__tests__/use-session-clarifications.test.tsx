import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("../../adapters/session-clarification-api", async importOriginal => {
  const actual = await importOriginal<typeof import("../../adapters/session-clarification-api")>();
  return {
    ...actual,
    answerSessionClarification: vi.fn(),
    fetchSessionClarifications: vi.fn(),
  };
});

import {
  answerSessionClarification,
  ClarificationNotAnswerableError,
  fetchSessionClarifications,
} from "../../adapters/session-clarification-api";
import { SessionApiError } from "../../adapters/session-api";
import { sessionKeys } from "../../lib/query-keys";
import { SessionBusyInputRefusalError } from "../../lib/session-busy-input-refusal";
import type { ClarificationPending, SessionBusyInputDraft } from "../../types";
import { useClarificationComposerAnswer } from "../use-clarification-composer-answer";
import { useAnswerSessionClarification } from "../use-session-clarifications";

const WS = "ws_alpha";
const OTHER_WS = "ws_other";
const ID = "sess-001";
const OTHER_ID = "sess-002";

function createWrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

function setup() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const invalidate = vi.spyOn(queryClient, "invalidateQueries");
  return { invalidate, wrapper: createWrapper(queryClient) };
}

afterEach(() => {
  vi.clearAllMocks();
});

describe("useAnswerSessionClarification", () => {
  it("Should reconcile the owning clarification and transcript after a successful answer", async () => {
    vi.mocked(answerSessionClarification).mockResolvedValue({
      choice: 0,
      fallback: false,
      text: "",
    });
    const { invalidate, wrapper } = setup();
    const { result } = renderHook(() => useAnswerSessionClarification(WS, ID), { wrapper });

    await result.current.mutateAsync({ requestId: "req-1", body: { choice_index: 0 } });

    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledWith({
        exact: true,
        queryKey: sessionKeys.clarifications(WS, ID),
      });
      expect(invalidate).toHaveBeenCalledWith({
        exact: true,
        queryKey: sessionKeys.transcript(WS, ID),
      });
    });
    expect(invalidate).not.toHaveBeenCalledWith({
      exact: true,
      queryKey: sessionKeys.clarifications(OTHER_WS, ID),
    });
    expect(invalidate).not.toHaveBeenCalledWith({
      exact: true,
      queryKey: sessionKeys.clarifications(WS, OTHER_ID),
    });
  });

  it("Should re-read the exact owner after a terminal-gone failure", async () => {
    vi.mocked(answerSessionClarification).mockRejectedValue(
      new ClarificationNotAnswerableError(ID, "req-1")
    );
    const { invalidate, wrapper } = setup();
    const { result } = renderHook(() => useAnswerSessionClarification(WS, ID), { wrapper });

    await expect(
      result.current.mutateAsync({ requestId: "req-1", body: { text: "late" } })
    ).rejects.toBeInstanceOf(ClarificationNotAnswerableError);

    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledWith({
        exact: true,
        queryKey: sessionKeys.clarifications(WS, ID),
      });
    });
  });
});

const PENDING: ClarificationPending = {
  agent_name: "claude",
  asked_at: "2026-11-18T22:00:00Z",
  choices: ["Ship it", "Hold"],
  deadline: "0001-01-01T00:00:00Z",
  question: "Ship the release?",
  request_id: "req-1",
  session_id: ID,
};

const NEXT: ClarificationPending = { ...PENDING, question: "Which region?", request_id: "req-2" };

type Attachment = SessionBusyInputDraft["attachments"][number];

function draft(message: string, attachments: Attachment[] = []) {
  return { attachments, message } satisfies SessionBusyInputDraft;
}

async function renderComposerAnswer(
  pending: ClarificationPending[],
  options: { stopPhase?: string } = {}
) {
  vi.mocked(fetchSessionClarifications).mockResolvedValue(pending);
  const { wrapper } = setup();
  const { result } = renderHook(
    () => useClarificationComposerAnswer({ sessionId: ID, workspaceId: WS, ...options }),
    {
      wrapper,
    }
  );
  await waitFor(() => {
    expect(fetchSessionClarifications).toHaveBeenCalled();
  });
  if (pending.length > 0 && options.stopPhase !== "stopping") {
    await waitFor(() => {
      expect(result.current.pending?.request_id).toBe(pending[0]?.request_id);
    });
  }
  return result;
}

describe("useClarificationComposerAnswer", () => {
  it("Should answer the pending clarification with a composer send instead of steering behind it", async () => {
    vi.mocked(answerSessionClarification).mockResolvedValue({
      choice: null,
      fallback: false,
      text: "Ship it on Friday",
    });
    const steer = vi.fn();
    const result = await renderComposerAnswer([PENDING]);

    await result.current.answering(steer)?.(draft("  Ship it on Friday  "));

    expect(answerSessionClarification).toHaveBeenCalledWith(WS, ID, "req-1", {
      text: "Ship it on Friday",
    });
    expect(steer).not.toHaveBeenCalled();
  });

  it("Should answer the next pending question when the docked one settled first", async () => {
    vi.mocked(answerSessionClarification)
      .mockRejectedValueOnce(new ClarificationNotAnswerableError(ID, "req-1"))
      .mockResolvedValueOnce({ choice: null, fallback: false, text: "Europe" });
    const steer = vi.fn();
    const result = await renderComposerAnswer([PENDING, NEXT]);
    vi.mocked(fetchSessionClarifications).mockResolvedValue([NEXT]);

    await result.current.answering(steer)?.(draft("Europe"));

    expect(answerSessionClarification).toHaveBeenLastCalledWith(WS, ID, "req-2", {
      text: "Europe",
    });
    expect(steer).not.toHaveBeenCalled();
  });

  it("Should send the draft as before when no question waits any more", async () => {
    vi.mocked(answerSessionClarification).mockRejectedValue(
      new ClarificationNotAnswerableError(ID, "req-1")
    );
    const queue = vi.fn();
    const result = await renderComposerAnswer([PENDING]);
    vi.mocked(fetchSessionClarifications).mockResolvedValue([]);
    const sent = draft("Never mind, go ahead");

    await result.current.answering(queue)?.(sent);

    expect(queue).toHaveBeenCalledWith(sent);
  });

  it("Should keep the draft with the reason when the question only accepts a listed choice", async () => {
    vi.mocked(answerSessionClarification).mockRejectedValue(
      new SessionApiError("tools: invalid clarification", 400, ID)
    );
    const steer = vi.fn();
    const result = await renderComposerAnswer([PENDING]);

    await expect(result.current.answering(steer)?.(draft("Ship it"))).rejects.toBeInstanceOf(
      SessionBusyInputRefusalError
    );
    expect(steer).not.toHaveBeenCalled();
  });

  it("Should keep the draft when the send carries files, with or without text", async () => {
    const steer = vi.fn();
    const result = await renderComposerAnswer([PENDING]);
    const file = { id: "att-1" } as Attachment;

    for (const sent of [draft("See the log", [file]), draft("", [file])]) {
      await expect(result.current.answering(steer)?.(sent)).rejects.toBeInstanceOf(
        SessionBusyInputRefusalError
      );
    }
    expect(answerSessionClarification).not.toHaveBeenCalled();
    expect(steer).not.toHaveBeenCalled();
  });

  it.each([
    ["the question is gone", () => vi.mocked(fetchSessionClarifications).mockResolvedValue([])],
    [
      "the re-read fails",
      () =>
        vi.mocked(fetchSessionClarifications).mockRejectedValue(new TypeError("Failed to fetch")),
    ],
  ])(
    "Should keep a lost-acknowledgment draft and replay it as the answer, never a prompt, when %s",
    async (_case, settleReread) => {
      vi.mocked(answerSessionClarification).mockRejectedValueOnce(new TypeError("Failed to fetch"));
      const steer = vi.fn();
      const result = await renderComposerAnswer([PENDING]);
      settleReread();

      await expect(result.current.answering(steer)?.(draft("Ship it"))).rejects.toBeInstanceOf(
        SessionBusyInputRefusalError
      );
      vi.mocked(answerSessionClarification).mockRejectedValueOnce(
        new ClarificationNotAnswerableError(ID, "req-1")
      );
      await waitFor(() => {
        expect(result.current.answering(steer)).not.toBe(steer);
      });
      await result.current.answering(steer)?.(draft("Ship it"));

      expect(answerSessionClarification).toHaveBeenLastCalledWith(WS, ID, "req-1", {
        text: "Ship it",
      });
      expect(steer).not.toHaveBeenCalled();
    }
  );

  it("Should leave busy sends untouched when no question is pending or the session is stopping", async () => {
    const steer = vi.fn();
    const idle = await renderComposerAnswer([]);
    const stopping = await renderComposerAnswer([PENDING], { stopPhase: "stopping" });

    expect(idle.current.answering(steer)).toBe(steer);
    expect(stopping.current.pending).toBeNull();
    expect(stopping.current.answering(steer)).toBe(steer);
  });
});
