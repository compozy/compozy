import { InvalidParamsError } from "./errors.js";
import type { ExtensionToolContext } from "./extension-contract.js";
import type { ClarifyAskParams, ExtensionToolCallRequest, JSONValue } from "./types.js";

export function makeExtensionToolContext<TInput = JSONValue>(
  call: ExtensionToolCallRequest,
  context: ExtensionToolContext["context"]
): ExtensionToolContext<TInput> {
  const invocationId = call.invocation_id;
  return {
    input: call.input as TInput,
    context,
    host: context.host,
    session: context.session,
    toolID: call.tool_id,
    handler: call.handler,
    ...(call.trusted_workspace !== undefined ? { trustedWorkspace: call.trusted_workspace } : {}),
    ...(invocationId !== undefined ? { invocationId } : {}),
    askClarification: async question => {
      if (invocationId === undefined || invocationId.trim() === "") {
        throw new InvalidParamsError("tool invocation context is required");
      }
      const params: ClarifyAskParams = {
        invocation_id: invocationId,
        question: question.question,
        ...(question.choices === undefined ? {} : { choices: [...question.choices] }),
      };
      return await context.host.request("clarify/ask", params);
    },
  };
}
