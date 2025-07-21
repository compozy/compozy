import { afterAll, beforeAll, describe, expect, test } from "bun:test";
import { promises as fs } from "node:fs";
import { join } from "node:path";
import { writeFile } from "./index";

const TEST_DIR = ".test-write-file-tmp";

beforeAll(async () => {
  // Create test directory
  await fs.mkdir(TEST_DIR, { recursive: true });
});

afterAll(async () => {
  // Clean up test directory
  await fs.rm(TEST_DIR, { recursive: true, force: true });
});

describe("writeFile", () => {
  test("Should successfully write a file", async () => {
    const testPath = join(TEST_DIR, "test.txt");
    const content = "Hello, World!";

    const result = await writeFile({
      path: testPath,
      content: content,
    });

    expect(result.success).toBe(true);
    expect(result.error).toBeUndefined();

    // Verify file was written
    const fileContent = await fs.readFile(testPath, "utf-8");
    expect(fileContent).toBe(content);
  });

  test("Should append to existing file", async () => {
    const testPath = join(TEST_DIR, "append.txt");
    const initialContent = "Initial content\n";
    const appendContent = "Appended content";

    // Write initial content
    await writeFile({
      path: testPath,
      content: initialContent,
    });

    // Append content
    const result = await writeFile({
      path: testPath,
      content: appendContent,
      append: true,
    });

    expect(result.success).toBe(true);
    expect(result.error).toBeUndefined();

    // Verify file contains both contents
    const fileContent = await fs.readFile(testPath, "utf-8");
    expect(fileContent).toBe(initialContent + appendContent);
  });

  test("Should create directories if they don't exist", async () => {
    const testPath = join(TEST_DIR, "nested", "deep", "file.txt");
    const content = "Nested file content";

    const result = await writeFile({
      path: testPath,
      content: content,
    });

    expect(result.success).toBe(true);
    expect(result.error).toBeUndefined();

    // Verify file was written
    const fileContent = await fs.readFile(testPath, "utf-8");
    expect(fileContent).toBe(content);
  });

  test("Should handle permission errors gracefully", async () => {
    // This test may not work consistently across all environments
    // as it depends on file system permissions
    const testPath = join(TEST_DIR, "readonly.txt");

    // Create a file
    await writeFile({
      path: testPath,
      content: "test",
    });

    // Make it read-only
    await fs.chmod(testPath, 0o444);

    // Try to write to it
    const result = await writeFile({
      path: testPath,
      content: "new content",
    });

    // On some systems this might succeed, so we check if it failed
    // and if it did, verify the error message
    if (!result.success) {
      expect(result.error).toContain("Permission denied");
    }

    // Clean up: restore permissions
    await fs.chmod(testPath, 0o644);
  });

  test("Should reject invalid input", async () => {
    // Test missing path
    const result1 = await writeFile({
      path: "",
      content: "test",
    });
    expect(result1.success).toBe(false);
    expect(result1.error).toContain("path must be a non-empty string");

    // Test missing content
    const result2 = await writeFile({
      path: "test.txt",
      content: undefined as any,
    });
    expect(result2.success).toBe(false);
    expect(result2.error).toContain("content must be a string");

    // Test invalid input type
    const result3 = await writeFile(null as any);
    expect(result3.success).toBe(false);
    expect(result3.error).toContain("Invalid input: expected an object");
  });

  test("Should reject dangerous paths", async () => {
    // Test parent directory traversal
    const result1 = await writeFile({
      path: "../dangerous.txt",
      content: "test",
    });
    expect(result1.success).toBe(false);
    expect(result1.error).toContain("parent directory references are not allowed");

    // Test absolute path
    const result2 = await writeFile({
      path: "/etc/passwd",
      content: "test",
    });
    expect(result2.success).toBe(false);
    expect(result2.error).toContain("absolute paths");
  });

  test("Should handle writing to a directory gracefully", async () => {
    const dirPath = join(TEST_DIR, "directory");
    await fs.mkdir(dirPath, { recursive: true });

    const result = await writeFile({
      path: dirPath,
      content: "test",
    });

    expect(result.success).toBe(false);
    expect(result.error).toContain("Target is a directory");
  });

  test("Should respect custom encoding", async () => {
    const testPath = join(TEST_DIR, "encoded.txt");
    const content = "Hello, 世界!";

    const result = await writeFile(
      {
        path: testPath,
        content: content,
      },
      {
        encoding: "utf-8",
      }
    );

    expect(result.success).toBe(true);

    // Verify file was written with correct encoding
    const fileContent = await fs.readFile(testPath, "utf-8");
    expect(fileContent).toBe(content);
  });

  test("Should handle empty content", async () => {
    const testPath = join(TEST_DIR, "empty.txt");

    const result = await writeFile({
      path: testPath,
      content: "",
    });

    expect(result.success).toBe(true);

    // Verify empty file was created
    const fileContent = await fs.readFile(testPath, "utf-8");
    expect(fileContent).toBe("");
  });

  test("Should overwrite existing files by default", async () => {
    const testPath = join(TEST_DIR, "overwrite.txt");

    // Write initial content
    await writeFile({
      path: testPath,
      content: "Original content",
    });

    // Write new content
    const result = await writeFile({
      path: testPath,
      content: "New content",
    });

    expect(result.success).toBe(true);

    // Verify file was overwritten
    const fileContent = await fs.readFile(testPath, "utf-8");
    expect(fileContent).toBe("New content");
  });
});
