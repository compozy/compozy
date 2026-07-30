import { pathToFileURL } from "node:url";

import {
  Extension,
  type ExtensionOptions,
  type MemoryForgetParams,
  type MemoryRecallParams,
  type MemoryStoreParams,
} from "@compozy/extension-sdk";

interface StoredEntry {
  content: string;
  tags: string[];
}

export function createExtension(options: ExtensionOptions = {}): Extension {
  const memory = new Map<string, StoredEntry>();
  const extension = new Extension(
    {
      name: "__EXTENSION_NAME__",
      version: "0.1.0",
      description: "In-memory extension memory backend",
      subprocess: { command: "node", args: ["index.js"] },
      capabilities: { provides: ["memory.backend"] },
      permissions: { requires: [] },
    },
    options
  );

  extension.handle("memory/store", async (_context, params: MemoryStoreParams) => {
    memory.set(params.key, { content: params.content, tags: [...(params.tags ?? [])] });
    return {};
  });
  extension.handle("memory/recall", async (_context, params: MemoryRecallParams) => {
    const query = params.query.toLowerCase();
    const entries = [...memory.entries()]
      .filter(([key, value]) => `${key} ${value.content}`.toLowerCase().includes(query))
      .slice(0, params.limit ?? 10)
      .map(([key, value]) => ({ key, content: value.content, score: 1 }));
    return { entries };
  });
  extension.handle("memory/forget", async (_context, params: MemoryForgetParams) => {
    memory.delete(params.key);
    return {};
  });
  return extension;
}

const entryPoint = process.argv[1];
if (entryPoint && import.meta.url === pathToFileURL(entryPoint).href) {
  void createExtension().start();
}
