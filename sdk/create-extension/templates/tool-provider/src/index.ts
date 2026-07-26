import { pathToFileURL } from "node:url";

import { Extension, type ExtensionOptions, type ToolResult } from "@agh/extension-sdk";

interface SearchInput {
  query: string;
}

const searchInputSchema = {
  type: "object",
  required: ["query"],
  properties: {
    query: { type: "string" },
  },
} as const;

export function createExtension(options: ExtensionOptions = {}): Extension {
  const extension = new Extension(
    {
      name: "__EXTENSION_NAME__",
      version: "0.1.0",
      capabilities: { provides: ["tool.provider"] },
    },
    options
  );

  extension.tool<SearchInput>(
    "search",
    {
      readOnly: true,
      inputSchema: searchInputSchema,
    },
    async ({ input }): Promise<ToolResult> => {
      return {
        content: [{ type: "text", text: `No results for ${input.query}` }],
        preview: input.query,
        truncated: false,
        bytes: 0,
        duration_ms: 0,
      };
    }
  );

  return extension;
}

const entryPoint = process.argv[1];
if (entryPoint && import.meta.url === pathToFileURL(entryPoint).href) {
  const extension = createExtension();
  void extension.start();
}
