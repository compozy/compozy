import { lstat, readFile, rm } from "node:fs/promises";

import { writeFileAtomic } from "../files/atomic-write";

const MAXIMUM_MARKER_BYTES = 4 * 1024;

export interface StartupMarker {
  readonly boot_id: string;
  readonly boot_phase: string;
  readonly started_at: string;
}

function validMarker(value: unknown): value is StartupMarker {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const marker = value as Record<string, unknown>;
  return (
    typeof marker.boot_id === "string" &&
    marker.boot_id.trim() !== "" &&
    typeof marker.boot_phase === "string" &&
    marker.boot_phase.trim() !== "" &&
    typeof marker.started_at === "string" &&
    Number.isFinite(Date.parse(marker.started_at))
  );
}

export async function readStartupMarker(path: string): Promise<StartupMarker | null> {
  let metadata;
  try {
    metadata = await lstat(path);
  } catch (error) {
    if (error instanceof Error && "code" in error && error.code === "ENOENT") return null;
    throw error;
  }
  if (!metadata.isFile() || metadata.isSymbolicLink() || metadata.size > MAXIMUM_MARKER_BYTES) {
    return null;
  }
  let value: unknown;
  try {
    value = JSON.parse(await readFile(path, "utf8"));
  } catch {
    return null;
  }
  return validMarker(value) ? value : null;
}

export async function writeStartupMarker(path: string, marker: StartupMarker): Promise<void> {
  await writeFileAtomic(path, `${JSON.stringify(marker)}\n`, 0o600);
}

export async function removeStartupMarker(path: string): Promise<void> {
  await rm(path, { force: true });
}
