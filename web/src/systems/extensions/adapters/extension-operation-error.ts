/** Safe structured fields used to recover from a refused extension lifecycle operation. */
export interface ExtensionOperationErrorMetadata {
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
