// Suite: the shell-hosted session delete confirmations
// Invariant: a session window asks the shell to confirm its rail deletes, so the dialog outlives
// the daemon's window reconciliation; no listener means the caller keeps its own fallback.
// Owning layer: the OS shell lifecycle host; canonical suite: this hook's suite.
import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { primarySessionFixture } from "@/systems/session/testing";
import type { SessionLifecycleActionHandlers } from "@/systems/session";

import {
  requestSessionDelete,
  subscribeSessionDeleteRequests,
} from "../../lib/session-delete-request";
import { useHostedSessionDeleteRequests } from "../use-hosted-session-delete-requests";

function actions(overrides: Partial<SessionLifecycleActionHandlers> = {}) {
  return {
    onDelete: vi.fn(),
    onDeleteMany: vi.fn(),
    ...overrides,
  } as unknown as SessionLifecycleActionHandlers;
}

describe("hosted session delete requests", () => {
  it("Should report no host and forward nothing when no shell listener is mounted", () => {
    expect(requestSessionDelete({ sessions: [primarySessionFixture], bulk: false })).toBe(false);
  });

  it("Should forward one request to every mounted host and stop after unsubscribe", () => {
    const first = vi.fn();
    const second = vi.fn();
    const stopFirst = subscribeSessionDeleteRequests(first);
    const stopSecond = subscribeSessionDeleteRequests(second);

    expect(requestSessionDelete({ sessions: [primarySessionFixture], bulk: false })).toBe(true);
    expect(first).toHaveBeenCalledOnce();
    expect(second).toHaveBeenCalledOnce();

    stopFirst();
    stopSecond();
    expect(requestSessionDelete({ sessions: [primarySessionFixture], bulk: false })).toBe(false);
  });

  it("Should route a single delete to onDelete and a batch to onDeleteMany", () => {
    const handlers = actions();
    const onSelectionChange = vi.fn();
    renderHook(() => useHostedSessionDeleteRequests(handlers));

    act(() => {
      requestSessionDelete({ sessions: [primarySessionFixture], bulk: false });
    });
    expect(handlers.onDelete).toHaveBeenCalledWith(primarySessionFixture);

    act(() => {
      requestSessionDelete({
        sessions: [primarySessionFixture],
        bulk: true,
        onSelectionChange,
      });
    });
    expect(handlers.onDeleteMany).toHaveBeenCalledWith([primarySessionFixture], onSelectionChange);
  });

  it("Should ignore a batch when the host has no batch action", () => {
    const handlers = actions({ onDeleteMany: undefined });
    renderHook(() => useHostedSessionDeleteRequests(handlers));

    act(() => {
      requestSessionDelete({ sessions: [primarySessionFixture], bulk: true });
    });
    expect(handlers.onDelete).not.toHaveBeenCalled();
  });
});
