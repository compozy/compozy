import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { chmod, mkdir, rmdir, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { listFiles } from "./index";

describe("listFiles", () => {
  let testDir: string;

  beforeEach(async () => {
    // Create a unique test directory
    testDir = join(tmpdir(), `test-list-files-${Date.now()}`);
    await mkdir(testDir, { recursive: true });
  });

  afterEach(async () => {
    // Clean up test directory
    try {
      await rmdir(testDir, { recursive: true });
    } catch {
      // Ignore cleanup errors
    }
  });

  test("Should list files in a directory", async () => {
    // Create test files
    await writeFile(join(testDir, "file1.txt"), "content1");
    await writeFile(join(testDir, "file2.js"), "content2");
    await writeFile(join(testDir, "file3.md"), "content3");

    const result = await listFiles({ dir: testDir });

    expect(result.files).toEqual(["file1.txt", "file2.js", "file3.md"]);
  });

  test("Should return empty array for empty directory", async () => {
    const result = await listFiles({ dir: testDir });

    expect(result.files).toEqual([]);
  });

  test("Should return empty array for non-existent directory", async () => {
    const nonExistentDir = join(testDir, "does-not-exist");

    const result = await listFiles({ dir: nonExistentDir });

    expect(result.files).toEqual([]);
  });

  test("Should handle permission errors gracefully", async () => {
    // Skip this test on Windows as permission handling is different
    if (process.platform === "win32") {
      return;
    }

    const restrictedDir = join(testDir, "restricted");
    await mkdir(restrictedDir);
    await writeFile(join(restrictedDir, "file.txt"), "content");

    // Remove read permissions
    await chmod(restrictedDir, 0o000);

    const result = await listFiles({ dir: restrictedDir });

    expect(result.files).toEqual([]);

    // Restore permissions for cleanup
    await chmod(restrictedDir, 0o755);
  });

  test("Should handle invalid input gracefully", async () => {
    // Test with null
    // @ts-expect-error Testing invalid input
    const resultNull = await listFiles(null);
    expect(resultNull.files).toEqual([]);

    // Test with undefined
    // @ts-expect-error Testing invalid input
    const resultUndefined = await listFiles(undefined);
    expect(resultUndefined.files).toEqual([]);

    // Test with wrong type
    // @ts-expect-error Testing invalid input
    const resultWrongType = await listFiles({ dir: 123 });
    expect(resultWrongType.files).toEqual([]);
  });

  test("Should include hidden files", async () => {
    await writeFile(join(testDir, ".hidden"), "content");
    await writeFile(join(testDir, "visible.txt"), "content");

    const result = await listFiles({ dir: testDir });

    expect(result.files).toContain(".hidden");
    expect(result.files).toContain("visible.txt");
  });

  test("Should return files in alphabetical order", async () => {
    await writeFile(join(testDir, "zebra.txt"), "content");
    await writeFile(join(testDir, "apple.txt"), "content");
    await writeFile(join(testDir, "banana.txt"), "content");

    const result = await listFiles({ dir: testDir });

    expect(result.files).toEqual(["apple.txt", "banana.txt", "zebra.txt"]);
  });

  test("Should handle special characters in file names", async () => {
    const specialFiles = [
      "file with spaces.txt",
      "file-with-dashes.txt",
      "file_with_underscores.txt",
      "file.multiple.dots.txt",
    ];

    for (const fileName of specialFiles) {
      await writeFile(join(testDir, fileName), "content");
    }

    const result = await listFiles({ dir: testDir });

    expect(result.files.sort()).toEqual(specialFiles.sort());
  });

  test("Should exclude directories from results", async () => {
    await writeFile(join(testDir, "file.txt"), "content");
    await mkdir(join(testDir, "subdirectory"));
    await writeFile(join(testDir, "subdirectory", "nested.txt"), "content");

    const result = await listFiles({ dir: testDir });

    expect(result.files).toEqual(["file.txt"]);
    expect(result.files).not.toContain("subdirectory");
  });

  test("Should include symbolic links", async () => {
    // Skip this test on Windows as symlink handling is different
    if (process.platform === "win32") {
      return;
    }

    const targetFile = join(testDir, "target.txt");
    const linkFile = join(testDir, "link.txt");

    await writeFile(targetFile, "content");
    await symlink(targetFile, linkFile);

    const result = await listFiles({ dir: testDir });

    expect(result.files).toContain("target.txt");
    expect(result.files).toContain("link.txt");
  });
});
