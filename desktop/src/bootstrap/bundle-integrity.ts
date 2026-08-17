import { timingSafeEqual } from "node:crypto";
import { readFile } from "node:fs/promises";
import { basename } from "node:path";

import { executableSha256File } from "./executable-digest";

export const RUNTIME_BUNDLE_MANIFEST_SCHEMA_VERSION = 1;

export interface RuntimeBundleManifest {
  readonly schema_version: typeof RUNTIME_BUNDLE_MANIFEST_SCHEMA_VERSION;
  readonly asset: string;
  readonly sha256: string;
}

function validManifest(value: unknown): value is RuntimeBundleManifest {
  if (!value || typeof value !== "object") return false;
  const record = value as Record<string, unknown>;
  return (
    record.schema_version === RUNTIME_BUNDLE_MANIFEST_SCHEMA_VERSION &&
    typeof record.asset === "string" &&
    record.asset.trim() !== "" &&
    typeof record.sha256 === "string" &&
    /^[a-f0-9]{64}$/u.test(record.sha256)
  );
}

export async function verifyRuntimeBundle(binaryPath: string, manifestPath: string): Promise<void> {
  const value: unknown = JSON.parse(await readFile(manifestPath, "utf8"));
  if (!validManifest(value)) throw new Error("The bundled runtime manifest is invalid.");
  if (basename(binaryPath) !== value.asset) {
    throw new Error(
      "The bundled runtime does not match its integrity manifest. Reinstall the app."
    );
  }
  const expected = Buffer.from(value.sha256, "hex");
  const actual = await executableSha256File(binaryPath);
  if (expected.length !== actual.length || !timingSafeEqual(expected, actual)) {
    throw new Error("The bundled CompozyOS runtime failed its integrity check. Reinstall the app.");
  }
}
