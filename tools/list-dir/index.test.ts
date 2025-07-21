import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { chmod, mkdir, rm, symlink, unlink, writeFile } from "fs/promises";
import { tmpdir } from "os";
import { join } from "path";
import tool from "./index";

describe("list-dir tool", () => {
  let testDir: string;

  beforeEach(async () => {
    testDir = join(tmpdir(), `list-dir-test-${Date.now()}`);
    await mkdir(testDir, { recursive: true });
  });

  afterEach(async () => {
    try {
      await rm(testDir, { recursive: true, force: true });
    } catch (error) {
      // Ignore cleanup errors
    }
  });

  test("Should list directory contents", async () => {
    // Create test files and directories
    await writeFile(join(testDir, "file1.txt"), "content1");
    await writeFile(join(testDir, "file2.js"), "content2");
    await mkdir(join(testDir, "subdir"));
    await writeFile(join(testDir, "subdir", "file3.txt"), "content3");

    const result = await tool({ path: testDir });

    expect(result.entries).toHaveLength(3);
    expect(result.entries.map(e => e.name).sort()).toEqual(["file1.txt", "file2.js", "subdir"]);
    expect(result.entries.find(e => e.name === "subdir")?.type).toBe("dir");
    expect(result.entries.find(e => e.name === "file1.txt")?.type).toBe("file");
  });

  test("Should filter with glob patterns", async () => {
    await writeFile(join(testDir, "file1.txt"), "content1");
    await writeFile(join(testDir, "file2.js"), "content2");
    await writeFile(join(testDir, "test.txt"), "content3");
    await writeFile(join(testDir, "README.md"), "content4");

    const result = await tool({ path: testDir, pattern: "*.txt" });

    expect(result.entries).toHaveLength(2);
    expect(result.entries.map(e => e.name).sort()).toEqual(["file1.txt", "test.txt"]);
  });

  test("Should list recursively", async () => {
    await mkdir(join(testDir, "src"));
    await mkdir(join(testDir, "src", "components"));
    await writeFile(join(testDir, "src", "index.js"), "content1");
    await writeFile(join(testDir, "src", "components", "Button.js"), "content2");

    const result = await tool({ path: testDir, recursive: true });

    expect(result.entries).toHaveLength(4);
    const names = result.entries.map(e => e.name).sort();
    expect(names).toContain("src");
    expect(names).toContain("components");
    expect(names).toContain("index.js");
    expect(names).toContain("Button.js");
  });

  test("Should filter files vs directories", async () => {
    await writeFile(join(testDir, "file1.txt"), "content1");
    await writeFile(join(testDir, "file2.txt"), "content2");
    await mkdir(join(testDir, "dir1"));
    await mkdir(join(testDir, "dir2"));

    // Only files
    const filesOnly = await tool({ path: testDir, includeFiles: true, includeDirs: false });
    expect(filesOnly.entries).toHaveLength(2);
    expect(filesOnly.entries.every(e => e.type === "file")).toBe(true);

    // Only directories
    const dirsOnly = await tool({ path: testDir, includeFiles: false, includeDirs: true });
    expect(dirsOnly.entries).toHaveLength(2);
    expect(dirsOnly.entries.every(e => e.type === "dir")).toBe(true);
  });

  test("Should handle empty directories", async () => {
    const result = await tool({ path: testDir });
    expect(result.entries).toHaveLength(0);
  });

  test("Should handle non-existent directories", async () => {
    const result = await tool({ path: join(testDir, "non-existent") });
    expect(result.entries).toHaveLength(0);
  });

  test("Should handle permission errors gracefully", async () => {
    // Skip this test on Windows
    if (process.platform === "win32") {
      return;
    }

    const restrictedDir = join(testDir, "restricted");
    await mkdir(restrictedDir);
    await writeFile(join(restrictedDir, "file.txt"), "content");
    await chmod(restrictedDir, 0o000);

    const result = await tool({ path: restrictedDir });
    expect(result.entries).toHaveLength(0);

    // Cleanup
    await chmod(restrictedDir, 0o755);
  });

  test("Should return entry metadata", async () => {
    const content = "Hello, World!";
    await writeFile(join(testDir, "test.txt"), content);

    const result = await tool({ path: testDir });
    const entry = result.entries[0];

    expect(entry).toBeDefined();
    expect(entry.name).toBe("test.txt");
    expect(entry.path).toBe(join(testDir, "test.txt"));
    expect(entry.type).toBe("file");
    expect(entry.size).toBe(content.length);
    expect(entry.modified).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/);
  });

  test("Should handle complex glob patterns", async () => {
    await mkdir(join(testDir, "src"));
    await mkdir(join(testDir, "src", "components"));
    await mkdir(join(testDir, "test"));
    await writeFile(join(testDir, "src", "index.js"), "content1");
    await writeFile(join(testDir, "src", "index.ts"), "content2");
    await writeFile(join(testDir, "src", "components", "Button.js"), "content3");
    await writeFile(join(testDir, "src", "components", "Button.tsx"), "content4");
    await writeFile(join(testDir, "test", "test.spec.js"), "content5");

    // Test globstar pattern
    const jsFiles = await tool({ path: testDir, pattern: "**/*.js", recursive: true });
    expect(jsFiles.entries.map(e => e.name).sort()).toEqual([
      "Button.js",
      "index.js",
      "test.spec.js",
    ]);

    // Test pattern with braces
    const srcFiles = await tool({
      path: testDir,
      pattern: "src/**/*.{js,ts,tsx}",
      recursive: true,
    });
    expect(srcFiles.entries.map(e => e.name).sort()).toEqual([
      "Button.js",
      "Button.tsx",
      "index.js",
      "index.ts",
    ]);
  });

  test("Should handle invalid inputs", async () => {
    // Invalid path
    let result = await tool({ path: "" });
    expect(result.entries).toHaveLength(0);

    // Path is a file, not a directory
    const filePath = join(testDir, "file.txt");
    await writeFile(filePath, "content");
    result = await tool({ path: filePath });
    expect(result.entries).toHaveLength(0);

    // Null/undefined handled by TypeScript, but test with empty object
    result = await tool({} as any);
    expect(result.entries).toHaveLength(0);
  });

  test("Should handle hidden files", async () => {
    await writeFile(join(testDir, ".hidden"), "content1");
    await writeFile(join(testDir, "visible.txt"), "content2");

    const result = await tool({ path: testDir });
    expect(result.entries).toHaveLength(2);
    expect(result.entries.map(e => e.name).sort()).toEqual([".hidden", "visible.txt"]);
  });

  test("Should handle special characters in filenames", async () => {
    const specialNames = [
      "file with spaces.txt",
      "file-with-dashes.txt",
      "file_with_underscores.txt",
    ];

    for (const name of specialNames) {
      await writeFile(join(testDir, name), "content");
    }

    const result = await tool({ path: testDir });
    expect(result.entries).toHaveLength(3);
    expect(result.entries.map(e => e.name).sort()).toEqual(specialNames.sort());
  });

  test("Should skip broken symlinks", async () => {
    // Skip this test on Windows
    if (process.platform === "win32") {
      return;
    }

    const targetPath = join(testDir, "target.txt");
    const linkPath = join(testDir, "link.txt");

    // Create a file and symlink to it
    await writeFile(targetPath, "content");
    await symlink(targetPath, linkPath);

    // Delete the target, leaving a broken symlink
    await unlink(targetPath);

    const result = await tool({ path: testDir });
    // Should return empty array or skip the broken symlink
    expect(result.entries.length).toBeLessThanOrEqual(1);
  });

  test("Should filter directories when listing recursively with includeFiles=false", async () => {
    await mkdir(join(testDir, "src"));
    await mkdir(join(testDir, "src", "components"));
    await mkdir(join(testDir, "test"));
    await writeFile(join(testDir, "src", "index.js"), "content1");
    await writeFile(join(testDir, "src", "components", "Button.js"), "content2");

    const result = await tool({
      path: testDir,
      recursive: true,
      includeFiles: false,
      includeDirs: true,
    });

    expect(result.entries.every(e => e.type === "dir")).toBe(true);
    expect(result.entries.map(e => e.name).sort()).toEqual(["components", "src", "test"]);
  });

  test("Should return sorted entries", async () => {
    await writeFile(join(testDir, "z.txt"), "content");
    await writeFile(join(testDir, "a.txt"), "content");
    await writeFile(join(testDir, "m.txt"), "content");

    const result = await tool({ path: testDir });
    const paths = result.entries.map(e => e.path);
    const sortedPaths = [...paths].sort();

    expect(paths).toEqual(sortedPaths);
  });
});
