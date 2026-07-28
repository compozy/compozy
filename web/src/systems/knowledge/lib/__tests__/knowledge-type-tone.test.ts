import { describe, expect, it } from "vitest";

import type { MemoryType } from "@/systems/knowledge/types";

import { KNOWLEDGE_TYPE_TONE, type KnowledgeType, knowledgeTypeFor } from "../knowledge-type-tone";

describe("KNOWLEDGE_TYPE_TONE", () => {
  it("Should ship the retuned vocabulary with semantic tones", () => {
    expect(KNOWLEDGE_TYPE_TONE.decisions).toBe("info");
    expect(KNOWLEDGE_TYPE_TONE.code).toBe("neutral");
    expect(KNOWLEDGE_TYPE_TONE.notes).toBe("neutral");
    expect(KNOWLEDGE_TYPE_TONE.runbooks).toBe("warning");
    expect(KNOWLEDGE_TYPE_TONE.archival).toBe("faint");
  });

  it("Should never assign accent to a memory-type tone (retune dropped accent)", () => {
    const tones = Object.values(KNOWLEDGE_TYPE_TONE);
    expect(tones).not.toContain("accent");
  });

  it("Should cover every KnowledgeType key exhaustively", () => {
    const declared: Record<KnowledgeType, true> = {
      decisions: true,
      code: true,
      notes: true,
      runbooks: true,
      archival: true,
    };
    for (const key of Object.keys(declared) as KnowledgeType[]) {
      expect(KNOWLEDGE_TYPE_TONE[key]).toBeDefined();
    }
    // Every dictionary key must be a declared KnowledgeType.
    for (const key of Object.keys(KNOWLEDGE_TYPE_TONE)) {
      expect(key in declared).toBe(true);
    }
  });

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
