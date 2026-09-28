import { describe, expect, it } from "vitest";

import {
  compareKnowledgeScope,
  decisionOpLabel,
  decisionSourceLabel,
  knowledgeAgentTierLabel,
  knowledgeMemoryKey,
  knowledgeScopeLabel,
  knowledgeTypeLabel,
} from "../knowledge-formatters";

describe("knowledge-formatters", () => {
  it("Should derive a stable knowledge memory key from scope plus filename", () => {
    expect(knowledgeMemoryKey({ filename: "user.md", scope: "profile", key: undefined })).toBe(
      "profile:user.md"
    );
    expect(knowledgeMemoryKey({ filename: "user.md", scope: "agent", key: "custom-key" })).toBe(
      "custom-key"
    );
  });

  it("Should sort scopes with profile before workspace before agent", () => {
    expect(compareKnowledgeScope("profile", "workspace")).toBeLessThan(0);
    expect(compareKnowledgeScope("workspace", "profile")).toBeGreaterThan(0);
    expect(compareKnowledgeScope("agent", "workspace")).toBeGreaterThan(0);
    expect(compareKnowledgeScope("profile", "profile")).toBe(0);
  });

  it("Should expose plain-language scope labels with workspace aliased to project", () => {
    expect(knowledgeScopeLabel("profile")).toBe("Profile");
    expect(knowledgeScopeLabel("workspace")).toBe("Project");
    expect(knowledgeScopeLabel("agent")).toBe("Agent");
  });

  it("Should expose plain-language agent tier labels", () => {
    expect(knowledgeAgentTierLabel("global")).toBe("Agent · all projects");
    expect(knowledgeAgentTierLabel("workspace")).toBe("Agent · this project");
  });

  it("Should map every memory type to a plain display label", () => {
    expect(knowledgeTypeLabel("user")).toBe("About you");
    expect(knowledgeTypeLabel("feedback")).toBe("Feedback");
    expect(knowledgeTypeLabel("project")).toBe("Project decision");
    expect(knowledgeTypeLabel("reference")).toBe("Reference");
  });

  it("Should expose sentence-case decision op and source labels", () => {
    expect(decisionOpLabel("noop")).toBe("No change");
    expect(decisionOpLabel("add")).toBe("Added");
    expect(decisionOpLabel("update")).toBe("Updated");
    expect(decisionOpLabel("delete")).toBe("Deleted");
    expect(decisionOpLabel("reject")).toBe("Rejected");
    expect(decisionSourceLabel("rule")).toBe("Rule");
    expect(decisionSourceLabel("llm")).toBe("Automatic");
  });
});
