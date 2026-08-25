// Suite: skill query hooks
// Invariant: skill reads preserve workspace scope and suspend polling when disabled.
// Boundary IN: skill query options, cache keys, liveness gates, and hook projections.
// Boundary OUT: skill mutations and settings UI, owned by their dedicated suites.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useSkill, useSkillContent, useSkillShadows, useSkills } from "../use-skills";

vi.mock("../../adapters/skill-api", () => ({
  listSkills: vi.fn(),
  getSkill: vi.fn(),
  getSkillContent: vi.fn(),
  getSkillShadows: vi.fn(),
  enableSkill: vi.fn(),
  disableSkill: vi.fn(),
}));

import { getSkill, getSkillContent, getSkillShadows, listSkills } from "../../adapters/skill-api";

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

const validSkill = {
  name: "test-skill",
  description: "A test skill",
  source: "bundled",
  owner_scope: "user",
  enabled: true,
  activation: { active: true },
  dir: "/path/to/skill",
  origin: "",
};

describe("useSkills", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("returns loading state then data", async () => {
    vi.mocked(listSkills).mockResolvedValue([validSkill]);

    const { result } = renderHook(() => useSkills("ws_123"), {
      wrapper: createWrapper(),
    });

    expect(result.current.isLoading).toBe(true);

    await waitFor(() => {
      expect(result.current.data).toHaveLength(1);
    });

    expect(result.current.data).toEqual([validSkill]);
    expect(listSkills).toHaveBeenCalledWith("ws_123", expect.any(AbortSignal), undefined);
  });

  it("keeps the exact profile in a workspace skill read", async () => {
    vi.mocked(listSkills).mockResolvedValue([validSkill]);

    const { result } = renderHook(() => useSkills("ws_123", true, "research"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.data).toEqual([validSkill]));
    expect(listSkills).toHaveBeenCalledWith("ws_123", expect.any(AbortSignal), "research");
  });

  it("loads global skills when workspace is empty", async () => {
    vi.mocked(listSkills).mockResolvedValue([validSkill]);
    const { result } = renderHook(() => useSkills(""), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.data).toEqual([validSkill]));
    expect(listSkills).toHaveBeenCalledWith("", expect.any(AbortSignal), undefined);
  });

  it("does not fetch skills while its retained window is inactive", () => {
    const { result } = renderHook(() => useSkills("ws_123", false), {
      wrapper: createWrapper(),
    });

    expect(result.current.fetchStatus).toBe("idle");
    expect(listSkills).not.toHaveBeenCalled();
  });
});

describe("useSkill", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("loads a single skill detail", async () => {
    vi.mocked(getSkill).mockResolvedValue(validSkill);

    const { result } = renderHook(() => useSkill("test-skill", "ws_123"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.data?.name).toBe("test-skill");
    });

    expect(getSkill).toHaveBeenCalledWith(
      "test-skill",
      "ws_123",
      expect.any(AbortSignal),
      undefined
    );
  });

  it("keeps the exact profile in a workspace skill detail read", async () => {
    vi.mocked(getSkill).mockResolvedValue(validSkill);

    const { result } = renderHook(() => useSkill("test-skill", "ws_123", "research"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.data).toEqual(validSkill));
    expect(getSkill).toHaveBeenCalledWith(
      "test-skill",
      "ws_123",
      expect.any(AbortSignal),
      "research"
    );
  });

  it("does not fetch when name is empty", () => {
    renderHook(() => useSkill("", "ws_123"), {
      wrapper: createWrapper(),
    });

    expect(getSkill).not.toHaveBeenCalled();
  });

  it("loads a global skill when workspace is empty", async () => {
    vi.mocked(getSkill).mockResolvedValue(validSkill);
    const { result } = renderHook(() => useSkill("test-skill", ""), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.data).toEqual(validSkill));
    expect(getSkill).toHaveBeenCalledWith("test-skill", "", expect.any(AbortSignal), undefined);
  });
});

describe("useSkillContent", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("loads skill content when enabled", async () => {
    vi.mocked(getSkillContent).mockResolvedValue("full skill content");

    const { result } = renderHook(() => useSkillContent("test-skill", "ws_123", true), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.data).toBe("full skill content");
    });

    expect(getSkillContent).toHaveBeenCalledWith(
      "test-skill",
      "ws_123",
      expect.any(AbortSignal),
      undefined
    );
  });

  it("keeps the exact profile in a workspace skill content read", async () => {
    vi.mocked(getSkillContent).mockResolvedValue("profile skill content");

    const { result } = renderHook(() => useSkillContent("test-skill", "ws_123", true, "research"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.data).toBe("profile skill content"));
    expect(getSkillContent).toHaveBeenCalledWith(
      "test-skill",
      "ws_123",
      expect.any(AbortSignal),
      "research"
    );
  });

  it("does not fetch when disabled", () => {
    renderHook(() => useSkillContent("test-skill", "ws_123", false), {
      wrapper: createWrapper(),
    });

    expect(getSkillContent).not.toHaveBeenCalled();
  });
});

describe("useSkillShadows", () => {
  it("loads skill shadow rows when name and workspace are present", async () => {
    const response = {
      name: "test-skill",
      winner: {
        detected_at: "2026-04-17T17:00:00Z",
        path: "/workspace/.compozy/skills/test-skill/SKILL.md",
        resolved_to_winner: true,
        tier: "workspace",
      },
      shadows: [],
    };
    vi.mocked(getSkillShadows).mockResolvedValue(response);

    const { result } = renderHook(() => useSkillShadows("test-skill", "ws_123"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(response);
    expect(getSkillShadows).toHaveBeenCalledWith(
      "test-skill",
      "ws_123",
      expect.any(AbortSignal),
      undefined
    );
  });

  it("keeps the exact profile in a workspace skill shadow read", async () => {
    const response = {
      name: "test-skill",
      winner: {
        detected_at: "2026-04-17T17:00:00Z",
        path: "/workspace/.compozy/skills/test-skill/SKILL.md",
        resolved_to_winner: true,
        tier: "workspace",
      },
      shadows: [],
    };
    vi.mocked(getSkillShadows).mockResolvedValue(response);

    const { result } = renderHook(() => useSkillShadows("test-skill", "ws_123", "research"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(getSkillShadows).toHaveBeenCalledWith(
      "test-skill",
      "ws_123",
      expect.any(AbortSignal),
      "research"
    );
  });
});
