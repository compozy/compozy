import { describe, expect, test, beforeAll, afterAll } from "bun:test";
import { promises as fs } from "fs";
import { join } from "path";
import { tmpdir } from "os";
import { grep } from "./index";

describe("grep tool", () => {
  let testDir: string;
  let testFile1: string;
  let testFile2: string;
  let subDir: string;
  let nestedFile: string;
  let binaryFile: string;
  let protectedFile: string;
  
  beforeAll(async () => {
    // Create temporary test directory structure
    testDir = await fs.mkdtemp(join(tmpdir(), "compozy-grep-test-"));
    testFile1 = join(testDir, "test1.txt");
    testFile2 = join(testDir, "test2.txt");
    subDir = join(testDir, "subdir");
    nestedFile = join(subDir, "nested.txt");
    binaryFile = join(testDir, "binary.bin");
    protectedFile = join(testDir, "protected.txt");
    
    // Create test files
    await fs.writeFile(testFile1, "Hello, Compozy!\nThis is a test file.\nPattern matching is fun!");
    await fs.writeFile(testFile2, "Another file with Compozy content.\nTesting grep functionality.\nHELLO in uppercase!");
    
    // Create subdirectory and nested file
    await fs.mkdir(subDir);
    await fs.writeFile(nestedFile, "Nested file content.\nCompozy works recursively!\nFind me with grep.");
    
    // Create binary file
    await fs.writeFile(binaryFile, Buffer.from([0x00, 0x01, 0x02, 0xFF, 0xFE]));
    
    // Create protected file
    await fs.writeFile(protectedFile, "Protected content");
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
  
  test("Should find matches in a single file", async () => {
    const result = await grep({ 
      pattern: "Compozy",
      path: testFile1 
    });
    expect(result).toHaveProperty("matches");
    if ("matches" in result) {
      expect(result.matches).toHaveLength(1);
      expect(result.matches[0]).toEqual({
        file: testFile1,
        line: 1,
        column: 8,
        text: "Hello, Compozy!",
        lineNumber: 1,
      });
    }
  });
  
  test("Should find multiple matches in a file", async () => {
    const result = await grep({ 
      pattern: "test|Test",
      path: testFile1 
    });
    expect(result).toHaveProperty("matches");
    if ("matches" in result) {
      expect(result.matches).toHaveLength(1);
      expect(result.matches[0].lineNumber).toBe(2);
      expect(result.matches[0].text).toBe("This is a test file.");
    }
  });
  
  test("Should search with case insensitive option", async () => {
    const result = await grep({ 
      pattern: "hello",
      path: testFile2,
      ignoreCase: true
    });
    expect(result).toHaveProperty("matches");
    if ("matches" in result) {
      expect(result.matches).toHaveLength(1);
      expect(result.matches[0].text).toBe("HELLO in uppercase!");
    }
  });
  
  test("Should search recursively in directories", async () => {
    const result = await grep({ 
      pattern: "Compozy",
      path: testDir,
      recursive: true
    });
    expect(result).toHaveProperty("matches");
    if ("matches" in result) {
      expect(result.matches.length).toBeGreaterThanOrEqual(3);
      const files = result.matches.map(m => m.file);
      expect(files).toContain(testFile1);
      expect(files).toContain(testFile2);
      expect(files).toContain(nestedFile);
    }
  });
  
  test("Should not search recursively by default", async () => {
    const result = await grep({ 
      pattern: "Compozy",
      path: testDir
    });
    expect(result).toHaveProperty("matches");
    if ("matches" in result) {
      const files = result.matches.map(m => m.file);
      expect(files).not.toContain(nestedFile);
    }
  });
  
  test("Should support regex patterns", async () => {
    const result = await grep({ 
      pattern: "\\b\\w+ing\\b",
      path: testFile1
    });
    expect(result).toHaveProperty("matches");
    if ("matches" in result) {
      expect(result.matches).toHaveLength(1);
      expect(result.matches[0].text).toBe("Pattern matching is fun!");
    }
  });
  
  test("Should handle binary files gracefully", async () => {
    const result = await grep({ 
      pattern: "test",
      path: binaryFile
    });
    expect(result).toHaveProperty("matches");
    if ("matches" in result) {
      expect(result.matches).toHaveLength(0);
    }
  });
  
  test("Should return empty matches when no matches found", async () => {
    const result = await grep({ 
      pattern: "nonexistent",
      path: testFile1
    });
    expect(result).toEqual({
      matches: [],
    });
  });
  
  test("Should handle permission errors gracefully", async () => {
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
    
    const result = await grep({ 
      pattern: "content",
      path: protectedFile
    });
    // Should either return empty matches or an error
    if ("error" in result) {
      expect(result.error).toContain("Permission denied");
    } else {
      expect(result.matches).toHaveLength(0);
    }
  });
  
  test("Should respect maxResults limit", async () => {
    const result = await grep({ 
      pattern: "Compozy",
      path: testDir,
      recursive: true,
      maxResults: 2
    });
    expect(result).toHaveProperty("matches");
    if ("matches" in result) {
      expect(result.matches).toHaveLength(2);
    }
  });
  
  test("Should handle invalid regex patterns", async () => {
    const result = await grep({ 
      pattern: "[",
      path: testFile1
    });
    expect(result).toEqual({
      error: "Invalid regular expression: [",
      code: "INVALID_REGEX",
    });
  });
  
  test("Should handle non-existent paths", async () => {
    const result = await grep({ 
      pattern: "test",
      path: join(testDir, "non-existent.txt")
    });
    expect(result).toHaveProperty("error");
    if ("error" in result) {
      expect(result.error).toContain("Path not found");
      expect(result.code).toBe("ENOENT");
    }
  });
  
  test("Should handle invalid input - null input", async () => {
    const result = await grep(null as any);
    expect(result).toEqual({
      error: "Invalid input: input must be an object",
    });
  });
  
  test("Should handle invalid input - missing pattern", async () => {
    const result = await grep({ path: testFile1 } as any);
    expect(result).toEqual({
      error: "Invalid input: pattern must be a non-empty string",
    });
  });
  
  test("Should handle invalid input - empty pattern", async () => {
    const result = await grep({ pattern: "", path: testFile1 });
    expect(result).toEqual({
      error: "Invalid input: pattern must be a non-empty string",
    });
  });
  
  test("Should handle invalid input - missing path", async () => {
    const result = await grep({ pattern: "test" } as any);
    expect(result).toEqual({
      error: "Invalid input: path must be a non-empty string",
    });
  });
  
  test("Should handle invalid input - empty path", async () => {
    const result = await grep({ pattern: "test", path: "" });
    expect(result).toEqual({
      error: "Invalid input: path must be a non-empty string",
    });
  });
  
  test("Should find matches with correct line and column numbers", async () => {
    const result = await grep({ 
      pattern: "matching",
      path: testFile1
    });
    expect(result).toHaveProperty("matches");
    if ("matches" in result) {
      expect(result.matches).toHaveLength(1);
      expect(result.matches[0]).toEqual({
        file: testFile1,
        line: 3,
        column: 9,
        text: "Pattern matching is fun!",
        lineNumber: 3,
      });
    }
  });
  
  test("Should trim whitespace from path", async () => {
    const result = await grep({ 
      pattern: "Compozy",
      path: `  ${testFile1}  `
    });
    expect(result).toHaveProperty("matches");
    if ("matches" in result) {
      expect(result.matches).toHaveLength(1);
    }
  });
});