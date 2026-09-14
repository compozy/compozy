import { z } from "zod";

import type { ExtensionEntry, ExtensionInstallPreview } from "../types";

/** Safe structured fields used to recover from a refused extension lifecycle operation. */
export interface ExtensionOperationErrorMetadata {
  readonly installedOrigin?: NonNullable<ExtensionEntry["origin"]>;
  readonly code?: string;
  readonly currentDigest?: string;
  readonly listedDigest?: string;
  readonly fetchedDigest?: string;
  readonly inputId?: string;
  readonly requiredInputs?: readonly string[];
  readonly inputDefinitions?: ExtensionInstallPreview["inputs"];
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
    inputDefinitions: inputDefinitions(error),
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

const inputFields = {
  id: z.string().min(1),
  prompt: z.string(),
  required: z.boolean(),
  binding: z.object({ type: z.enum(["env", "url_query"]), name: z.string().min(1) }),
};
const inputDefinitionsSchema = z.array(
  z.discriminatedUnion("type", [
    z.object({ ...inputFields, type: z.literal("secret"), default: z.never().optional() }),
    z.object({ ...inputFields, type: z.literal("boolean"), default: z.boolean().optional() }),
    z.object({
      ...inputFields,
      type: z.enum(["string", "identifier"]),
      default: z.string().optional(),
    }),
  ])
);

function inputDefinitions(error: object): ExtensionOperationErrorMetadata["inputDefinitions"] {
  const result = inputDefinitionsSchema.safeParse(Reflect.get(error, "input_definitions"));
  return result.success ? result.data : undefined;
}
