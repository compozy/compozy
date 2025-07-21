import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { mkdtempSync, rmSync, writeFileSync } from "fs";
import { tmpdir } from "os";
import { join } from "path";
import { exec } from "./index";

describe("exec", () => {
  let tempDir: string;

  beforeEach(() => {
    // Create a temporary directory for tests
    tempDir = mkdtempSync(join(tmpdir(), "exec-test-"));
  });

  afterEach(() => {
    // Clean up temporary directory
    rmSync(tempDir, { recursive: true, force: true });
  });

  describe("input validation", () => {
    test("Should reject invalid input types", async () => {
      // @ts-ignore - Testing invalid input
      const result1 = await exec(null);
      expect(result1).toEqual({
        error: "Invalid input: input must be an object",
      });

      // @ts-ignore - Testing invalid input
      const result2 = await exec("command");
      expect(result2).toEqual({
        error: "Invalid input: input must be an object",
      });
    });

    test("Should reject empty or invalid commands", async () => {
      const result1 = await exec({ command: "" });
      expect(result1).toEqual({
        error: "Command must be a non-empty string",
      });

      const result2 = await exec({ command: "   " });
      expect(result2).toEqual({
        error: "Command must be a non-empty string",
      });

      // @ts-ignore - Testing invalid command type
      const result3 = await exec({ command: 123 });
      expect(result3).toEqual({
        error: "Command must be a non-empty string",
      });
    });

    test("Should reject invalid optional parameters", async () => {
      // @ts-ignore - Testing invalid cwd type
      const result1 = await exec({ command: "echo test", cwd: 123 });
      expect(result1).toEqual({
        error: "Invalid input: cwd must be a string",
      });

      // @ts-ignore - Testing invalid env type
      const result2 = await exec({ command: "echo test", env: "invalid" });
      expect(result2).toEqual({
        error: "Invalid input: env must be an object",
      });

      const result3 = await exec({ command: "echo test", timeout: -1 });
      expect(result3).toEqual({
        error: "Invalid input: timeout must be a positive number",
      });

      const result4 = await exec({ command: "echo test", timeout: 400000 });
      expect(result4).toEqual({
        error: "Invalid input: timeout cannot exceed 5 minutes (300000ms)",
      });
    });
  });

  describe("security validation", () => {
    test("Should reject commands with dangerous patterns", async () => {
      const dangerousCommands = [
        "ls; rm -rf /",
        "echo test && malicious",
        "echo test | grep something",
        "echo test > file.txt",
        "echo test < input.txt",
        "echo $(whoami)",
        "echo `date`",
        "echo test \\\nmalicious",
        "echo ${PATH}",
      ];

      for (const command of dangerousCommands) {
        const result = await exec({ command });
        expect(result).toHaveProperty("error");
        expect((result as any).error).toContain("dangerous pattern");
      }
    });

    test("Should reject non-whitelisted commands", async () => {
      const disallowedCommands = [
        "rm -rf /",
        "curl http://example.com",
        "wget http://example.com",
        "nc -l 8080",
        "python script.py",
        "node script.js",
        "sh script.sh",
        "bash script.sh",
      ];

      for (const command of disallowedCommands) {
        const result = await exec({ command });
        expect(result).toHaveProperty("error");
        expect((result as any).error).toContain("not in the allowed list");
      }
    });
  });

  describe("successful execution", () => {
    test("Should execute simple commands", async () => {
      const result = await exec({ command: "echo 'Hello, World!'" });
      expect(result).toMatchObject({
        stdout: "Hello, World!\n",
        stderr: "",
        exitCode: 0,
        success: true,
      });
    });

    test("Should execute commands with arguments", async () => {
      const result = await exec({ command: "echo -n 'No newline'" });
      expect(result).toMatchObject({
        stdout: "No newline",
        stderr: "",
        exitCode: 0,
        success: true,
      });
    });

    test("Should capture multiline output", async () => {
      // Since && is blocked, we'll test with a command that naturally produces multiline output
      const result = await exec({ command: "echo -e 'Line 1\\nLine 2'" });
      expect(result).toMatchObject({
        stdout: "Line 1\nLine 2\n",
        stderr: "",
        exitCode: 0,
        success: true,
      });
    });
  });

  describe("command failures", () => {
    test("Should handle non-zero exit codes", async () => {
      const result = await exec({ command: "ls /nonexistent/path" });
      expect(result).toMatchObject({
        exitCode: expect.any(Number),
        success: false,
      });
      expect((result as any).exitCode).not.toBe(0);
      expect((result as any).stderr).toContain("No such file or directory");
    });

    test("Should capture stderr output", async () => {
      // Create a test file
      const testFile = join(tempDir, "test.txt");
      writeFileSync(testFile, "test content");

      // Try to list a file as directory
      const result = await exec({ command: `ls ${testFile}/` });
      expect(result).toMatchObject({
        success: false,
      });
      expect((result as any).stderr).toBeTruthy();
    });
  });

  describe("timeout handling", () => {
    test("Should timeout long-running commands", async () => {
      const result = await exec({
        command: "sleep 5",
        timeout: 100, // 100ms timeout
      });
      expect(result).toHaveProperty("error");
      expect((result as any).error).toContain("timed out after 100ms");
    });

    test("Should respect custom timeout values", async () => {
      const start = Date.now();
      const result = await exec({
        command: "sleep 0.5",
        timeout: 1000, // 1 second timeout
      });
      const duration = Date.now() - start;

      // Should complete successfully within timeout
      expect(result).toMatchObject({
        success: true,
        exitCode: 0,
      });
      expect(duration).toBeLessThan(1000);
    });
  });

  describe("working directory", () => {
    test("Should execute commands in specified directory", async () => {
      const result = await exec({
        command: "pwd",
        cwd: tempDir,
      });
      expect(result).toMatchObject({
        stderr: "",
        exitCode: 0,
        success: true,
      });
      // On macOS, /var might be a symlink to /private/var
      const stdout = (result as any).stdout.trim();
      expect(stdout).toMatch(new RegExp(`${tempDir}$`));
    });

    test("Should handle invalid working directory", async () => {
      const result = await exec({
        command: "pwd",
        cwd: "/nonexistent/directory",
      });
      expect(result).toHaveProperty("error");
      expect((result as any).error).toContain("Failed to execute command");
    });
  });

  describe("environment variables", () => {
    test("Should pass custom environment variables", async () => {
      const result = await exec({
        command: "echo $CUSTOM_VAR",
        env: { CUSTOM_VAR: "test-value" },
      });
      expect(result).toMatchObject({
        stdout: "test-value\n",
        stderr: "",
        exitCode: 0,
        success: true,
      });
    });

    test("Should merge with existing environment", async () => {
      // PATH should still be available
      const result = await exec({
        command: "echo $PATH",
        env: { CUSTOM_VAR: "test" },
      });
      expect((result as any).stdout).toBeTruthy();
      expect((result as any).success).toBe(true);
    });
  });

  describe("complex commands", () => {
    test("Should handle commands with quotes", async () => {
      const result = await exec({
        command: `echo "Hello 'nested' quotes"`,
      });
      expect(result).toMatchObject({
        stdout: "Hello 'nested' quotes\n",
        stderr: "",
        exitCode: 0,
        success: true,
      });
    });

    test("Should handle file operations in allowed commands", async () => {
      // Create a test file
      const testFile = join(tempDir, "test.txt");
      writeFileSync(testFile, "Line 1\nLine 2\nLine 3\n");

      // Test various file operations
      const result1 = await exec({
        command: `cat ${testFile}`,
        cwd: tempDir,
      });
      expect(result1).toMatchObject({
        stdout: "Line 1\nLine 2\nLine 3\n",
        success: true,
      });

      const result2 = await exec({
        command: `wc -l ${testFile}`,
        cwd: tempDir,
      });
      expect((result2 as any).stdout).toContain("3");
      expect((result2 as any).success).toBe(true);

      const result3 = await exec({
        command: `head -n 1 ${testFile}`,
        cwd: tempDir,
      });
      expect(result3).toMatchObject({
        stdout: "Line 1\n",
        success: true,
      });
    });
  });
});
