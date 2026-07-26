import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("../../adapters/session-clarification-api", async importOriginal => {
  const actual = await importOriginal<typeof import("../../adapters/session-clarification-api")>();
  return { ...actual, answerSessionClarification: vi.fn() };
});

import {
  answerSessionClarification,
  ClarificationNotAnswerableError,
} from "../../adapters/session-clarification-api";
import { sessionKeys } from "../../lib/query-keys";
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
  it("Should reconcile only the owning session's clarifications after a successful answer", async () => {
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
