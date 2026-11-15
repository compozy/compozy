// Suite: the shell-hosted session delete confirmations
// Invariant: a session window asks the shell to confirm its rail deletes through one xstate
// request store, so the dialog outlives the daemon's window reconciliation and each request
// is consumed once.
// Owning layer: the OS shell lifecycle host; canonical suite: this hook's suite.
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { primarySessionFixture } from "@/systems/session/testing";
import type { SessionLifecycleActionHandlers } from "@/systems/session";

import {
  requestSessionDelete,
  sessionDeleteRequestStore,
  settleSessionDeleteRequest,
  useHostedSessionDeleteRequests,
} from "../use-session-delete-request";

function actions(overrides: Partial<SessionLifecycleActionHandlers> = {}) {
  return {
    onDelete: vi.fn(),
    onDeleteMany: vi.fn(),
    ...overrides,
  } as unknown as SessionLifecycleActionHandlers;
}

describe("hosted session delete requests", () => {
  beforeEach(() => {
    sessionDeleteRequestStore.trigger.settled();
  });

  it("Should hold one request until the host settles it", () => {
    requestSessionDelete({ sessions: [primarySessionFixture], bulk: false });
    expect(sessionDeleteRequestStore.getSnapshot().context.request).toEqual({
      sessions: [primarySessionFixture],
      bulk: false,
    });

    settleSessionDeleteRequest();
    expect(sessionDeleteRequestStore.getSnapshot().context.request).toBeNull();
  });

  it("Should route a single delete to onDelete and clear the request", () => {
    const handlers = actions();
    renderHook(() => useHostedSessionDeleteRequests(handlers));

    act(() => {
      requestSessionDelete({ sessions: [primarySessionFixture], bulk: false });
    });
    expect(handlers.onDelete).toHaveBeenCalledWith(primarySessionFixture);
    expect(sessionDeleteRequestStore.getSnapshot().context.request).toBeNull();
  });

  it("Should route a batch to onDeleteMany with its selection callback", () => {
    const handlers = actions();
    const onSelectionChange = vi.fn();
    renderHook(() => useHostedSessionDeleteRequests(handlers));

    act(() => {
      requestSessionDelete({
        sessions: [primarySessionFixture],
        bulk: true,
        onSelectionChange,
      });
    });
    expect(handlers.onDeleteMany).toHaveBeenCalledWith([primarySessionFixture], onSelectionChange);
    expect(sessionDeleteRequestStore.getSnapshot().context.request).toBeNull();
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
