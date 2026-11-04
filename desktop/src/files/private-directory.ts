import { chmod, mkdir } from "node:fs/promises";

import { protectWindowsDirectory } from "./windows-permissions";

export async function ensurePrivateDirectory(path: string): Promise<void> {
  await mkdir(path, { recursive: true, mode: 0o700 });
  if (process.platform === "win32") await protectWindowsDirectory(path);
  else await chmod(path, 0o700);
}
