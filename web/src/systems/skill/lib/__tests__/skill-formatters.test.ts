import { describe, expect, it } from "vitest";

import type { SkillPayload } from "../../types";

import {
  compareSkillSource,
  deriveSkillAuthor,
  deriveSkillCapabilities,
  deriveSkillRecentCalls,
  deriveSkillTags,
  filterSkillsByQuery,
  MARKETPLACE_CATEGORIES,
  matchesMarketplaceCategory,
  skillSourceLabel,
  skillSourceTone,
  skillStatusTone,
} from "../skill-formatters";

function makeSkill(overrides: Partial<SkillPayload> = {}): SkillPayload {
  return {
    name: "test-skill",
    description: "desc",
    source: "bundled",
    enabled: true,
    activation: { active: true },
    dir: "/path",
    ...overrides,
  };
}

describe("skill-formatters", () => {
  it("Should order sources as bundled → workspace → marketplace → user → additional", () => {
    const sorted = ["additional", "user", "marketplace", "workspace", "bundled"].sort(
      compareSkillSource
    );
    expect(sorted).toEqual(["bundled", "workspace", "marketplace", "user", "additional"]);
  });

  it("Should map sources to MonoBadge tones", () => {
    expect(skillSourceTone("bundled")).toBe("success");
    expect(skillSourceTone("workspace")).toBe("info");
    expect(skillSourceTone("marketplace")).toBe("accent");
    expect(skillSourceTone("user")).toBe("warning");
    expect(skillSourceTone("additional")).toBe("neutral");
    expect(skillSourceTone("unknown")).toBe("neutral");
  });

  it("Should return sentence-case source labels", () => {
    expect(skillSourceLabel("bundled")).toBe("bundled");
    expect(skillSourceLabel("workspace")).toBe("workspace");
    expect(skillSourceLabel("marketplace")).toBe("marketplace");
  });

  it("Should map enabled flag to status dot tone", () => {
    expect(skillStatusTone(true)).toBe("success");
    expect(skillStatusTone(false)).toBe("neutral");
  });

  it("Should prefer provenance slug for author, falling back to metadata.author", () => {
    expect(deriveSkillAuthor(makeSkill())).toBeUndefined();
    expect(
      deriveSkillAuthor(
        makeSkill({
          provenance: {
            slug: "compozy",
            registry: "official",
            installed_at: "",
            precedence_tier: "marketplace",
            version: "1",
          },
        })
      )
    ).toBe("compozy");
    expect(
      deriveSkillAuthor(
        makeSkill({
          provenance: {
            slug: "workspace",
            registry: "workspace",
            installed_at: "",
            precedence_tier: "workspace",
            version: "1",
          },
          metadata: { author: "pedronauck" },
        })
      )
    ).toBe("pedronauck");
  });

  it("Should filter tags down to string entries", () => {
    expect(
      deriveSkillTags(makeSkill({ metadata: { tags: ["a", 1, null, "b"] as unknown as string[] } }))
    ).toEqual(["a", "b"]);
  });

  it("Should expose sentence-case marketplace category vocabulary", () => {
    expect(MARKETPLACE_CATEGORIES).toEqual([
      "all",
      "testing",
      "database",
      "deploy",
      "ai",
      "devops",
      "security",
    ]);
    for (const category of MARKETPLACE_CATEGORIES) {
      expect(category).toBe(category.toLowerCase());
    }
  });

  it("Should match marketplace category against tags (case-insensitive)", () => {
    const skill = makeSkill({ metadata: { tags: ["Testing", "AI"] } });
    expect(matchesMarketplaceCategory(skill, "all")).toBe(true);
    expect(matchesMarketplaceCategory(skill, "testing")).toBe(true);
    expect(matchesMarketplaceCategory(skill, "ai")).toBe(true);
    expect(matchesMarketplaceCategory(skill, "database")).toBe(false);
  });

  it("Should filter skills by name, description, and tags", () => {
    const skills: SkillPayload[] = [
      makeSkill({ name: "alpha", description: "first", metadata: { tags: ["testing"] } }),
      makeSkill({ name: "beta", description: "second" }),
    ];
    expect(filterSkillsByQuery(skills, "")).toHaveLength(2);
    expect(filterSkillsByQuery(skills, "alpha")).toHaveLength(1);
    expect(filterSkillsByQuery(skills, "SECOND")).toHaveLength(1);
    expect(filterSkillsByQuery(skills, "testing")).toHaveLength(1);
    expect(filterSkillsByQuery(skills, "zzz")).toHaveLength(0);
  });

  it("Should parse capabilities metadata into string array", () => {
    expect(
      deriveSkillCapabilities(
        makeSkill({
          metadata: { capabilities: ["shell.run", 42, "git.stage"] as unknown as string[] },
        })
      )
    ).toEqual(["shell.run", "git.stage"]);
    expect(deriveSkillCapabilities(makeSkill())).toEqual([]);
  });

  it("Should parse recent_calls metadata, defaulting status to success", () => {
    const calls = deriveSkillRecentCalls(
      makeSkill({
        metadata: {
          recent_calls: [
            { label: "skill.run", status: "success", timestamp: "2026-04-17T12:00:00Z" },
            { label: "skill.fail", status: "error" },
            { label: "skill.pending" },
            { status: "success" },
          ],
        },
      })
    );
    expect(calls).toEqual([
      { label: "skill.run", status: "success", timestamp: "2026-04-17T12:00:00Z" },
      { label: "skill.fail", status: "error", timestamp: undefined },
      { label: "skill.pending", status: "success", timestamp: undefined },
    ]);
  });
});
