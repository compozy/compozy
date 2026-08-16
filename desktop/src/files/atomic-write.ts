import { chmod, mkdir, open, rename, rm } from "node:fs/promises";
import { dirname, join } from "node:path";
import { randomUUID } from "node:crypto";

export async function writeFileAtomic(
  path: string,
  data: string | Uint8Array,
  mode: number
): Promise<void> {
  const parent = dirname(path);
  await mkdir(parent, { recursive: true, mode: 0o700 });
  await chmod(parent, 0o700);
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
  await chmod(temporary, mode);
  await rename(temporary, path);
}
