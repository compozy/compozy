import { describe, expect, it } from "vitest";

import {
  detectFilePath,
  filePathBasename,
  resolveFilePath,
  type FilePathToken,
} from "../file-path-links";

describe("detectFilePath", () => {
  // UT-030. Invariant: only whole-token path grammar qualifies, keeping `:line[:col]`.
  it.each<[string, "inline_code" | "href", Partial<FilePathToken>]>([
    [
      "internal/mcp/hosted_proxy.go:42",
      "inline_code",
      { path: "internal/mcp/hosted_proxy.go", line: 42, relative: true },
    ],
    ["/tmp/x", "inline_code", { path: "/tmp/x", relative: false }],
    ["~/.compozy/config.toml", "inline_code", { path: "~/.compozy/config.toml", relative: false }],
    ["./a.ts:3:7", "inline_code", { path: "./a.ts", line: 3, column: 7, relative: true }],
    ["main.go:42", "inline_code", { path: "main.go", line: 42, relative: true }],
    [".github/workflows/ci.yml", "inline_code", { path: ".github/workflows/ci.yml" }],
    ["file:///Users/d/a%20b.md", "href", { path: "/Users/d/a b.md", relative: false }],
    ["file://localhost/tmp/x.md", "href", { path: "/tmp/x.md", relative: false }],
  ])("Should detect %j as a path", (text, source, expected) => {
    expect(detectFilePath(text, source)).toMatchObject({ raw: text, ...expected });
  });

  it.each<[string, "inline_code" | "href"]>([
    ["file://evil.host/x", "href"],
    ["file:///tmp/x.md", "inline_code"],
    ["and/or", "inline_code"],
    ["1/2", "inline_code"],
    ["@compozy/ui", "inline_code"],
    ["compozy/compozy", "inline_code"],
    ["https://x.dev/a.ts", "href"],
    ["https://x.dev/a.ts", "inline_code"],
    ["foo bar/baz.ts", "inline_code"],
    ["Next.js", "inline_code"],
    ["config.toml", "inline_code"],
    ["/", "inline_code"],
    ["~/", "inline_code"],
    ["a.ts:0", "inline_code"],
    [`/${"a".repeat(1024)}`, "inline_code"],
    ["\\\\host\\share\\x.txt", "inline_code"],
    ["\\\\?\\C:\\x", "inline_code"],
    ["\\\\.\\pipe\\x", "inline_code"],
    ["//host/x.ts", "inline_code"],
    ["", "inline_code"],
  ])("Should not detect %j", (text, source) => {
    expect(detectFilePath(text, source)).toBeNull();
  });
  // FR-004. Invariant: drive paths name a file only on a Windows desktop.
  it.each<[string, "inline_code" | "href"]>([
    ["C:\\Users\\a\\b.txt", "inline_code"],
    ["C:/Users/a/b.txt:3", "inline_code"],
    ["file:///C:/Users/a/b.txt", "href"],
  ])("Should not detect the drive path %j off Windows", (text, source) => {
    expect(detectFilePath(text, source)).toBeNull();
    expect(detectFilePath(text, source, { windows: false })).toBeNull();
    expect(detectFilePath(text, source, { windows: true })).toMatchObject({ relative: false });
    expect(detectFilePath("/tmp/x", source, { windows: false })).toMatchObject({
      path: "/tmp/x",
    });
  });
});

describe("resolveFilePath", () => {
  const token = (text: string, windows = false) => {
    const detected = detectFilePath(text, "inline_code", { windows });
    if (!detected) throw new Error(`not a path: ${text}`);
    return detected;
  };

  // UT-031. Invariant: relative paths join working dir, then worktree, then workspace root.
  it("Should resolve relative paths against the first known base", () => {
    expect(
      resolveFilePath(token("src/app.tsx"), { workingDir: "/w/web", worktreePath: "/wt" })
    ).toBe("/w/web/src/app.tsx");
    expect(
      resolveFilePath(token("src/app.tsx"), { worktreePath: "/wt", workspaceRoot: "/root" })
    ).toBe("/wt/src/app.tsx");
    expect(resolveFilePath(token("src/app.tsx"), { workspaceRoot: "/root" })).toBe(
      "/root/src/app.tsx"
    );
    expect(resolveFilePath(token("../x.ts"), { workingDir: "/w/web" })).toBe("/w/x.ts");
    expect(resolveFilePath(token("./a/../b.ts:3"), { workingDir: "/w/" })).toBe("/w/b.ts");
  });

  it("Should return null for relative paths without a base and keep absolute paths", () => {
    expect(resolveFilePath(token("src/app.tsx"), {})).toBeNull();
    expect(resolveFilePath(token("/tmp/a/./b.md"), {})).toBe("/tmp/a/b.md");
    expect(resolveFilePath(token("~/notes.md"), {})).toBe("~/notes.md");
    expect(resolveFilePath(token("C:\\Users\\a\\b.txt", true), {})).toBe("C:\\Users\\a\\b.txt");
  });

  // FR-004. Invariant: only a Windows desktop treats `\\` as a separator.
  it("Should keep POSIX backslashes literal and normalize Windows separators", () => {
    expect(resolveFilePath(token("/repo/a\\b.ts"), {})).toBe("/repo/a\\b.ts");
    expect(resolveFilePath(token("a\\b.ts:3"), { workingDir: "/repo" })).toBe("/repo/a\\b.ts");
    expect(filePathBasename("/repo/a\\b.ts")).toBe("a\\b.ts");
    expect(resolveFilePath(token("C:\\repo\\..\\x.ts", true), {}, { windows: true })).toBe(
      "C:\\x.ts"
    );
    expect(
      resolveFilePath(token("..\\x.ts:1", true), { workingDir: "C:\\repo\\web" }, { windows: true })
    ).toBe("C:\\repo\\x.ts");
    expect(filePathBasename("C:\\repo\\x.ts", { windows: true })).toBe("x.ts");
  });

  // FR-004. Invariant: a Windows `\\` marks a relative path exactly like `/` does.
  it("Should detect Windows backslash-relative paths like their slash forms", () => {
    for (const text of ["src\\main.ts", ".\\main.ts", "..\\lib\\x.ts", ".\\a/b.ts"]) {
      expect(detectFilePath(text, "inline_code", { windows: true })).toMatchObject({
        path: text,
        relative: true,
      });
    }
    expect(detectFilePath("..\\", "inline_code", { windows: true })).toBeNull();
    expect(detectFilePath(".\\..\\", "inline_code", { windows: true })).toBeNull();
    expect(detectFilePath("src\\main.ts", "inline_code", { windows: false })).toBeNull();
    expect(detectFilePath(".\\main.ts", "inline_code", { windows: false })).toBeNull();
    expect(
      resolveFilePath(token("src\\main.ts", true), { workingDir: "C:\\repo" }, { windows: true })
    ).toBe("C:\\repo\\src\\main.ts");
  });
});
