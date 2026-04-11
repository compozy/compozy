import { Extension } from "@compozy/extension-sdk";
import type { PromptPostBuildPayload, PromptTextPatch } from "@compozy/extension-sdk";

export function createPromptDecoratorExtension(
  name = "__EXTENSION_NAME__",
  version = "__EXTENSION_VERSION__"
): Extension {
  return new Extension(name, version).onPromptPostBuild(
    async (_context, payload: PromptPostBuildPayload): Promise<PromptTextPatch> => ({
      prompt_text: `${payload.prompt_text}\n\nDecorated by ${name}.`,
    })
  );
}
