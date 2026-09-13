import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor, act } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useDisableSkill, useEnableSkill } from "@/systems/skill/hooks/use-skill-actions";

vi.mock("@/systems/skill/adapters/skill-api", () => ({
  listSkills: vi.fn(),
  getSkill: vi.fn(),
  enableSkill: vi.fn(),
  disableSkill: vi.fn(),
}));

import { disableSkill, enableSkill } from "@/systems/skill/adapters/skill-api";

describe("useEnableSkill", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("calls enableSkill and invalidates skill list cache on settle", async () => {
    vi.mocked(enableSkill).mockResolvedValue({ ok: true });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(() => useEnableSkill(), { wrapper });

    act(() => {
      result.current.mutate({ name: "test-skill", workspace: "ws_123", profile: "research" });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(enableSkill).toHaveBeenCalledWith("test-skill", "ws_123", "research");
    expect(invalidateSpy).toHaveBeenNthCalledWith(1, {
      queryKey: ["skills", "list", "ws_123", "research"],
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(2, {
      queryKey: ["skills", "detail", "test-skill", "ws_123", "research"],
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(3, {
      queryKey: ["skills", "content", "test-skill", "ws_123", "research"],
    });
  });

  it("invalidates skill list cache when enableSkill fails", async () => {
    vi.mocked(enableSkill).mockRejectedValue(new Error("fail"));

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(() => useEnableSkill(), { wrapper });

    act(() => {
      result.current.mutate({ name: "test-skill", workspace: "ws_123", profile: "research" });
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(invalidateSpy).toHaveBeenNthCalledWith(1, {
      queryKey: ["skills", "list", "ws_123", "research"],
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(2, {
      queryKey: ["skills", "detail", "test-skill", "ws_123", "research"],
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(3, {
      queryKey: ["skills", "content", "test-skill", "ws_123", "research"],
    });
  });
});

describe("useDisableSkill", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("calls disableSkill and invalidates skill list cache on settle", async () => {
    vi.mocked(disableSkill).mockResolvedValue({ ok: true });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(() => useDisableSkill(), { wrapper });

    act(() => {
      result.current.mutate({ name: "test-skill", workspace: "ws_123", profile: "research" });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(disableSkill).toHaveBeenCalledWith("test-skill", "ws_123", "research");
    expect(invalidateSpy).toHaveBeenNthCalledWith(1, {
      queryKey: ["skills", "list", "ws_123", "research"],
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(2, {
      queryKey: ["skills", "detail", "test-skill", "ws_123", "research"],
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(3, {
      queryKey: ["skills", "content", "test-skill", "ws_123", "research"],
    });
  });

  it("invalidates skill list cache when disableSkill fails", async () => {
    vi.mocked(disableSkill).mockRejectedValue(new Error("fail"));

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(() => useDisableSkill(), { wrapper });

    act(() => {
      result.current.mutate({ name: "test-skill", workspace: "ws_123", profile: "research" });
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(invalidateSpy).toHaveBeenNthCalledWith(1, {
      queryKey: ["skills", "list", "ws_123", "research"],
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(2, {
      queryKey: ["skills", "detail", "test-skill", "ws_123", "research"],
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(3, {
      queryKey: ["skills", "content", "test-skill", "ws_123", "research"],
    });
  });
});
