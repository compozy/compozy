import type {
  ExtensionProvideToolsResponse,
  ExtensionToolCallResponse,
  ExtensionToolRuntimeDescriptor,
} from "./types.js";
import type { ExtensionContext, RegisteredTool } from "./extension-contract.js";
import { InvalidParamsError, MethodNotFoundError, isRPCError } from "./errors.js";
import {
  normalizeToolResult,
  parseToolCallRequest,
  toolExecutionError,
} from "./extension-runtime.js";
import { makeExtensionToolContext } from "./extension-tool-context.js";

export function provideRegisteredTools(
  tools: ExtensionToolRuntimeDescriptor[]
): ExtensionProvideToolsResponse {
  return { tools };
}

export async function callRegisteredTool(
  tools: ReadonlyMap<string, RegisteredTool>,
  params: unknown,
  context: ExtensionContext
): Promise<ExtensionToolCallResponse> {
  const call = parseToolCallRequest(params);
  const registered = tools.get(call.handler);
  if (!registered) {
    throw new MethodNotFoundError(call.handler);
  }
  if (registered.descriptor.id !== call.tool_id) {
    throw new InvalidParamsError("tool_id does not match handler", {
      expected_tool_id: registered.descriptor.id,
      actual_tool_id: call.tool_id,
      handler: call.handler,
    });
  }

  try {
    const result = await registered.handler(makeExtensionToolContext(call, context));
    return { result: normalizeToolResult(result) };
  } catch (error) {
    if (isRPCError(error)) {
      throw error;
    }
    throw toolExecutionError(error, call, registered.sensitiveInputFields);
  }
}
