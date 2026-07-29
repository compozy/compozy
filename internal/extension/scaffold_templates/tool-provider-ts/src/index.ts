import { pathToFileURL } from "node:url";

import {
  Extension,
  type ExtensionOptions,
  type JSONValue,
  type ToolResult,
} from "@compozy/extension-sdk";

interface SearchInput {
  query: string;
}

const searchInputSchema: JSONValue = {
  type: "object",
  additionalProperties: false,
  required: ["query"],
  properties: {
    query: { type: "string" },
  },
};

export function createExtension(options: ExtensionOptions = {}): Extension {
  const extension = new Extension(
    {
      name: "__EXTENSION_NAME__",
      version: "0.1.0",
      description: "Search extension-owned data",
      subprocess: { command: "node", args: ["index.js"] },
      capabilities: { provides: [] },
      permissions: { requires: [] },
    },
    options
  );

  extension.tool<SearchInput>(
    "search",
    {
      description: "Search extension-owned data",
      readOnly: true,
      inputSchema: searchInputSchema,
    },
    async ({ input }): Promise<ToolResult> => ({
      content: [{ type: "text", text: `No results for ${input.query}` }],
      preview: input.query,
      truncated: false,
      bytes: 0,
      duration_ms: 0,
    })
  );
  return extension;
}

const entryPoint = process.argv[1];
if (entryPoint && import.meta.url === pathToFileURL(entryPoint).href) {
  void createExtension().start();
}
