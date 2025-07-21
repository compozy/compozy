import { readdir } from "node:fs/promises";

/**
 * Input parameters for the list-files tool
 */
interface ListFilesInput {
  /** The directory path to list files from */
  dir: string;
}

/**
 * Output structure for the list-files tool
 */
interface ListFilesOutput {
  /** Array of file names found in the directory */
  files: string[];
  /** Error message if the operation failed */
  error?: string;
}

/**
 * Lists all files in a directory (non-recursive)
 *
 * @param input - The input parameters containing the directory path
 * @returns An object containing an array of file names, or error information on failure
 *
 * @example
 * ```typescript
 * const result = await listFiles({ dir: './src' });
 * console.log(result.files); // ['index.ts', 'utils.ts', ...]
 * ```
 */
export async function listFiles(input: ListFilesInput): Promise<ListFilesOutput> {
  try {
    // Validate input
    if (!input || typeof input !== "object") {
      return {
        files: [],
        error: "Invalid input: input must be an object",
      };
    }

    if (typeof input.dir !== "string" || input.dir.trim() === "") {
      return {
        files: [],
        error: "Invalid input: dir must be a non-empty string",
      };
    }

    // Read directory entries with file type information
    const entries = await readdir(input.dir, { withFileTypes: true });

    // Filter for files only (excluding directories) and sort alphabetically
    const files = entries
      .filter(entry => entry.isFile() || entry.isSymbolicLink())
      .map(entry => entry.name)
      .sort();

    return { files };
  } catch (error: any) {
    // Handle specific error types for better error reporting
    if (error.code === "ENOENT") {
      return {
        files: [],
        error: `Directory not found: ${input.dir}`,
      };
    } else if (error.code === "EACCES") {
      return {
        files: [],
        error: `Permission denied: ${input.dir}`,
      };
    } else if (error.code === "ENOTDIR") {
      return {
        files: [],
        error: `Path is not a directory: ${input.dir}`,
      };
    } else {
      return {
        files: [],
        error: `Failed to read directory: ${error.message || "Unknown error"}`,
      };
    }
  }
}

// Default export for Compozy runtime compatibility
export default listFiles;
