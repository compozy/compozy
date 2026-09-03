// Suite: terminal input answer mutation
// Invariant: the typed secret reaches the daemon without creating a TanStack
// mutation record, so a cache dump cannot replay hidden input.
// Owning layer: terminal answer hook. Canonical suite: this file — no prior
// suite owned the mutation-cache boundary; the card suite owns the DOM mask.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { PASSWORD_REQUEST } from "../../mocks/terminal-fixtures";
import { useTerminalInputAnswer } from "../use-terminal-input-answer";

const SECRET = "hunter2hunter2";

vi.mock("../../adapters/terminal-api", () => ({
  answerTerminalInputRequest: vi.fn(),
}));

import { answerTerminalInputRequest } from "../../adapters/terminal-api";

function renderAnswer() {
  const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  const onSuccess = vi.fn();
  const onError = vi.fn();
  const view = renderHook(() => useTerminalInputAnswer("ws-atlas", { onSuccess, onError }), {
    wrapper: ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    ),
  });
  return { ...view, onError, onSuccess, queryClient };
}

describe("useTerminalInputAnswer", () => {
  it("Should deliver the secret without creating a mutation cache record", async () => {
    vi.mocked(answerTerminalInputRequest).mockResolvedValue({
      delivered_bytes: SECRET.length,
      redacted: true,
    });
    const { onSuccess, queryClient, result } = renderAnswer();

    act(() => {
      result.current.mutate({ request: PASSWORD_REQUEST, value: SECRET });
    });

    await waitFor(() => expect(onSuccess).toHaveBeenCalledOnce());
    expect(answerTerminalInputRequest).toHaveBeenCalledWith(
      "ws-atlas",
      PASSWORD_REQUEST.terminal_id,
      PASSWORD_REQUEST.id,
      SECRET,
      { profile: PASSWORD_REQUEST.profile_name }
    );

    expect(queryClient.getMutationCache().getAll()).toEqual([]);
  });

  it("Should keep concurrent answers isolated for the same pending request", async () => {
    const first = "first-secret";
    const second = "second-secret";
    vi.mocked(answerTerminalInputRequest).mockClear();
    vi.mocked(answerTerminalInputRequest).mockResolvedValue({
      delivered_bytes: first.length,
      redacted: true,
    });
    const { queryClient, result } = renderAnswer();

    act(() => {
      result.current.mutate({ request: PASSWORD_REQUEST, value: first });
      result.current.mutate({ request: PASSWORD_REQUEST, value: second });
    });

    await waitFor(() => expect(answerTerminalInputRequest).toHaveBeenCalledTimes(2));
    expect(answerTerminalInputRequest).toHaveBeenNthCalledWith(
      1,
      "ws-atlas",
      PASSWORD_REQUEST.terminal_id,
      PASSWORD_REQUEST.id,
      first,
      { profile: PASSWORD_REQUEST.profile_name }
    );
    expect(answerTerminalInputRequest).toHaveBeenNthCalledWith(
      2,
      "ws-atlas",
      PASSWORD_REQUEST.terminal_id,
      PASSWORD_REQUEST.id,
      second,
      { profile: PASSWORD_REQUEST.profile_name }
    );
    expect(JSON.stringify(queryClient.getMutationCache().getAll())).not.toContain(first);
    expect(JSON.stringify(queryClient.getMutationCache().getAll())).not.toContain(second);
    expect(queryClient.getMutationCache().getAll()).toEqual([]);
  });
});
