import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor, act } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  useDisableSkill,
  useEnableSkill,
  useInstallSkillMarketplace,
  useRemoveSkillMarketplace,
  useUpdateSkillMarketplace,
} from "@/systems/skill/hooks/use-skill-actions";

vi.mock("@/systems/skill/adapters/skill-api", () => ({
  listSkills: vi.fn(),
  getSkill: vi.fn(),
  enableSkill: vi.fn(),
  disableSkill: vi.fn(),
  installSkillMarketplace: vi.fn(),
  updateSkillMarketplace: vi.fn(),
  removeSkillMarketplace: vi.fn(),
}));

import {
  disableSkill,
  enableSkill,
  installSkillMarketplace,
  removeSkillMarketplace,
  updateSkillMarketplace,
} from "@/systems/skill/adapters/skill-api";

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
      result.current.mutate({ name: "test-skill", workspace: "ws_123" });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(enableSkill).toHaveBeenCalledWith("test-skill", "ws_123");
    expect(invalidateSpy).toHaveBeenNthCalledWith(1, {
      queryKey: ["skills", "list", "ws_123"],
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(2, {
      queryKey: ["skills", "detail", "test-skill", "ws_123"],
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(3, {
      queryKey: ["skills", "content", "test-skill", "ws_123"],
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
      result.current.mutate({ name: "test-skill", workspace: "ws_123" });
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(invalidateSpy).toHaveBeenNthCalledWith(1, {
      queryKey: ["skills", "list", "ws_123"],
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(2, {
      queryKey: ["skills", "detail", "test-skill", "ws_123"],
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(3, {
      queryKey: ["skills", "content", "test-skill", "ws_123"],
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
      result.current.mutate({ name: "test-skill", workspace: "ws_123" });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(disableSkill).toHaveBeenCalledWith("test-skill", "ws_123");
    expect(invalidateSpy).toHaveBeenNthCalledWith(1, {
      queryKey: ["skills", "list", "ws_123"],
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(2, {
      queryKey: ["skills", "detail", "test-skill", "ws_123"],
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(3, {
      queryKey: ["skills", "content", "test-skill", "ws_123"],
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
      result.current.mutate({ name: "test-skill", workspace: "ws_123" });
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(invalidateSpy).toHaveBeenNthCalledWith(1, {
      queryKey: ["skills", "list", "ws_123"],
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(2, {
      queryKey: ["skills", "detail", "test-skill", "ws_123"],
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(3, {
      queryKey: ["skills", "content", "test-skill", "ws_123"],
    });
  });
});

describe("useInstallSkillMarketplace", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("installs by slug and invalidates the installed list", async () => {
    vi.mocked(installSkillMarketplace).mockResolvedValue({
      name: "demo",
      slug: "@compozy/demo",
      status: "installed",
      hash: "sha256:demo",
      path: "/opt/agh/skills/demo",
      registry: "clawhub",
      version: "1.0.0",
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(() => useInstallSkillMarketplace(), { wrapper });

    act(() => {
      result.current.mutate({
        body: { slug: "@compozy/demo" },
        workspace: "ws_123",
      });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(installSkillMarketplace).toHaveBeenCalledWith({ slug: "@compozy/demo" });
    expect(invalidateSpy).toHaveBeenNthCalledWith(1, {
      queryKey: ["skills", "list", "ws_123"],
    });
  });
});

describe("useUpdateSkillMarketplace", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("updates the named skill and invalidates the installed list", async () => {
    vi.mocked(updateSkillMarketplace).mockResolvedValue([]);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(() => useUpdateSkillMarketplace(), { wrapper });

    act(() => {
      result.current.mutate({
        body: { name: "demo" },
        workspace: "ws_123",
      });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(updateSkillMarketplace).toHaveBeenCalledWith({ name: "demo" });
    expect(invalidateSpy).toHaveBeenNthCalledWith(1, {
      queryKey: ["skills", "list", "ws_123"],
    });
  });
});

describe("useRemoveSkillMarketplace", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("removes by name and invalidates the installed list", async () => {
    vi.mocked(removeSkillMarketplace).mockResolvedValue({
      name: "demo",
      slug: "@compozy/demo",
      status: "removed",
      path: "/opt/agh/skills/demo",
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(() => useRemoveSkillMarketplace(), { wrapper });

    act(() => {
      result.current.mutate({ name: "demo", workspace: "ws_123" });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(removeSkillMarketplace).toHaveBeenCalledWith("demo");
    expect(invalidateSpy).toHaveBeenNthCalledWith(1, {
      queryKey: ["skills", "list", "ws_123"],
    });
  });
});
