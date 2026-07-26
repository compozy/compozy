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
  enabled: true,
  activation: { active: true },
  dir: "/path/to/skill",
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
    expect(listSkills).toHaveBeenCalledWith("ws_123", expect.any(AbortSignal));
  });

  it("loads global skills when workspace is empty", async () => {
    vi.mocked(listSkills).mockResolvedValue([validSkill]);
    const { result } = renderHook(() => useSkills(""), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.data).toEqual([validSkill]));
    expect(listSkills).toHaveBeenCalledWith("", expect.any(AbortSignal));
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

    expect(getSkill).toHaveBeenCalledWith("test-skill", "ws_123", expect.any(AbortSignal));
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
    expect(getSkill).toHaveBeenCalledWith("test-skill", "", expect.any(AbortSignal));
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

    expect(getSkillContent).toHaveBeenCalledWith("test-skill", "ws_123", expect.any(AbortSignal));
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
        path: "/workspace/.agh/skills/test-skill/SKILL.md",
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
    expect(getSkillShadows).toHaveBeenCalledWith("test-skill", "ws_123", expect.any(AbortSignal));
  });
});
