import { afterEach, beforeEach, describe, expect, it } from "bun:test";
import { chmod, mkdir, rm, writeFile } from "fs/promises";
import { tmpdir } from "os";
import { join } from "path";
import deleteFile from "./index";

describe("deleteFile", () => {
  let tempDir: string;

  beforeEach(async () => {
    tempDir = join(tmpdir(), `compozy-test-${Date.now()}`);
    await mkdir(tempDir, { recursive: true });
  });

  afterEach(async () => {
    try {
      await rm(tempDir, { recursive: true, force: true });
    } catch {
      // Ignore cleanup errors
    }
  });

  it("Should successfully delete an existing file", async () => {
    const testFile = join(tempDir, "test.txt");
    await writeFile(testFile, "test content");

    const result = await deleteFile({ path: testFile });

    expect(result.success).toBe(true);

    // Verify file was actually deleted
    const fileExists = await Bun.file(testFile).exists();
    expect(fileExists).toBe(false);
  });

  it("Should return false for non-existent file", async () => {
    const nonExistentFile = join(tempDir, "does-not-exist.txt");

    const result = await deleteFile({ path: nonExistentFile });

    expect(result.success).toBe(false);
  });

  it("Should handle permission errors gracefully", async () => {
    const testFile = join(tempDir, "readonly.txt");
    await writeFile(testFile, "test content");

    // Create a read-only directory to test permission errors
    const readOnlyDir = join(tempDir, "readonly-dir");
    await mkdir(readOnlyDir);
    const fileInReadOnlyDir = join(readOnlyDir, "file.txt");
    await writeFile(fileInReadOnlyDir, "content");

    // Make directory read-only (no write permissions)
    await chmod(readOnlyDir, 0o444);

    const result = await deleteFile({ path: fileInReadOnlyDir });

    expect(result.success).toBe(false);

    // Cleanup: restore permissions
    await chmod(readOnlyDir, 0o755);
  });

  it("Should throw error for invalid input", async () => {
    // Test missing input
    await expect(deleteFile(null as any)).rejects.toThrow("Invalid input: path must be a string");

    // Test missing path property
    await expect(deleteFile({} as any)).rejects.toThrow("Invalid input: path must be a string");

    // Test non-string path
    await expect(deleteFile({ path: 123 as any })).rejects.toThrow(
      "Invalid input: path must be a string"
    );

    // Test empty path
    await expect(deleteFile({ path: "" })).rejects.toThrow("Invalid input: path cannot be empty");

    // Test whitespace-only path
    await expect(deleteFile({ path: "   " })).rejects.toThrow(
      "Invalid input: path cannot be empty"
    );
  });

  it("Should handle file paths with spaces", async () => {
    const testFile = join(tempDir, "file with spaces.txt");
    await writeFile(testFile, "test content");

    const result = await deleteFile({ path: testFile });

    expect(result.success).toBe(true);
  });

  it("Should handle nested directory paths", async () => {
    const nestedDir = join(tempDir, "nested", "directory");
    await mkdir(nestedDir, { recursive: true });
    const testFile = join(nestedDir, "test.txt");
    await writeFile(testFile, "test content");

    const result = await deleteFile({ path: testFile });

    expect(result.success).toBe(true);
  });

  it("Should handle special characters in file names", async () => {
    const testFile = join(tempDir, "file-with_special.chars!@#.txt");
    await writeFile(testFile, "test content");

    const result = await deleteFile({ path: testFile });

    expect(result.success).toBe(true);
  });
});
