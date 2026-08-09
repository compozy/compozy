import { pathToFileURL } from "node:url";

import {
  Extension,
  registerConnectivityProvider,
  type ExtensionOptions,
} from "@compozy/extension-sdk";

export function createExtension(options: ExtensionOptions = {}): Extension {
  const extension = new Extension(
    {
      name: "__EXTENSION_NAME__",
      version: "0.1.0",
      description: "Provide verified daemon reachability",
      subprocess: { command: "node", args: ["index.js"] },
      capabilities: { provides: ["connectivity.provider"] },
      permissions: { requires: [] },
      network_participation: {
        required: true,
        mode: "live",
        channel_scopes: ["gateway.private", "gateway.public"],
      },
    },
    options
  );

  registerConnectivityProvider(extension, {
    establish: async () => {
      throw new Error("implement provider forwarding");
    },
    status: async () => {
      throw new Error("implement provider health reporting");
    },
    teardown: async () => ({ stopped: true }),
  });
  return extension;
}

const entryPoint = process.argv[1];
if (entryPoint && import.meta.url === pathToFileURL(entryPoint).href) {
  void createExtension().start();
}
