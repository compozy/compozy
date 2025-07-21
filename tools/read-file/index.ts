import { promises as fs } from "fs";
import { resolve } from "path";

interface ReadFileInput {
  path: string;
}

interface ReadFileOutput {
  content: string;
}

interface ReadFileError {
  error: string;
  code?: string;
}

/**
 * Read file content from the filesystem
 * @param input - Input containing the file path
 * @returns Object containing the file content
 * @throws Error if file cannot be read
 */
export async function readFile(input: ReadFileInput): Promise<ReadFileOutput | ReadFileError> {
  // Input validation
  if (!input || typeof input !== "object") {
    return {
      error: "Invalid input: input must be an object",
    };
  }
  if (typeof input.path !== "string" || input.path.trim() === "") {
    return {
      error: "Invalid input: path must be a non-empty string",
    };
  }
  const filePath = input.path.trim();
  const resolvedPath = resolve(filePath);
  try {
    // Read file content
    const content = await fs.readFile(resolvedPath, "utf-8");
    return {
      content,
    };
  } catch (error: any) {
    // Handle specific error types
    if (error.code === "ENOENT") {
      return {
        error: `File not found: ${filePath}`,
        code: "ENOENT",
      };
    } else if (error.code === "EACCES") {
      return {
        error: `Permission denied: ${filePath}`,
        code: "EACCES",
      };
    } else if (error.code === "EISDIR") {
      return {
        error: `Path is a directory, not a file: ${filePath}`,
        code: "EISDIR",
      };
    } else {
      return {
        error: `Failed to read file: ${error.message || "Unknown error"}`,
        code: error.code,
      };
    }
  }
}

// Default export for Compozy runtime compatibility
export default readFile;