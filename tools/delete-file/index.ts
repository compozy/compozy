import { unlink } from "fs/promises";

interface DeleteFileInput {
  path: string;
}

interface DeleteFileOutput {
  success: boolean;
}

export default async function deleteFile(input: DeleteFileInput): Promise<DeleteFileOutput> {
  // Validate input
  if (!input || typeof input.path !== "string") {
    throw new Error("Invalid input: path must be a string");
  }

  const filePath = input.path.trim();

  if (!filePath) {
    throw new Error("Invalid input: path cannot be empty");
  }

  // Security check: prevent directory traversal
  if (filePath.includes("..")) {
    throw new Error("Invalid path: parent directory references are not allowed");
  }

  // Additional security: prevent null bytes and control characters
  // eslint-disable-next-line no-control-regex
  if (filePath.includes("\0") || /[\x00-\x1f\x7f]/.test(filePath)) {
    throw new Error("Invalid path: path contains invalid characters");
  }

  try {
    await unlink(filePath);
    return { success: true };
  } catch (error: any) {
    // Handle expected errors gracefully
    if (error.code === "ENOENT") {
      // File doesn't exist
      return { success: false };
    }

    if (error.code === "EACCES" || error.code === "EPERM") {
      // Permission denied
      return { success: false };
    }

    // Re-throw unexpected errors
    throw error;
  }
}
