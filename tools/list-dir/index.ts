import { readdir, stat } from 'fs/promises';
import { join, resolve } from 'path';
import { minimatch } from 'minimatch';

export interface ToolInput {
  path: string;
  pattern?: string;
  recursive?: boolean;
  includeFiles?: boolean;
  includeDirs?: boolean;
}

export interface DirEntry {
  name: string;
  path: string;
  type: 'file' | 'dir';
  size: number;
  modified: string;
}

export interface ToolOutput {
  entries: DirEntry[];
  error?: {
    message: string;
    code?: string;
  };
}

async function listDirRecursive(
  dirPath: string,
  basePath: string,
  pattern?: string,
  includeFiles = true,
  includeDirs = true
): Promise<DirEntry[]> {
  const entries: DirEntry[] = [];

  try {
    const items = await readdir(dirPath);

    for (const item of items) {
      const fullPath = join(dirPath, item);
      const relativePath = fullPath.slice(basePath.length + 1);

      try {
        const stats = await stat(fullPath);
        const isDirectory = stats.isDirectory();
        const type: 'file' | 'dir' = isDirectory ? 'dir' : 'file';

        // Check if we should include this type
        if ((type === 'file' && !includeFiles) || (type === 'dir' && !includeDirs)) {
          // Still recurse into directories even if we don't include them
          if (isDirectory) {
            const subEntries = await listDirRecursive(fullPath, basePath, pattern, includeFiles, includeDirs);
            entries.push(...subEntries);
          }
          continue;
        }

        // Check pattern match
        if (pattern && !minimatch(relativePath, pattern)) {
          // Still recurse into directories for globstar patterns
          if (isDirectory && pattern.includes('**')) {
            const subEntries = await listDirRecursive(fullPath, basePath, pattern, includeFiles, includeDirs);
            entries.push(...subEntries);
          }
          continue;
        }

        entries.push({
          name: item,
          path: fullPath,
          type,
          size: isDirectory ? 0 : stats.size,
          modified: stats.mtime.toISOString()
        });

        // Recurse into directories
        if (isDirectory) {
          const subEntries = await listDirRecursive(fullPath, basePath, pattern, includeFiles, includeDirs);
          entries.push(...subEntries);
        }
      } catch (error) {
        // Skip entries we can't stat (e.g., broken symlinks)
      }
    }
  } catch (error) {
    // Skip directories we can't read
  }

  return entries;
}

export default async function tool(input: ToolInput): Promise<ToolOutput> {
  const {
    path,
    pattern,
    recursive = false,
    includeFiles = true,
    includeDirs = true
  } = input;

  // Validate input
  if (!path || typeof path !== 'string') {
    return { 
      entries: [],
      error: {
        message: "Invalid input: path must be a non-empty string",
        code: "INVALID_PATH"
      }
    };
  }

  const absolutePath = resolve(path);
  const entries: DirEntry[] = [];

  try {
    // Check if the path exists and is a directory
    const pathStats = await stat(absolutePath);
    if (!pathStats.isDirectory()) {
      return { 
        entries: [],
        error: {
          message: `Path is not a directory: ${absolutePath}`,
          code: "NOT_DIRECTORY"
        }
      };
    }

    if (recursive) {
      // Recursive listing
      const recursiveEntries = await listDirRecursive(absolutePath, absolutePath, pattern, includeFiles, includeDirs);
      entries.push(...recursiveEntries);
    } else {
      // Non-recursive listing
      const items = await readdir(absolutePath);

      for (const item of items) {
        const fullPath = join(absolutePath, item);

        try {
          const stats = await stat(fullPath);
          const isDirectory = stats.isDirectory();
          const type: 'file' | 'dir' = isDirectory ? 'dir' : 'file';

          // Check if we should include this type
          if ((type === 'file' && !includeFiles) || (type === 'dir' && !includeDirs)) {
            continue;
          }

          // Check pattern match (for non-recursive, match against item name)
          if (pattern && !minimatch(item, pattern)) {
            continue;
          }

          entries.push({
            name: item,
            path: fullPath,
            type,
            size: isDirectory ? 0 : stats.size,
            modified: stats.mtime.toISOString()
          });
        } catch (error) {
          // Skip entries we can't stat
        }
      }
    }

    // Sort entries by path for consistent output
    entries.sort((a, b) => a.path.localeCompare(b.path));
  } catch (error: any) {
    // Handle specific error types
    if (error.code === 'ENOENT') {
      return {
        entries: [],
        error: {
          message: `Path does not exist: ${absolutePath}`,
          code: "PATH_NOT_FOUND"
        }
      };
    } else if (error.code === 'EACCES') {
      return {
        entries: [],
        error: {
          message: `Permission denied: ${absolutePath}`,
          code: "ACCESS_DENIED"
        }
      };
    } else {
      return {
        entries: [],
        error: {
          message: `Failed to list directory: ${error.message || 'Unknown error'}`,
          code: error.code || "LIST_ERROR"
        }
      };
    }
  }

  return { entries };
}