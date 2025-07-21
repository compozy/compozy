import { readdir } from 'node:fs/promises';
import { join } from 'node:path';

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
}

/**
 * Lists all files in a directory (non-recursive)
 * 
 * @param input - The input parameters containing the directory path
 * @returns An object containing an array of file names, or empty array on error
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
    if (!input || typeof input.dir !== 'string') {
      return { files: [] };
    }

    // Read directory entries with file type information
    const entries = await readdir(input.dir, { withFileTypes: true });
    
    // Filter for files only (excluding directories) and sort alphabetically
    const files = entries
      .filter(entry => entry.isFile() || entry.isSymbolicLink())
      .map(entry => entry.name)
      .sort();
    
    return { files };
  } catch (error) {
    // Return empty array on any error (non-existent directory, permission issues, etc.)
    return { files: [] };
  }
}

// Default export for Compozy runtime compatibility
export default listFiles;