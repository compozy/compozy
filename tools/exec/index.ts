import { spawn } from "child_process";

interface ExecInput {
  command: string;
  cwd?: string;
  env?: Record<string, string>;
  timeout?: number;
}

interface ExecOutput {
  stdout: string;
  stderr: string;
  exitCode: number;
  success: boolean;
}

interface ExecError {
  error: string;
  stdout?: string;
  stderr?: string;
  exitCode?: number;
}

// Security: Command validation patterns
const DANGEROUS_PATTERNS = [
  /[;&|<>]/, // Command chaining, pipes, redirects
  /\$\(/, // Command substitution
  /`/, // Backticks
  /\\\n/, // Line continuation
  /\$\{.*\}/, // Variable expansion
];

const ALLOWED_COMMANDS = new Set([
  "ls",
  "pwd",
  "echo",
  "cat",
  "grep",
  "find",
  "wc",
  "sort",
  "uniq",
  "head",
  "tail",
  "cut",
  "awk",
  "sed",
  "date",
  "whoami",
  "hostname",
  "df",
  "du",
  "ps",
  "top",
  "free",
  "uptime",
  "which",
  "whereis",
  "file",
  "stat",
  "basename",
  "dirname",
  "realpath",
  "readlink",
  "sleep", // Added for testing timeout functionality
]);

/**
 * Validate command for security risks
 * @param command - Command to validate
 * @returns Error message if invalid, null if valid
 */
function validateCommand(command: string): string | null {
  if (!command || typeof command !== "string") {
    return "Command must be a non-empty string";
  }
  const trimmedCommand = command.trim();
  if (trimmedCommand.length === 0) {
    return "Command must be a non-empty string";
  }
  // Check for dangerous patterns
  for (const pattern of DANGEROUS_PATTERNS) {
    if (pattern.test(trimmedCommand)) {
      return `Command contains dangerous pattern: ${pattern}`;
    }
  }
  // Extract base command (first word)
  const baseCommand = trimmedCommand.split(/\s+/)[0];
  // Check if command is in allowed list
  if (!ALLOWED_COMMANDS.has(baseCommand)) {
    return `Command '${baseCommand}' is not in the allowed list`;
  }
  return null;
}

/**
 * Execute a bash command with security validation
 * @param input - Input containing command and optional parameters
 * @returns Object containing execution results
 */
export async function exec(input: ExecInput): Promise<ExecOutput | ExecError> {
  // Input validation
  if (!input || typeof input !== "object") {
    return {
      error: "Invalid input: input must be an object",
    };
  }
  // Validate command
  const validationError = validateCommand(input.command);
  if (validationError) {
    return {
      error: validationError,
    };
  }
  // Validate optional parameters
  if (input.cwd !== undefined && typeof input.cwd !== "string") {
    return {
      error: "Invalid input: cwd must be a string",
    };
  }
  if (input.env !== undefined && (typeof input.env !== "object" || input.env === null)) {
    return {
      error: "Invalid input: env must be an object",
    };
  }
  if (input.timeout !== undefined) {
    if (typeof input.timeout !== "number" || input.timeout <= 0) {
      return {
        error: "Invalid input: timeout must be a positive number",
      };
    }
    if (input.timeout > 300000) {
      // 5 minutes max
      return {
        error: "Invalid input: timeout cannot exceed 5 minutes (300000ms)",
      };
    }
  }
  const command = input.command.trim();
  const cwd = input.cwd || process.cwd();
  const timeout = input.timeout || 30000; // Default 30 seconds

  // Prepare environment
  const env = input.env ? { ...process.env, ...input.env } : process.env;

  return new Promise<ExecOutput | ExecError>(resolve => {
    let stdout = "";
    let stderr = "";
    let timedOut = false;

    // Spawn the process
    const proc = spawn("bash", ["-c", command], {
      cwd,
      env,
      shell: false, // Don't use shell to spawn bash itself
    });

    // Set up timeout
    const timer = setTimeout(() => {
      timedOut = true;
      proc.kill("SIGTERM");
      // Force kill after 5 seconds if not terminated
      setTimeout(() => {
        if (!proc.killed) {
          proc.kill("SIGKILL");
        }
      }, 5000);
    }, timeout);

    // Collect stdout
    proc.stdout.on("data", data => {
      stdout += data.toString();
    });

    // Collect stderr
    proc.stderr.on("data", data => {
      stderr += data.toString();
    });

    // Handle process exit
    proc.on("exit", (code, signal) => {
      clearTimeout(timer);

      if (timedOut) {
        resolve({
          error: `Command timed out after ${timeout}ms`,
          stdout,
          stderr,
          exitCode: code ?? -1,
        });
        return;
      }

      if (signal) {
        resolve({
          error: `Command terminated by signal: ${signal}`,
          stdout,
          stderr,
          exitCode: code ?? -1,
        });
        return;
      }

      const exitCode = code ?? 0;
      resolve({
        stdout,
        stderr,
        exitCode,
        success: exitCode === 0,
      });
    });

    // Handle process errors
    proc.on("error", error => {
      clearTimeout(timer);
      resolve({
        error: `Failed to execute command: ${error.message}`,
        stdout,
        stderr,
      });
    });
  });
}

// Default export for Compozy runtime compatibility
export default exec;
