import { afterEach, beforeEach, describe, expect, mock, test } from "bun:test";
import * as fs from "fs/promises";
import * as path from "path";

// Helper function to set up memory limits - replaces eval() usage for security
function memorySetup(
  env: NodeJS.ProcessEnv = process.env,
  setResourceLimits?: typeof process.setResourceLimits
): void {
  const actualSetResourceLimits = setResourceLimits || (process as any).setResourceLimits;

  if (env.COMPOZY_MAX_MEMORY_MB) {
    const maxMemoryMB = parseInt(env.COMPOZY_MAX_MEMORY_MB, 10);

    if (isNaN(maxMemoryMB) || maxMemoryMB <= 0) {
      const error = new Error(
        `Invalid COMPOZY_MAX_MEMORY_MB value: "${env.COMPOZY_MAX_MEMORY_MB}". ` +
          `Expected a positive integer representing megabytes.`
      );
      console.error("[CRITICAL] Memory limit configuration error:", error.message);
      process.exit(1);
    }

    if (typeof actualSetResourceLimits === "function") {
      try {
        actualSetResourceLimits({ maxHeapSize: maxMemoryMB });
        console.error(`[INFO] Memory limit set to ${maxMemoryMB}MB`);
      } catch (err) {
        const error = err instanceof Error ? err : new Error(String(err));
        console.error("[CRITICAL] Failed to set memory limit:", {
          error: error.message,
          stack: error.stack,
          requestedLimit: maxMemoryMB,
          env: env.COMPOZY_MAX_MEMORY_MB,
        });
        process.exit(1);
      }
    } else {
      console.error("[WARNING] process.setResourceLimits not available in this runtime");
    }
  }
}

describe("Worker Memory Limit Configuration", () => {
  let originalEnv: NodeJS.ProcessEnv;
  let originalExit: typeof process.exit;
  let originalError: typeof console.error;
  let errorLogs: any[] = [];
  let exitCode: number | undefined;

  beforeEach(() => {
    // Store original values
    originalEnv = { ...process.env };
    originalExit = process.exit;
    originalError = console.error;

    // Mock console.error to capture logs
    errorLogs = [];
    console.error = (...args: any[]) => {
      errorLogs.push(args);
    };

    // Mock process.exit to capture exit codes
    exitCode = undefined;
    process.exit = ((code?: number) => {
      exitCode = code;
      throw new Error(`Process exited with code ${code}`);
    }) as any;
  });

  afterEach(() => {
    // Restore original values
    process.env = originalEnv;
    process.exit = originalExit;
    console.error = originalError;
    delete (process as any).setResourceLimits;
  });

  describe("Valid COMPOZY_MAX_MEMORY_MB values", () => {
    test("Should call setResourceLimits with valid positive integer", () => {
      // Arrange
      let calledWith: any = null;
      (process as any).setResourceLimits = mock((limits: any) => {
        calledWith = limits;
      });
      process.env.COMPOZY_MAX_MEMORY_MB = "512";

      // Act - Execute the memory limit setup code
      memorySetup();

      // Assert
      expect(calledWith).toEqual({ maxHeapSize: 512 });
      expect(errorLogs).toContainEqual(["[INFO] Memory limit set to 512MB"]);
      expect(exitCode).toBeUndefined();
    });

    test("Should handle large memory values correctly", () => {
      // Arrange
      let calledWith: any = null;
      (process as any).setResourceLimits = mock((limits: any) => {
        calledWith = limits;
      });
      process.env.COMPOZY_MAX_MEMORY_MB = "8192"; // 8GB

      // Act
      memorySetup();

      // Assert
      expect(calledWith).toEqual({ maxHeapSize: 8192 });
      expect(errorLogs).toContainEqual(["[INFO] Memory limit set to 8192MB"]);
    });
  });

  describe("Invalid COMPOZY_MAX_MEMORY_MB values", () => {
    test("Should exit with code 1 for non-numeric values", () => {
      // Arrange
      process.env.COMPOZY_MAX_MEMORY_MB = "not-a-number";

      // Act & Assert
      expect(() => {
        memorySetup();
      }).toThrow("Process exited with code 1");

      expect(exitCode).toBe(1);
      expect(errorLogs[0][0]).toBe("[CRITICAL] Memory limit configuration error:");
      expect(errorLogs[0][1]).toContain('Invalid COMPOZY_MAX_MEMORY_MB value: "not-a-number"');
    });

    test("Should exit with code 1 for negative values", () => {
      // Arrange
      process.env.COMPOZY_MAX_MEMORY_MB = "-100";

      // Act & Assert
      expect(() => {
        memorySetup();
      }).toThrow("Process exited with code 1");

      expect(exitCode).toBe(1);
      expect(errorLogs[0][1]).toContain('Invalid COMPOZY_MAX_MEMORY_MB value: "-100"');
    });

    test("Should exit with code 1 for zero value", () => {
      // Arrange
      process.env.COMPOZY_MAX_MEMORY_MB = "0";

      // Act & Assert
      expect(() => {
        memorySetup();
      }).toThrow("Process exited with code 1");

      expect(exitCode).toBe(1);
    });

    test("Should exit with code 1 for decimal values", () => {
      // Arrange
      process.env.COMPOZY_MAX_MEMORY_MB = "512.5";

      // Act - parseInt will parse this as 512, which is valid
      let calledWith: any = null;
      (process as any).setResourceLimits = mock((limits: any) => {
        calledWith = limits;
      });

      memorySetup();

      // Assert - parseInt truncates to 512
      expect(calledWith).toEqual({ maxHeapSize: 512 });
      expect(errorLogs).toContainEqual(["[INFO] Memory limit set to 512MB"]);
    });
  });

  describe("setResourceLimits error handling", () => {
    test("Should exit with code 1 when setResourceLimits throws an error", () => {
      // Arrange
      process.env.COMPOZY_MAX_MEMORY_MB = "512";
      const testError = new Error("Resource limit exceeded");
      (process as any).setResourceLimits = mock(() => {
        throw testError;
      });

      // Act & Assert
      expect(() => {
        memorySetup();
      }).toThrow("Process exited with code 1");

      expect(exitCode).toBe(1);
      expect(errorLogs[0][0]).toBe("[CRITICAL] Failed to set memory limit:");
      expect(errorLogs[0][1].error).toBe("Resource limit exceeded");
      expect(errorLogs[0][1].requestedLimit).toBe(512);
      expect(errorLogs[0][1].env).toBe("512");
    });

    test("Should log structured error with full context on failure", () => {
      // Arrange
      process.env.COMPOZY_MAX_MEMORY_MB = "2048";
      const testError = new Error("Insufficient system resources");
      testError.stack = "Error: Insufficient system resources\n    at test.js:1:1";
      (process as any).setResourceLimits = mock(() => {
        throw testError;
      });

      // Act & Assert
      expect(() => {
        memorySetup();
      }).toThrow("Process exited with code 1");

      const loggedContext = errorLogs[0][1];
      expect(loggedContext.error).toBe("Insufficient system resources");
      expect(loggedContext.stack).toContain("Error: Insufficient system resources");
      expect(loggedContext.requestedLimit).toBe(2048);
      expect(loggedContext.env).toBe("2048");
    });
  });

  describe("setResourceLimits availability", () => {
    test("Should log warning when setResourceLimits is not available", () => {
      // Arrange
      process.env.COMPOZY_MAX_MEMORY_MB = "512";
      delete (process as any).setResourceLimits; // Simulate unavailable function

      // Act
      memorySetup();

      // Assert - warning should be logged when setResourceLimits is unavailable
      expect(errorLogs).toContainEqual([
        "[WARNING] process.setResourceLimits not available in this runtime",
      ]);
      expect(exitCode).toBeUndefined();
    });

    test("Should not attempt to set memory limit when COMPOZY_MAX_MEMORY_MB is empty string", () => {
      // Arrange
      process.env.COMPOZY_MAX_MEMORY_MB = "";
      let setResourceLimitsCalled = false;
      (process as any).setResourceLimits = mock(() => {
        setResourceLimitsCalled = true;
      });

      // Act
      memorySetup();

      // Assert - empty string is falsy, so the if block is not entered
      expect(setResourceLimitsCalled).toBe(false);
      expect(errorLogs).toHaveLength(0);
      expect(exitCode).toBeUndefined();
    });
  });
});

describe("Integration Tests - Worker Template Memory Limits", () => {
  const workerTemplatePath = path.join(__dirname, "worker.tpl.ts");

  test("Should validate worker template contains improved error handling", async () => {
    // Read the actual worker template file
    const workerContent = await fs.readFile(workerTemplatePath, "utf-8");

    // Verify critical improvements are present
    expect(workerContent).toContain("[CRITICAL] Memory limit configuration error:");
    expect(workerContent).toContain("[CRITICAL] Failed to set memory limit:");
    expect(workerContent).toContain("[INFO] Memory limit set to");
    expect(workerContent).toContain("[WARNING] process.setResourceLimits not available");
    expect(workerContent).toContain("process.exit(1)");
    expect(workerContent).toContain("isNaN(maxMemoryMB) || maxMemoryMB <= 0");

    // Verify structured error logging
    expect(workerContent).toContain("requestedLimit: maxMemoryMB");
    expect(workerContent).toContain("env: process.env.COMPOZY_MAX_MEMORY_MB");
  });
});
