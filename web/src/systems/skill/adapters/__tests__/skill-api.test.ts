import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockJsonResponse } from "@/test/fetch-test-utils";
import {
  disableSkill,
  enableSkill,
  getSkill,
  getSkillContent,
  getSkillShadows,
  installSkillMarketplace,
  listSkills,
  removeSkillMarketplace,
  SkillApiError,
  updateSkillMarketplace,
} from "@/systems/skill/adapters/skill-api";

const validSkill = {
  name: "test-skill",
  description: "A test skill",
  source: "bundled",
  enabled: true,
  dir: "/path/to/skill",
};

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("listSkills", () => {
  const validResponse = { skills: [validSkill] };

  it("calls GET /api/skills?workspace=:id and returns typed array", async () => {
    mockJsonResponse(validResponse);

    const result = await listSkills("ws_123");

    expect(result).toEqual([validSkill]);
    await expectFetchRequest({ path: "/api/skills?workspace=ws_123" });
  });

  it("passes abort signal to fetch", async () => {
    mockJsonResponse(validResponse);

    const controller = new AbortController();
    await listSkills("ws_123", controller.signal);

    await expectFetchRequest({
      path: "/api/skills?workspace=ws_123",
      signal: controller.signal,
    });
  });

  it("returns empty array when server returns empty list", async () => {
    mockJsonResponse({ skills: [] });

    const result = await listSkills("ws_123");

    expect(result).toEqual([]);
  });

  it("throws SkillApiError on non-2xx response", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(listSkills("ws_123")).rejects.toThrow(SkillApiError);
    await expect(listSkills("ws_123")).rejects.toThrow("Failed to fetch skills: 500");
  });

  it("encodes workspace in URL", async () => {
    mockJsonResponse(validResponse);

    await listSkills("/home/user/project");

    await expectFetchRequest({ path: "/api/skills?workspace=%2Fhome%2Fuser%2Fproject" });
  });
});

describe("getSkill", () => {
  const validResponse = { skill: validSkill };

  it("calls GET /api/skills/:name?workspace=:id and returns typed object", async () => {
    mockJsonResponse(validResponse);

    const result = await getSkill("test-skill", "ws_123");

    expect(result).toEqual(validSkill);
    await expectFetchRequest({ path: "/api/skills/test-skill?workspace=ws_123" });
  });

  it("throws SkillApiError with 404 for unknown skill", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(getSkill("unknown", "ws_123")).rejects.toThrow("Skill not found: unknown");

    try {
      await getSkill("unknown", "ws_123");
    } catch (error) {
      expect(error).toBeInstanceOf(SkillApiError);
      expect((error as SkillApiError).status).toBe(404);
    }
  });

  it("throws SkillApiError for other failures", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 503 }));

    await expect(getSkill("test-skill", "ws_123")).rejects.toThrow(
      'Failed to fetch skill "test-skill": 503'
    );
  });

  it("encodes skill name in URL", async () => {
    mockJsonResponse(validResponse);

    await getSkill("my skill", "ws_123");

    await expectFetchRequest({ path: "/api/skills/my%20skill?workspace=ws_123" });
  });
});

describe("getSkillContent", () => {
  it("calls GET /api/skills/:name/content?workspace=:id and returns content string", async () => {
    mockJsonResponse({ content: "full skill content" });

    const result = await getSkillContent("test-skill", "ws_123");

    expect(result).toBe("full skill content");
    await expectFetchRequest({ path: "/api/skills/test-skill/content?workspace=ws_123" });
  });

  it("encodes skill name in content URL", async () => {
    mockJsonResponse({ content: "full skill content" });

    await getSkillContent("my skill", "ws_123");

    await expectFetchRequest({ path: "/api/skills/my%20skill/content?workspace=ws_123" });
  });
});

describe("getSkillShadows", () => {
  it("calls GET /api/skills/:name/shadows?workspace=:id and returns resolver rows", async () => {
    const response = {
      name: "test-skill",
      winner: {
        detected_at: "2026-04-17T17:00:00Z",
        path: "/workspace/.agh/skills/test-skill/SKILL.md",
        resolved_to_winner: true,
        tier: "workspace",
      },
      shadows: [
        {
          detected_at: "2026-04-17T17:00:00Z",
          path: "/workspace/.agh/skills/test-skill/SKILL.md",
          resolved_to_winner: true,
          tier: "workspace",
        },
      ],
    };
    mockJsonResponse(response);

    const result = await getSkillShadows("test-skill", "ws_123");

    expect(result).toEqual(response);
    await expectFetchRequest({ path: "/api/skills/test-skill/shadows?workspace=ws_123" });
  });
});

describe("enableSkill", () => {
  it("calls POST /api/skills/:name/enable and returns {ok: true}", async () => {
    mockJsonResponse({ ok: true });

    const result = await enableSkill("test-skill", "ws_123");

    expect(result).toEqual({ ok: true });
    await expectFetchRequest({
      method: "POST",
      path: "/api/skills/test-skill/enable?workspace=ws_123",
    });
  });

  it("throws SkillApiError on 404", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(enableSkill("unknown", "ws_123")).rejects.toThrow("Skill not found: unknown");
  });

  it("throws SkillApiError on non-2xx response", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(enableSkill("test-skill", "ws_123")).rejects.toThrow(SkillApiError);
  });
});

describe("disableSkill", () => {
  it("calls POST /api/skills/:name/disable and returns {ok: true}", async () => {
    mockJsonResponse({ ok: true });

    const result = await disableSkill("test-skill", "ws_123");

    expect(result).toEqual({ ok: true });
    await expectFetchRequest({
      method: "POST",
      path: "/api/skills/test-skill/disable?workspace=ws_123",
    });
  });

  it("throws SkillApiError on 404", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(disableSkill("unknown", "ws_123")).rejects.toThrow("Skill not found: unknown");
  });

  it("throws SkillApiError on non-2xx response", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(disableSkill("test-skill", "ws_123")).rejects.toThrow(SkillApiError);
  });
});

describe("SkillApiError", () => {
  it("has correct name and status properties", () => {
    const error = new SkillApiError("test error", 404);

    expect(error.name).toBe("SkillApiError");
    expect(error.status).toBe(404);
    expect(error.message).toBe("test error");
    expect(error).toBeInstanceOf(Error);
  });
});

describe("installSkillMarketplace", () => {
  it("calls POST /api/skills/marketplace/install with the slug body", async () => {
    mockJsonResponse({
      skill: {
        name: "demo",
        slug: "@compozy/demo",
        status: "installed",
        hash: "sha256:demo",
        path: "/opt/agh/skills/demo",
        registry: "clawhub",
        version: "0.1.0",
      },
    });

    const result = await installSkillMarketplace({ slug: "@compozy/demo" });
    expect(result.status).toBe("installed");
    await expectFetchRequest({
      method: "POST",
      path: "/api/skills/marketplace/install",
      body: { slug: "@compozy/demo" },
    });
  });

  it("throws SkillApiError on 503 when marketplace is unconfigured", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 503 }));
    await expect(installSkillMarketplace({ slug: "@compozy/demo" })).rejects.toThrow(SkillApiError);
  });
});

describe("updateSkillMarketplace", () => {
  it("calls POST /api/skills/marketplace/update with the supplied body", async () => {
    mockJsonResponse({
      skills: [
        {
          name: "demo",
          slug: "@compozy/demo",
          status: "updated",
          path: "/opt/agh/skills/demo",
          current_version: "0.1.0",
          latest_version: "0.2.0",
        },
      ],
    });

    const result = await updateSkillMarketplace({ name: "demo" });
    expect(result).toHaveLength(1);
    await expectFetchRequest({
      method: "POST",
      path: "/api/skills/marketplace/update",
      body: { name: "demo" },
    });
  });

  it("throws SkillApiError on 422 when the skill is not marketplace-managed", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 422 }));
    await expect(updateSkillMarketplace({ name: "demo" })).rejects.toThrow(SkillApiError);
  });
});

describe("removeSkillMarketplace", () => {
  it("calls DELETE /api/skills/marketplace/{name}", async () => {
    mockJsonResponse({
      skill: {
        name: "demo",
        slug: "@compozy/demo",
        status: "removed",
        path: "/opt/agh/skills/demo",
      },
    });

    const result = await removeSkillMarketplace("demo");
    expect(result.status).toBe("removed");
    await expectFetchRequest({
      method: "DELETE",
      path: "/api/skills/marketplace/demo",
    });
  });

  it("throws SkillApiError with 404 for unknown installed name", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));
    await expect(removeSkillMarketplace("missing")).rejects.toThrow(
      "Installed marketplace skill not found: missing"
    );
  });

  it("encodes name segment in URL", async () => {
    mockJsonResponse({
      skill: {
        name: "my skill",
        slug: "@compozy/my-skill",
        status: "removed",
        path: "/opt/agh/skills/my-skill",
      },
    });
    await removeSkillMarketplace("my skill");
    await expectFetchRequest({
      method: "DELETE",
      path: "/api/skills/marketplace/my%20skill",
    });
  });
});
