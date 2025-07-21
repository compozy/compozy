import { promises as fs } from "fs";
import { join, resolve } from "path";

interface GrepInput {
  pattern: string;
  path: string;
  recursive?: boolean;
  ignoreCase?: boolean;
  maxResults?: number;
}

interface GrepMatch {
  file: string;
  line: number;
  column: number;
  text: string;
}

interface GrepOutput {
  matches: GrepMatch[];
}

interface GrepError {
  error: string;
  code?: string;
}

/**
 * Search for content in files using pattern matching (supports regex)
 * @param input - Input containing the pattern, path, and options
 * @returns Object containing array of matches or error
 */
export async function grep(input: GrepInput): Promise<GrepOutput | GrepError> {
  // Input validation
  if (!input || typeof input !== "object") {
    return {
      error: "Invalid input: input must be an object",
    };
  }
  if (typeof input.pattern !== "string" || input.pattern === "") {
    return {
      error: "Invalid input: pattern must be a non-empty string",
    };
  }
  if (typeof input.path !== "string" || input.path.trim() === "") {
    return {
      error: "Invalid input: path must be a non-empty string",
    };
  }
  const searchPath = input.path.trim();
  const pattern = input.pattern;
  const recursive = input.recursive ?? false;
  const ignoreCase = input.ignoreCase ?? false;
  const maxResults = input.maxResults;
  const resolvedPath = resolve(searchPath);
  const matches: GrepMatch[] = [];
  try {
    // Create regex from pattern
    let regex: RegExp;
    try {
      regex = new RegExp(pattern, ignoreCase ? "gi" : "g");
    } catch {
      return {
        error: `Invalid regular expression: ${pattern}`,
        code: "INVALID_REGEX",
      };
    }
    // Check if path exists
    const stats = await fs.stat(resolvedPath);
    if (stats.isDirectory()) {
      // Search in directory
      await searchDirectory(resolvedPath, regex, matches, recursive, maxResults);
    } else if (stats.isFile()) {
      // Search in single file
      await searchFile(resolvedPath, regex, matches, maxResults);
    } else {
      return {
        error: `Path is neither a file nor a directory: ${searchPath}`,
        code: "INVALID_PATH",
      };
    }
    return {
      matches,
    };
  } catch (error: any) {
    // Handle specific error types
    if (error.code === "ENOENT") {
      return {
        error: `Path not found: ${searchPath}`,
        code: "ENOENT",
      };
    } else if (error.code === "EACCES") {
      return {
        error: `Permission denied: ${searchPath}`,
        code: "EACCES",
      };
    } else {
      return {
        error: `Failed to search: ${error.message || "Unknown error"}`,
        code: error.code,
      };
    }
  }
}

async function searchDirectory(
  dir: string,
  regex: RegExp,
  matches: GrepMatch[],
  recursive: boolean,
  maxResults?: number
): Promise<void> {
  try {
    const entries = await fs.readdir(dir, { withFileTypes: true });
    for (const entry of entries) {
      if (maxResults && matches.length >= maxResults) {
        return;
      }
      const fullPath = join(dir, entry.name);
      if (entry.isDirectory() && recursive) {
        await searchDirectory(fullPath, regex, matches, recursive, maxResults);
      } else if (entry.isFile()) {
        await searchFile(fullPath, regex, matches, maxResults);
      }
    }
  } catch {
    // Silently skip directories we can't read
  }
}

async function searchFile(
  filePath: string,
  regex: RegExp,
  matches: GrepMatch[],
  maxResults?: number
): Promise<void> {
  try {
    // Read file content
    const content = await fs.readFile(filePath, "utf-8").catch(() => null);
    if (content === null) {
      // Skip binary or unreadable files
      return;
    }
    const lines = content.split("\n");
    for (let lineIndex = 0; lineIndex < lines.length; lineIndex++) {
      if (maxResults && matches.length >= maxResults) {
        return;
      }
      const line = lines[lineIndex];
      // Reset regex lastIndex for each line
      regex.lastIndex = 0;
      let match;
      while ((match = regex.exec(line)) !== null) {
        if (maxResults && matches.length >= maxResults) {
          return;
        }
        matches.push({
          file: filePath,
          line: lineIndex + 1,
          column: match.index + 1,
          text: line.trim(),
        });
        // Prevent infinite loop on zero-width matches
        if (match.index === regex.lastIndex) {
          regex.lastIndex++;
        }
      }
    }
  } catch {
    // Silently skip files we can't read (e.g., binary files)
  }
}

// Default export for Compozy runtime compatibility
export default grep;
