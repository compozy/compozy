import { promises as fs } from "node:fs";
import { dirname } from "node:path";

interface WriteFileInput {
  path: string;
  content: string;
  append?: boolean;
}

interface WriteFileOutput {
  success: boolean;
  error?: string;
}

interface WriteFileConfig {
  encoding?: BufferEncoding;
  mode?: number;
}

/**
 * Write content to a file
 *
 * @param input - The input parameters
 * @param input.path - The file path to write to
 * @param input.content - The content to write
 * @param input.append - Whether to append to existing file (default: false)
 * @param config - Optional configuration
 * @param config.encoding - File encoding (default: 'utf-8')
 * @param config.mode - File permissions mode (default: 0o644)
 * @returns Object with success status
 */
export async function writeFile(
  input: WriteFileInput,
  config?: WriteFileConfig
): Promise<WriteFileOutput> {
  // Input validation
  if (!input || typeof input !== "object") {
    return {
      success: false,
      error: "Invalid input: expected an object",
    };
  }

  if (!input.path || typeof input.path !== "string") {
    return {
      success: false,
      error: "Invalid input: path must be a non-empty string",
    };
  }

  if (typeof input.content !== "string") {
    return {
      success: false,
      error: "Invalid input: content must be a string",
    };
  }

  const filePath = input.path.trim();
  if (filePath === "") {
    return {
      success: false,
      error: "Invalid input: path cannot be empty",
    };
  }

  const encoding = config?.encoding || "utf-8";
  const mode = config?.mode || 0o644;

  try {
    // Create parent directories if they don't exist
    const dir = dirname(filePath);
    if (dir && dir !== ".") {
      await fs.mkdir(dir, { recursive: true });
    }

    // Write or append to file
    if (input.append) {
      await fs.appendFile(filePath, input.content, { encoding, mode });
    } else {
      await fs.writeFile(filePath, input.content, { encoding, mode });
    }

    return {
      success: true,
    };
  } catch (error) {
    // Handle specific error cases
    if (error instanceof Error) {
      if (error.message.includes("EACCES") || error.message.includes("EPERM")) {
        return {
          success: false,
          error: `Permission denied: cannot write to ${filePath}`,
        };
      }

      if (error.message.includes("ENOENT")) {
        return {
          success: false,
          error: `Path not found: ${filePath}`,
        };
      }

      if (error.message.includes("EISDIR")) {
        return {
          success: false,
          error: `Target is a directory: ${filePath}`,
        };
      }

      if (error.message.includes("ENOSPC")) {
        return {
          success: false,
          error: "No space left on device",
        };
      }

      return {
        success: false,
        error: `Write failed: ${error.message}`,
      };
    }

    return {
      success: false,
      error: "Unknown error occurred while writing file",
    };
  }
}

// Export as default for Compozy runtime
export default writeFile;
