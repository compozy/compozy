import { afterAll, beforeAll, describe, expect, test } from "bun:test";
import { promises as fs } from "fs";
import { tmpdir } from "os";
import { join } from "path";
import { readFile } from "./index";

describe("readFile tool", () => {
  let testDir: string;
  let testFile: string;
  let protectedFile: string;
  let testDirPath: string;

  beforeAll(async () => {
    // Create temporary test directory
    testDir = await fs.mkdtemp(join(tmpdir(), "compozy-test-"));
    testFile = join(testDir, "test.txt");
    protectedFile = join(testDir, "protected.txt");
    testDirPath = join(testDir, "subdir");

    // Create test file
    await fs.writeFile(testFile, "Hello, Compozy!");

    // Create a subdirectory for directory test
    await fs.mkdir(testDirPath);

    // Create protected file and make it unreadable (if possible)
    await fs.writeFile(protectedFile, "Secret content");
    try {
      await fs.chmod(protectedFile, 0o000);
    } catch {
      // Some systems may not support chmod
    }
  });

  afterAll(async () => {
    // Restore permissions before cleanup
    try {
      await fs.chmod(protectedFile, 0o644);
    } catch {
      // Ignore if chmod is not supported
    }

    // Clean up test directory
    await fs.rm(testDir, { recursive: true, force: true });
  });

  test("Should successfully read a file", async () => {
    const result = await readFile({ path: testFile });
    expect(result).toEqual({
      content: "Hello, Compozy!",
    });
  });

  test("Should handle non-existent files", async () => {
    const result = await readFile({ path: join(testDir, "non-existent.txt") });
    expect(result).toEqual({
      error: `File not found: ${join(testDir, "non-existent.txt")}`,
      code: "ENOENT",
    });
  });

  test("Should handle permission errors", async () => {
    // Skip this test if chmod is not supported or we're on Windows
    if (process.platform === "win32") {
      return;
    }

    try {
      await fs.access(protectedFile, fs.constants.R_OK);
      // If we can read it, chmod didn't work, skip the test
      return;
    } catch {
      // Good, we can't read it
    }

    const result = await readFile({ path: protectedFile });
    expect(result).toEqual({
      error: `Permission denied: ${protectedFile}`,
      code: "EACCES",
    });
  });

  test("Should handle directory paths", async () => {
    const result = await readFile({ path: testDirPath });
    expect(result).toEqual({
      error: `Path is a directory, not a file: ${testDirPath}`,
      code: "EISDIR",
    });
  });

  test("Should handle invalid input - null input", async () => {
    const result = await readFile(null as any);
    expect(result).toEqual({
      error: "Invalid input: input must be an object",
    });
  });

  test("Should handle invalid input - undefined input", async () => {
    const result = await readFile(undefined as any);
    expect(result).toEqual({
      error: "Invalid input: input must be an object",
    });
  });

  test("Should handle invalid input - string input", async () => {
    const result = await readFile("path/to/file" as any);
    expect(result).toEqual({
      error: "Invalid input: input must be an object",
    });
  });

  test("Should handle invalid input - missing path", async () => {
    const result = await readFile({} as any);
    expect(result).toEqual({
      error: "Invalid input: path must be a non-empty string",
    });
  });

  test("Should handle invalid input - empty path", async () => {
    const result = await readFile({ path: "" });
    expect(result).toEqual({
      error: "Invalid input: path must be a non-empty string",
    });
  });

  test("Should handle invalid input - non-string path", async () => {
    const result = await readFile({ path: 123 as any });
    expect(result).toEqual({
      error: "Invalid input: path must be a non-empty string",
    });
  });

  test("Should handle relative paths", async () => {
    // Create a file in current directory
    const relativeFile = "test-relative.txt";
    await fs.writeFile(relativeFile, "Relative content");

    try {
      const result = await readFile({ path: relativeFile });
      expect(result).toEqual({
        content: "Relative content",
      });
    } finally {
      // Clean up
      await fs.unlink(relativeFile);
    }
  });

  test("Should trim whitespace from path", async () => {
    const result = await readFile({ path: `  ${testFile}  ` });
    expect(result).toEqual({
      content: "Hello, Compozy!",
    });
  });
});
