import { pathToFileURL } from "node:url";

import {
  Extension,
  type ExtensionOptions,
  type HealthCheckResult,
  type HostAPI,
  type MemoryForgetParams,
  type MemoryRecallParams,
  type MemoryStoreParams,
} from "@agh/extension-sdk";

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
      capabilities: { provides: ["memory.backend"] },
      actions: { requires: ["sessions/list"] },
      security: { capabilities: ["memory.read", "memory.write", "session.read"] },
    },
    options
  );

  extension.handle("memory/store", async (_ctx, params: MemoryStoreParams) => {
    memory.set(params.key, {
      content: params.content,
      tags: [...(params.tags ?? [])],
    });
    return {};
  });

  extension.handle("memory/recall", async (_ctx, params: MemoryRecallParams) => {
    const query = params.query.toLowerCase();
    const entries = [...memory.entries()]
      .map(([key, value]) => {
        const haystack = `${key} ${value.content} ${value.tags.join(" ")}`.toLowerCase();
        const score = haystack.includes(query) ? 1 : 0;
        return {
          key,
          content: value.content,
          score,
        };
      })
      .filter(entry => entry.score > 0)
      .slice(0, params.limit ?? 10);

    return { entries };
  });

  extension.handle("memory/forget", async (_ctx, params: MemoryForgetParams) => {
    memory.delete(params.key);
    return {};
  });

  extension.handle("health_check", async (): Promise<HealthCheckResult> => {
    return {
      healthy: true,
      message: "",
      details: {},
    };
  });

  extension.onReady(async (host: HostAPI) => {
    const sessions = await host.sessions.list();
    console.error(`Connected. ${sessions.length} active sessions.`);
  });

  return extension;
}

const entryPoint = process.argv[1];
if (entryPoint && import.meta.url === pathToFileURL(entryPoint).href) {
  const extension = createExtension();
  void extension.start();
}
