import type { InlineCodeUsageGenerator } from "fumadocs-openapi/requests/generators";

export interface SDKSampleManifest {
  version: number;
  operations: Record<
    string,
    {
      typescript?: string;
      go?: string;
      kind: "json" | "sse" | "prompt" | "websocket";
      app_access: "allowed" | "denied";
    }
  >;
}

/** Method names and stream handling belong to the SDK generator, not the documentation site. */
export function generateSDKCodeSamples(
  manifest: SDKSampleManifest,
  operationId: string | undefined
): InlineCodeUsageGenerator[] {
  const sample = operationId ? manifest.operations[operationId] : undefined;
  return [
    {
      id: "sdk-typescript",
      label: "TypeScript SDK",
      lang: "ts",
      source: sample?.typescript || false,
    },
    { id: "sdk-go", label: "Go SDK", lang: "go", source: sample?.go || false },
  ];
}

export function appAllowedOperations(manifest: SDKSampleManifest): string[] {
  const allowed: string[] = [];
  for (const [operationId, operation] of Object.entries(manifest.operations)) {
    if (operation.app_access === "allowed") allowed.push(operationId);
  }
  return allowed.sort();
}
