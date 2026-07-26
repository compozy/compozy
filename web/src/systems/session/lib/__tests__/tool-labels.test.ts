import { describe, it, expect } from "vitest";
import {
  getToolIcon,
  getToolLabel,
  getToolCompactSummary,
  resolveRegisteredToolName,
} from "../tool-labels";
import {
  Brain,
  FileEdit,
  FileText,
  FolderSearch,
  Globe,
  Plug,
  Search,
  Terminal,
  Wrench,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

describe("getToolIcon", () => {
  it("Should map builtin tool ids to their per-tool glyph — never the terminal fallback", () => {
    const cases: Array<[toolId: string, icon: LucideIcon]> = [
      ["Bash", Terminal],
      ["Read", FileText],
      ["Write", FileEdit],
      ["Edit", FileEdit],
      ["Grep", Search],
      ["Glob", FolderSearch],
      ["WebSearch", Globe],
    ];
    for (const [toolId, icon] of cases) {
      expect(getToolIcon(toolId)).toBe(icon);
      if (toolId !== "Bash") expect(getToolIcon(toolId)).not.toBe(Terminal);
    }
  });

  it("Should map AGH native tool families from the agh__ taxonomy", () => {
    expect(getToolIcon("agh__edit")).toBe(FileEdit);
    expect(getToolIcon("agh__memory_note")).toBe(Brain);
    expect(getToolIcon("agh__memory_search")).toBe(Brain);
    // Unmapped native family falls through to the generic tool glyph.
    expect(getToolIcon("agh__deny_native")).toBe(Wrench);
  });

  it("Should map MCP bridge tools to the connector glyph", () => {
    expect(getToolIcon("mcp__context7__resolve-library-id")).toBe(Plug);
    expect(getToolIcon("mcp__github__search_issues", { url: "https://example.com" })).toBe(Plug);
  });

  it("Should return the generic tool fallback for unknown ids", () => {
    expect(getToolIcon("SomeUnknownTool")).toBe(Wrench);
    expect(getToolIcon("")).toBe(Wrench);
  });

  it("Should use semantic input fallbacks for uncatalogued dynamic tools", () => {
    expect(getToolIcon("SomeUnknownTool", { command: "ls -la" })).toBe(Terminal);
    expect(getToolIcon("SomeUnknownTool", { file_path: "/tmp/file.txt" })).toBe(FileText);
    expect(getToolIcon("SomeUnknownTool", { filePath: "/tmp/file.txt" })).toBe(FileText);
    expect(getToolIcon("SomeUnknownTool", { pattern: "TODO" })).toBe(Search);
    expect(getToolIcon("SomeUnknownTool", { url: "https://example.com" })).toBe(Globe);
    expect(getToolIcon("SomeUnknownTool", { query: "search term" })).toBe(Globe);
    expect(getToolIcon("SomeUnknownTool", { other: true })).toBe(Wrench);
  });
});

describe("getToolLabel", () => {
  it("returns active label for known tools", () => {
    expect(getToolLabel("Read", "active")).toBe("Reading...");
    expect(getToolLabel("Bash", "active")).toBe("Running...");
    expect(getToolLabel("Edit", "active")).toBe("Editing...");
    expect(getToolLabel("Write", "active")).toBe("Writing...");
  });

  it("returns past label for known tools", () => {
    expect(getToolLabel("Read", "past")).toBe("Read file");
    expect(getToolLabel("Bash", "past")).toBe("Ran command");
    expect(getToolLabel("Grep", "past")).toBe("Searched content");
  });

  it("returns failure label for known tools", () => {
    expect(getToolLabel("Read", "failure")).toBe("read file");
    expect(getToolLabel("Bash", "failure")).toBe("run command");
    expect(getToolLabel("WebSearch", "failure")).toBe("search web");
  });

  it("returns fallback for unknown tool - active", () => {
    expect(getToolLabel("CustomTool", "active")).toBe("Running CustomTool...");
  });

  it("returns fallback for unknown tool - past", () => {
    expect(getToolLabel("CustomTool", "past")).toBe("Used CustomTool");
  });

  it("returns fallback for unknown tool - failure", () => {
    expect(getToolLabel("CustomTool", "failure")).toBe("use CustomTool");
  });
});

describe("resolveRegisteredToolName", () => {
  it("returns the registry id for exact and ACP-style titles", () => {
    expect(resolveRegisteredToolName("Read")).toBe("Read");
    expect(resolveRegisteredToolName("Read routes.go")).toBe("Read");
    expect(resolveRegisteredToolName("Bash")).toBe("Bash");
  });

  it("returns the trimmed name for unknown tools", () => {
    expect(resolveRegisteredToolName("agh__skill_view")).toBe("agh__skill_view");
    expect(resolveRegisteredToolName("  custom  ")).toBe("custom");
  });

  it("returns tool for empty input", () => {
    expect(resolveRegisteredToolName("")).toBe("tool");
  });
});

describe("getToolCompactSummary", () => {
  it("extracts command from Bash input", () => {
    expect(getToolCompactSummary("Bash", { command: "ls -la" })).toBe("ls -la");
  });

  it("extracts file_path from Read input", () => {
    expect(getToolCompactSummary("Read", { file_path: "/src/index.ts" })).toBe("/src/index.ts");
  });

  it("extracts pattern from Grep input", () => {
    expect(getToolCompactSummary("Grep", { pattern: "TODO|FIXME" })).toBe("TODO|FIXME");
  });

  it("extracts pattern from Glob input", () => {
    expect(getToolCompactSummary("Glob", { pattern: "**/*.ts" })).toBe("**/*.ts");
  });

  it("extracts query from WebSearch input", () => {
    expect(getToolCompactSummary("WebSearch", { query: "React hooks" })).toBe("React hooks");
  });

  it("truncates long strings", () => {
    const longCommand = "a".repeat(100);
    const result = getToolCompactSummary("Bash", { command: longCommand });
    expect(result!.length).toBeLessThanOrEqual(80);
    expect(result!.endsWith("\u2026")).toBe(true);
  });

  it("returns undefined for unknown tools", () => {
    expect(getToolCompactSummary("UnknownTool", { data: "stuff" })).toBeUndefined();
  });

  it("returns undefined when toolInput is undefined", () => {
    expect(getToolCompactSummary("Bash")).toBeUndefined();
  });
});
