import { chmod, open, rename, rm } from "node:fs/promises";
import { dirname, join } from "node:path";
import { randomUUID } from "node:crypto";

import { ensurePrivateDirectory } from "./private-directory";

export async function writeFileAtomic(
  path: string,
  data: string | Uint8Array,
  mode: number
): Promise<void> {
  const parent = dirname(path);
  await ensurePrivateDirectory(parent);
  const temporary = join(parent, `.${randomUUID()}.tmp`);
  const file = await open(temporary, "wx", mode);
  let failure: unknown;
  try {
    await file.writeFile(data);
    await file.sync();
  } catch (error) {
    failure = error;
  }
  try {
    await file.close();
  } catch (error) {
    failure = failure ? new AggregateError([failure, error]) : error;
  }
  if (failure) {
    try {
      await rm(temporary, { force: true });
    } catch (cleanupError) {
      throw new AggregateError([failure, cleanupError], "The atomic write and its cleanup failed.");
    }
    throw failure;
  }
  try {
    await chmod(temporary, mode);
    await rename(temporary, path);
  } catch (error) {
    try {
      await rm(temporary, { force: true });
    } catch (cleanupError) {
      throw new AggregateError([error, cleanupError], "Atomic finalization and cleanup failed.");
    }
    throw error;
  }
}
