import { describe, expect, it } from "vitest";

import type { AgentPayload } from "../../types";

import {
  buildAgentCategoryTree,
  formatCategoryLabel,
  getAgentCategoryFolderId,
  getAgentLeafId,
  isAgentRootLevel,
} from "../agent-category";

function makeAgent(overrides: Partial<AgentPayload> & { name: string }): AgentPayload {
  return {
    provider: overrides.provider ?? "claude",
    prompt: overrides.prompt ?? `prompt for ${overrides.name}`,
    ...overrides,
  } as AgentPayload;
}

describe("agent-category", () => {
  describe("buildAgentCategoryTree", () => {
    it("Should build a flat list when no agent has category_path", () => {
      const tree = buildAgentCategoryTree([
        makeAgent({ name: "writer" }),
        makeAgent({ name: "coder" }),
      ]);
      expect(tree).toHaveLength(2);
      expect(tree.every(node => node.kind === "leaf")).toBe(true);
      expect(tree.map(node => node.label)).toEqual(["coder", "writer"]);
    });

    it("Should group agents by single-segment category_path", () => {
      const tree = buildAgentCategoryTree([
        makeAgent({ name: "support", category_path: ["Sales"] }),
        makeAgent({ name: "outbound", category_path: ["Sales"] }),
      ]);
      expect(tree).toHaveLength(1);
      const folder = tree[0];
      if (folder.kind !== "folder") throw new Error("expected folder");
      expect(folder.label).toBe("Sales");
      expect(folder.children.map(child => child.label)).toEqual(["outbound", "support"]);
    });

    it("Should build nested folders for multi-segment paths", () => {
      const tree = buildAgentCategoryTree([
        makeAgent({ name: "deals", category_path: ["Marketing", "Sales"] }),
      ]);
      const root = tree[0];
      if (root.kind !== "folder") throw new Error("expected folder");
      expect(root.label).toBe("Marketing");
      expect(root.children).toHaveLength(1);
      const inner = root.children[0];
      if (inner.kind !== "folder") throw new Error("expected folder");
      expect(inner.label).toBe("Sales");
      expect(inner.segments).toEqual(["Marketing", "Sales"]);
      expect(inner.children).toHaveLength(1);
      expect(inner.children[0].label).toBe("deals");
    });

    it("Should sort folders before leaves", () => {
      const tree = buildAgentCategoryTree([
        makeAgent({ name: "alpha" }),
        makeAgent({ name: "indexed", category_path: ["Z"] }),
      ]);
      expect(tree[0].kind).toBe("folder");
      expect(tree[1].kind).toBe("leaf");
    });

    it("Should sort siblings case-insensitively by visible label", () => {
      const tree = buildAgentCategoryTree([
        makeAgent({ name: "Beta" }),
        makeAgent({ name: "alpha" }),
        makeAgent({ name: "Charlie" }),
      ]);
      expect(tree.map(node => node.label)).toEqual(["alpha", "Beta", "Charlie"]);
    });

    it("Should derive deterministic folder IDs from joined segments", () => {
      const tree = buildAgentCategoryTree([
        makeAgent({ name: "deals", category_path: ["Marketing", "Sales"] }),
      ]);
      const root = tree[0];
      if (root.kind !== "folder") throw new Error("expected folder");
      expect(root.id).toBe("category:Marketing");
      const inner = root.children[0];
      if (inner.kind !== "folder") throw new Error("expected folder");
      expect(inner.id).toBe("category:Marketing/Sales");
    });

    it("Should derive deterministic leaf IDs from agent names", () => {
      const tree = buildAgentCategoryTree([makeAgent({ name: "coder" })]);
      expect(tree[0].id).toBe("agent:coder");
    });

    it("Should render root-level leaves alongside top-level folders without an Uncategorized folder", () => {
      const tree = buildAgentCategoryTree([
        makeAgent({ name: "alpha" }),
        makeAgent({ name: "beta", category_path: ["Engineering"] }),
      ]);
      expect(tree).toHaveLength(2);
      expect(
        tree.find(node => node.kind === "folder" && node.label === "Uncategorized")
      ).toBeUndefined();
      expect(tree.map(node => node.kind)).toEqual(["folder", "leaf"]);
    });

    it("Should treat undefined and empty-array category_path as root-level", () => {
      const tree = buildAgentCategoryTree([
        makeAgent({ name: "alpha", category_path: undefined }),
        makeAgent({ name: "beta", category_path: [] }),
      ]);
      expect(tree.every(node => node.kind === "leaf")).toBe(true);
      expect(tree.map(node => node.label).sort()).toEqual(["alpha", "beta"]);
    });

    it("Should treat segments with different casing as distinct folders", () => {
      const tree = buildAgentCategoryTree([
        makeAgent({ name: "alpha", category_path: ["Engineering"] }),
        makeAgent({ name: "beta", category_path: ["engineering"] }),
      ]);
      expect(tree).toHaveLength(2);
      const folders = tree.filter(node => node.kind === "folder");
      expect(folders).toHaveLength(2);
      const labels = folders.map(folder => folder.label).sort();
      expect(labels).toEqual(["Engineering", "engineering"]);
      for (const folder of folders) {
        if (folder.kind !== "folder") throw new Error("expected folder");
        expect(folder.children).toHaveLength(1);
      }
    });

    it("Should preserve casing in folder IDs derived from category_path segments", () => {
      const tree = buildAgentCategoryTree([
        makeAgent({ name: "deals", category_path: ["Marketing", "Sales"] }),
      ]);
      const root = tree[0];
      if (root.kind !== "folder") throw new Error("expected folder");
      expect(root.id).toBe("category:Marketing");
      expect(root.label).toBe("Marketing");
    });
  });

  describe("formatCategoryLabel", () => {
    it("Should join segments with separator", () => {
      expect(formatCategoryLabel(["Marketing", "Sales"])).toBe("Marketing / Sales");
    });

    it("Should return empty string for null, undefined, or empty array", () => {
      expect(formatCategoryLabel(null)).toBe("");
      expect(formatCategoryLabel(undefined)).toBe("");
      expect(formatCategoryLabel([])).toBe("");
    });
  });

  describe("id helpers", () => {
    it("Should compute folder id from segments", () => {
      expect(getAgentCategoryFolderId(["Marketing", "Sales"])).toBe("category:Marketing/Sales");
    });

    it("Should compute leaf id from name", () => {
      expect(getAgentLeafId("coder")).toBe("agent:coder");
    });
  });

  describe("isAgentRootLevel", () => {
    it("Should return true when category_path is empty or missing", () => {
      expect(isAgentRootLevel(makeAgent({ name: "a" }))).toBe(true);
      expect(isAgentRootLevel(makeAgent({ name: "a", category_path: [] }))).toBe(true);
    });

    it("Should return false when agent has a non-empty category_path", () => {
      expect(isAgentRootLevel(makeAgent({ name: "a", category_path: ["X"] }))).toBe(false);
    });
  });
});
