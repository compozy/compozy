import { describe, expect, it } from "vitest";

import {
  compareKnowledgeScope,
  decisionOpLabel,
  decisionSourceLabel,
  knowledgeAgentTierLabel,
  knowledgeAgentTierShortLabel,
  knowledgeMemoryKey,
  knowledgeScopeLabel,
  knowledgeScopeShortLabel,
  memoryScopeTone,
  memoryTypeTone,
} from "../knowledge-formatters";

describe("knowledge-formatters", () => {
  it("Should derive a stable knowledge memory key from scope plus filename", () => {
    expect(knowledgeMemoryKey({ filename: "user.md", scope: "global", key: undefined })).toBe(
      "global:user.md"
    );
    expect(knowledgeMemoryKey({ filename: "user.md", scope: "agent", key: "custom-key" })).toBe(
      "custom-key"
    );
  });

  it("Should sort scopes with global before workspace before agent", () => {
    expect(compareKnowledgeScope("global", "workspace")).toBeLessThan(0);
    expect(compareKnowledgeScope("workspace", "global")).toBeGreaterThan(0);
    expect(compareKnowledgeScope("agent", "workspace")).toBeGreaterThan(0);
    expect(compareKnowledgeScope("global", "global")).toBe(0);
  });

  it("Should expose sentence-case scope labels", () => {
    expect(knowledgeScopeLabel("global")).toBe("Global");
    expect(knowledgeScopeLabel("workspace")).toBe("Workspace");
    expect(knowledgeScopeLabel("agent")).toBe("Agent");
    expect(knowledgeScopeShortLabel("global")).toBe("global");
    expect(knowledgeScopeShortLabel("workspace")).toBe("ws");
    expect(knowledgeScopeShortLabel("agent")).toBe("agent");
  });

  it("Should expose sentence-case agent tier labels", () => {
    expect(knowledgeAgentTierLabel("global")).toBe("Agent · global");
    expect(knowledgeAgentTierLabel("workspace")).toBe("Agent · workspace");
    expect(knowledgeAgentTierShortLabel("global")).toBe("ag-global");
    expect(knowledgeAgentTierShortLabel("workspace")).toBe("ag-ws");
  });

  it("Should pass memory type tone through unchanged", () => {
    expect(memoryTypeTone("user")).toBe("user");
    expect(memoryTypeTone("feedback")).toBe("feedback");
    expect(memoryTypeTone("project")).toBe("project");
    expect(memoryTypeTone("reference")).toBe("reference");
  });

  it("Should pass memory scope tone through unchanged", () => {
    expect(memoryScopeTone("global")).toBe("global");
    expect(memoryScopeTone("workspace")).toBe("workspace");
    expect(memoryScopeTone("agent")).toBe("agent");
  });

  it("Should expose sentence-case decision op and source labels", () => {
    expect(decisionOpLabel("noop")).toBe("noop");
    expect(decisionOpLabel("add")).toBe("add");
    expect(decisionOpLabel("update")).toBe("update");
    expect(decisionOpLabel("delete")).toBe("delete");
    expect(decisionOpLabel("reject")).toBe("reject");
    expect(decisionSourceLabel("rule")).toBe("rule");
    expect(decisionSourceLabel("llm")).toBe("llm");
  });
});
