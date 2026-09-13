import type { ExtensionEntry } from "../types";

/** Safe structured fields used to recover from a refused extension lifecycle operation. */
export interface ExtensionOperationErrorMetadata {
  readonly installedOrigin?: NonNullable<ExtensionEntry["origin"]>;
  readonly code?: string;
  readonly currentDigest?: string;
  readonly listedDigest?: string;
  readonly fetchedDigest?: string;
  readonly inputId?: string;
  readonly requiredInputs?: readonly string[];
}

function errorString(error: object, field: string): string | undefined {
  const value: unknown = Reflect.get(error, field);
  return typeof value === "string" && value.trim() !== "" ? value.trim() : undefined;
}

export function extensionOperationErrorMetadata(error: unknown): ExtensionOperationErrorMetadata {
  if (error === null || typeof error !== "object") return {};
  const inputs: unknown = Reflect.get(error, "inputs");
  return {
    code: errorString(error, "code"),
    installedOrigin: installedOrigin(error),
    currentDigest: errorString(error, "current_digest"),
    listedDigest: errorString(error, "listed_digest"),
    fetchedDigest: errorString(error, "fetched_digest"),
    inputId: errorString(error, "input_id"),
    requiredInputs:
      Array.isArray(inputs) && inputs.every((input): input is string => typeof input === "string")
        ? [...inputs]
        : undefined,
  };
}

function installedOrigin(error: object): ExtensionOperationErrorMetadata["installedOrigin"] {
  const origin: unknown = Reflect.get(error, "installed_origin");
  if (origin === null || typeof origin !== "object") return undefined;
  const sourceRef = errorString(origin, "source_ref");
  const entryId = errorString(origin, "entry_id");
  if (!sourceRef || !entryId) return undefined;
  return { source: errorString(origin, "source") ?? "", source_ref: sourceRef, entry_id: entryId };
}
