import type {
  ExtensionToolHandler,
  ExtensionToolOptions,
  RegisteredTool,
} from "./extension-contract.js";
import {
  canonicalExtensionToolID,
  normalizeSchema,
  normalizeStringList,
} from "./extension-runtime.js";
import { schemaDigest } from "./schema-digest.js";
import type { ExtensionToolRuntimeDescriptor } from "./types.js";

export function buildRegisteredTool<TInput>(
  extensionName: string,
  handler: string,
  options: ExtensionToolOptions,
  toolHandler: ExtensionToolHandler<TInput>
): RegisteredTool {
  const inputSchema = normalizeSchema(options.inputSchema, "inputSchema");
  const outputSchema =
    options.outputSchema === undefined
      ? undefined
      : normalizeSchema(options.outputSchema, "outputSchema");
  const descriptor: ExtensionToolRuntimeDescriptor = {
    id: options.id ?? canonicalExtensionToolID(extensionName, handler),
    handler,
    ...(options.description?.trim() ? { description: options.description.trim() } : {}),
    ...(options.friendlyVerb?.trim() ? { friendly_verb: options.friendlyVerb.trim() } : {}),
    ...(options.preview?.trim() ? { preview: options.preview.trim() } : {}),
    input_schema: inputSchema,
    ...(outputSchema ? { output_schema: outputSchema } : {}),
    input_schema_digest: schemaDigest(inputSchema),
    ...(outputSchema ? { output_schema_digest: schemaDigest(outputSchema) } : {}),
    read_only: Boolean(options.readOnly),
    risk: options.risk ?? (options.readOnly ? "read" : "mutating"),
    requires_interaction: Boolean(options.requiresInteraction),
    capabilities: normalizeStringList(options.capabilities),
    ...(options.command
      ? {
          command: {
            verb: options.command.verb.trim(),
            summary: options.command.summary.trim(),
            ...(options.command.example?.trim() ? { example: options.command.example.trim() } : {}),
            flags: { ...options.command.flags },
          },
        }
      : {}),
  };

  return {
    descriptor,
    handler: toolHandler as ExtensionToolHandler,
    sensitiveInputFields: normalizeStringList(options.sensitiveInputFields),
  };
}
