import { z } from "zod";

import type { ExtensionEntry, ExtensionInstallPreview } from "../types";

/** Safe structured fields used to recover from a refused extension lifecycle operation. */
export interface ExtensionOperationErrorMetadata {
  readonly installedOrigin?: NonNullable<ExtensionEntry["origin"]>;
  readonly code?: string;
  readonly listedDigest?: string;
  readonly fetchedDigest?: string;
  readonly inputId?: string;
  readonly requiredInputs?: readonly string[];
  readonly inputDefinitions?: ExtensionInstallPreview["inputs"];
  /** Dependencies a consented install completed before it failed; they stay installed. */
  readonly dependenciesInstalled?: readonly string[];
}

function errorStrings(error: object, field: string): readonly string[] | undefined {
  const value: unknown = Reflect.get(error, field);
  return Array.isArray(value) && value.every((item): item is string => typeof item === "string")
    ? [...value]
    : undefined;
}

function errorString(error: object, field: string): string | undefined {
  const value: unknown = Reflect.get(error, field);
  return typeof value === "string" && value.trim() !== "" ? value.trim() : undefined;
}

export function extensionOperationErrorMetadata(error: unknown): ExtensionOperationErrorMetadata {
  if (error === null || typeof error !== "object") return {};
  return {
    code: errorString(error, "code"),
    dependenciesInstalled: errorStrings(error, "dependencies_installed"),
    inputDefinitions: inputDefinitions(error),
    installedOrigin: installedOrigin(error),
    listedDigest: errorString(error, "listed_digest"),
    fetchedDigest: errorString(error, "fetched_digest"),
    inputId: errorString(error, "input_id"),
    requiredInputs: errorStrings(error, "inputs"),
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
  binding: z.object({ type: z.enum(["env", "url_query", "template"]), name: z.string().min(1) }),
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
