import { randomBytes, timingSafeEqual } from "node:crypto";
import { readFile, rm } from "node:fs/promises";
import { dirname } from "node:path";

import { writeFileAtomic } from "../files/atomic-write";
import { ensurePrivateDirectory } from "../files/private-directory";

export async function rotateControlToken(path: string): Promise<string> {
  // Refresh Windows permissions even when the same directory was secured previously.
  await ensurePrivateDirectory(dirname(path), true);
  const token = randomBytes(32).toString("base64url");
  await writeFileAtomic(path, `${token}\n`, 0o600);
  return token;
}

export async function readControlToken(path: string): Promise<string> {
  return (await readFile(path, "utf8")).trim();
}

export function tokenMatches(expected: string, received: unknown): boolean {
  if (typeof received !== "string") return false;
  const left = Buffer.from(expected);
  const right = Buffer.from(received);
  return left.length === right.length && timingSafeEqual(left, right);
}

export async function deleteControlToken(path: string): Promise<void> {
  await rm(path, { force: true });
}
