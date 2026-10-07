import { describe, expect, it } from "vitest";

import type { MemoryType } from "@/systems/knowledge/types";

import { KNOWLEDGE_TYPE_TONE, knowledgeTypeFor } from "../knowledge-type-tone";

describe("KNOWLEDGE_TYPE_TONE", () => {
  it("Should map every backend MemoryType onto a KnowledgeType key", () => {
    const sample: MemoryType[] = ["user", "feedback", "project", "reference"];
    for (const type of sample) {
      const key = knowledgeTypeFor(type);
      expect(KNOWLEDGE_TYPE_TONE[key]).toBeDefined();
    }
    expect(knowledgeTypeFor("project")).toBe("decisions");
    expect(knowledgeTypeFor("reference")).toBe("code");
    expect(knowledgeTypeFor("user")).toBe("notes");
    expect(knowledgeTypeFor("feedback")).toBe("notes");
  });
});
