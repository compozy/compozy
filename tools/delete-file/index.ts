import { unlink } from 'fs/promises';

interface DeleteFileInput {
  path: string;
}

interface DeleteFileOutput {
  success: boolean;
}

export default async function deleteFile(input: DeleteFileInput): Promise<DeleteFileOutput> {
  // Validate input
  if (!input || typeof input.path !== 'string') {
    throw new Error('Invalid input: path must be a string');
  }
  
  const filePath = input.path.trim();
  
  if (!filePath) {
    throw new Error('Invalid input: path cannot be empty');
  }
  
  try {
    await unlink(filePath);
    return { success: true };
  } catch (error: any) {
    // Handle expected errors gracefully
    if (error.code === 'ENOENT') {
      // File doesn't exist
      return { success: false };
    }
    
    if (error.code === 'EACCES' || error.code === 'EPERM') {
      // Permission denied
      return { success: false };
    }
    
    // Re-throw unexpected errors
    throw error;
  }
}