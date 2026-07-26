import { pathToFileURL } from "node:url";

import {
  Extension,
  type ExecuteHookParams,
  type ExtensionOptions,
  type PromptPatch,
} from "@agh/extension-sdk";

export function createExtension(options: ExtensionOptions = {}): Extension {
  const extension = new Extension(
    {
      name: "__EXTENSION_NAME__",
      version: "0.1.0",
      capabilities: { provides: ["prompt.provider"] },
      supported_hook_events: ["prompt.post_assemble"],
    },
    options
  );

  extension.handle(
    "execute_hook",
    async (_ctx, params: ExecuteHookParams<"prompt.post_assemble">) => {
      const prompt = params.payload.prompt ?? "";
      const workspace = params.payload.workspace ?? params.payload.workspace_id ?? "unknown";
      return {
        prompt: `[Workspace: ${workspace}]\n\n${prompt}`,
      } satisfies PromptPatch;
    }
  );

  return extension;
}

const entryPoint = process.argv[1];
if (entryPoint && import.meta.url === pathToFileURL(entryPoint).href) {
  const extension = createExtension();
  void extension.start();
}
