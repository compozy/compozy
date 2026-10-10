import { describe, it, expect } from "vitest";
import {
  getToolIcon,
  getToolLabel,
  getToolCompactSummary,
  humanizeToolId,
  resolveRegisteredToolName,
} from "../tool-labels";
import { resolveToolDisplay } from "../tool-display";
import { inferShellIntent } from "../tool-shell-intent";
import {
  Bot,
  FileEdit,
  FileText,
  FolderSearch,
  Globe,
  Search,
  SlidersHorizontal,
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

  it("Should map CompozyOS native tool families from the compozy__ taxonomy", () => {
    expect(getToolIcon("compozy__edit")).toBe(FileEdit);
    expect(getToolIcon("compozy__config_set")).toBe(SlidersHorizontal);
    // Unmapped native family falls through to the generic tool glyph.
    expect(getToolIcon("compozy__deny_native")).toBe(Wrench);
    expect(getToolIcon("compozy__terminal_exec")).toBe(Terminal);
    expect(getToolIcon("compozy__terminal_open")).toBe(Terminal);
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
    expect(getToolLabel("Read", "active")).toBe("Reading…");
    expect(getToolLabel("Bash", "active")).toBe("Running…");
    expect(getToolLabel("Edit", "active")).toBe("Editing…");
    expect(getToolLabel("Write", "active")).toBe("Writing…");
  });

  it("returns past label for known tools", () => {
    expect(getToolLabel("Read", "past")).toBe("Read file");
    expect(getToolLabel("Bash", "past")).toBe("Ran command");
    expect(getToolLabel("Grep", "past")).toBe("Searched content");
    expect(getToolLabel("compozy__terminal_exec", "past")).toBe("Used terminal");
    expect(getToolLabel("compozy__terminal_open", "past")).toBe("Opened terminal");
  });

  it("returns failure label for known tools", () => {
    expect(getToolLabel("Read", "failure")).toBe("read file");
    expect(getToolLabel("Bash", "failure")).toBe("run command");
    expect(getToolLabel("WebSearch", "failure")).toBe("search web");
  });

  it("returns fallback for unknown tool - active", () => {
    expect(getToolLabel("CustomTool", "active")).toBe("Running CustomTool…");
  });

  it("returns fallback for unknown tool - past", () => {
    expect(getToolLabel("CustomTool", "past")).toBe("Used CustomTool");
  });

  it("returns fallback for unknown tool - failure", () => {
    expect(getToolLabel("CustomTool", "failure")).toBe("use CustomTool");
  });
});

describe("humanizeToolId", () => {
  it("Should read an uncatalogued tool id as plain words, never the raw id", () => {
    expect(humanizeToolId("compozy__config_set")).toBe("config set");
    expect(humanizeToolId("mcp__github__create_issue")).toBe("create issue (Github)");
    expect(humanizeToolId("mcp__compozy__compozy__config_set")).toBe("config set");
    expect(humanizeToolId("CustomTool")).toBe("CustomTool");
    expect(getToolLabel("compozy__config_set", "past")).toBe("Used config set");
    expect(getToolLabel("mcp__github__create_issue", "active")).toBe(
      "Running create issue (Github)…"
    );
  });

  it("Should render a retired native tool id from an old session with the generic glyph and a humanized label", () => {
    expect(getToolIcon("compozy__memory_note")).toBe(Wrench);
    expect(humanizeToolId("compozy__memory_note")).toBe("memory note");
    expect(getToolLabel("compozy__memory_note", "past")).toBe("Used memory note");
  });
});

describe("resolveRegisteredToolName", () => {
  it("resolves exact identities while preserving free-form titles", () => {
    expect(resolveRegisteredToolName("Read")).toBe("Read");
    expect(resolveRegisteredToolName("Read routes.go")).toBe("Read routes.go");
    expect(resolveRegisteredToolName("Bash")).toBe("Bash");
  });

  it("returns the trimmed name for unknown tools", () => {
    expect(resolveRegisteredToolName("compozy__skill_view")).toBe("compozy__skill_view");
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

  it("Should summarize a terminal open by its title and never by an id or argv", () => {
    expect(getToolCompactSummary("compozy__terminal_open", { title: "dev server" })).toBe(
      "dev server"
    );
    expect(
      getToolCompactSummary("compozy__terminal_exec", { command: "bun run dev" })
    ).toBeUndefined();
  });

  it("returns undefined for unknown tools", () => {
    expect(getToolCompactSummary("UnknownTool", { data: "stuff" })).toBeUndefined();
  });

  it("returns undefined when toolInput is undefined", () => {
    expect(getToolCompactSummary("Bash")).toBeUndefined();
  });
});

// Invariant: display-only normalization is one line, grapheme-safe, and never invents tool identity.
// Owner: session presentation; canonical suite: tool-labels.
describe("provider summary presentation", () => {
  it("Should treat a script title as an unknown tool and preserve explicit native identity", () => {
    const title = "python3 - <<'PY'\n" + "print('hello')\n".repeat(30) + "PY";
    expect(getToolLabel(title, "active")).toBe("Running tool…");
    expect(getToolLabel(title, "past")).toBe("Used tool");
    expect(resolveRegisteredToolName(title)).toBe(title);
    expect(resolveRegisteredToolName("mcp__host__compozy__terminal_exec")).toBe(
      "compozy__terminal_exec"
    );
    expect(getToolLabel("Bash dependency investigation", "past")).toBe("Used tool");
  });
  it("Should normalize multiline input and truncate between complete graphemes", () => {
    const emoji = "👩🏽‍💻";
    const command = emoji.repeat(90) + "\nraw-tail";
    expect(getToolCompactSummary("Bash", { command })).toBe(emoji.repeat(79) + "…");
    expect(getToolCompactSummary("Bash", { command: "first\n\tsecond" })).toBe("first second");
    expect(command).toContain("\nraw-tail");
  });
});

describe("subagent tool labels (UT-W09)", () => {
  it("Should speak the subagent family in verbs, per tense", () => {
    expect(getToolLabel("compozy__subagent_delegate", "active")).toBe("Delegating a subagent");
    expect(getToolLabel("compozy__subagent_delegate", "past")).toBe("Delegated a subagent");
    expect(getToolLabel("compozy__subagent_capabilities", "active")).toBe(
      "Checking subagent capabilities"
    );
    expect(getToolLabel("compozy__subagent_status", "past")).toBe("Read subagent status");
    expect(getToolLabel("compozy__subagent_cancel", "past")).toBe("Canceled a subagent");
    expect(getToolIcon("compozy__subagent_delegate")).toBe(Bot);
  });
});

// Invariant: a row reads as verb + object, and its kind follows the registered
// name first, then the input shape — never a provider's display title.
describe("resolveToolDisplay", () => {
  const past = (toolName: string, args: Record<string, unknown> = {}, toolTitle?: string) =>
    resolveToolDisplay({ toolName, args, ...(toolTitle ? { toolTitle } : {}) }, "past");

  it("Should split catalogued tools into a tense-aware verb and a mono object", () => {
    expect(past("Read", { file_path: "web/src/app.tsx" })).toMatchObject({
      kind: "read",
      verb: "Read",
      target: "web/src/app.tsx",
      targetKind: "file",
    });
    expect(past("Bash", { command: "cd /repo && go test ./..." })).toMatchObject({
      kind: "command",
      verb: "Ran",
      target: "go test ./...",
      targetKind: "code",
    });
    expect(
      resolveToolDisplay({ toolName: "Grep", args: { pattern: "TODO" } }, "active")
    ).toMatchObject({ kind: "search", verb: "Searching", target: "TODO" });
  });

  it("Should infer the kind of a title-named or uncatalogued tool from its input", () => {
    expect(past("Terminal", { command: "bun test" })).toMatchObject({
      kind: "command",
      verb: "Ran",
      target: "bun test",
    });
    expect(past("Preparing file…", { file_path: "a.md", content: "# A" })).toMatchObject({
      kind: "edit",
      verb: "Wrote",
      target: "a.md",
    });
    expect(
      past("mcp__compozy-hosted-tools__compozy__skill_view", { name: "compozy" })
    ).toMatchObject({ kind: "other", verb: "Used", target: "skill view", targetKind: "text" });
  });

  it("Should read simple read/list/search shell commands as that action", () => {
    expect(past("Bash", { command: "sed -n '1,40p' web/src/app.tsx" })).toMatchObject({
      kind: "read",
      verb: "Read",
      target: "web/src/app.tsx",
    });
    expect(past("Bash", { command: "ls web/src" })).toMatchObject({
      verb: "Listed",
      target: "web/src",
    });
    expect(past("Bash", { command: "rg -n TODO web | head -20" })).toMatchObject({
      kind: "search",
      target: "TODO",
    });
  });

  it("Should keep a long provider title out of the verb and bound the object", () => {
    const title = "Inspect the layout\n" + "x".repeat(400);
    const display = past(title);
    expect(display.verb).toBe("Used tool");
    expect(display.target).toBe(display.target?.slice(0, 120));
    expect(display.target).not.toContain("\n");
  });
});

describe("inferShellIntent", () => {
  it("Should stay the raw command whenever the command does more than read", () => {
    expect(inferShellIntent("cat a.ts > b.ts")).toBeNull();
    expect(inferShellIntent("cat a.ts && rm a.ts")).toBeNull();
    expect(inferShellIntent("sed -i 's/a/b/' a.ts")).toBeNull();
    expect(inferShellIntent("cat $(ls)")).toBeNull();
    expect(inferShellIntent("cat a.ts b.ts")).toBeNull();
    expect(inferShellIntent("git status")).toBeNull();
    expect(inferShellIntent("rg TODO | xargs rm")).toBeNull();
  });
});
