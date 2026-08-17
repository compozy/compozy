// Suite: daemon-backed session-list preferences
// Invariant: full-section preference writes never overlap, so an older response
// cannot overwrite a newer sort or scope choice.
// Owning layer: session-list preference hook. No prior suite owned this flow.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { mutation, query } = vi.hoisted(() => ({
  mutation: { isPending: false, mutate: vi.fn() },
  query: {
    data: {
      config: { sessions: { sort: "last_activity", scope: "workspace" } },
    },
    isLoading: false,
  },
}));

vi.mock("@/systems/settings", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/settings")>();
  return {
    ...actual,
    useSettingsShell: () => query,
    useUpdateSettingsShell: () => mutation,
  };
});

import { useSessionListPreferences } from "../use-session-list-preferences";

function wrapper({ children }: { children: ReactNode }) {
  return createElement(QueryClientProvider, { client: new QueryClient() }, children);
}

describe("useSessionListPreferences", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mutation.isPending = false;
  });

  it("Should reject a second full-config write until the first settles", () => {
    const { result } = renderHook(() => useSessionListPreferences(), { wrapper });

    act(() => {
      result.current.setSort("attention");
      result.current.setScope("all-workspaces");
    });

    expect(mutation.mutate).toHaveBeenCalledTimes(1);
    expect(mutation.mutate).toHaveBeenCalledWith(
      { config: { sessions: { sort: "attention", scope: "workspace" } } },
      expect.objectContaining({ onSettled: expect.any(Function) })
    );
  });
});
