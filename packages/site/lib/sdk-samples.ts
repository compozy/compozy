import type { InlineCodeUsageGenerator } from "fumadocs-openapi/requests/generators";
import type { OpenAPIPageProps_Preloaded } from "fumadocs-openapi/ui";

export interface SDKSampleManifest {
  version: number;
  operations: Record<
    string,
    {
      typescript?: string;
      go?: string;
      kind: "json" | "sse" | "prompt" | "websocket";
      app_access: "allowed" | "denied";
      resource: string;
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

/** Add only the rendered page's samples without changing the shared preloaded document. */
export function withSDKCodeSamples(
  props: OpenAPIPageProps_Preloaded,
  manifest: SDKSampleManifest
): OpenAPIPageProps_Preloaded {
  const document = props.preloaded.docs[props.document];
  if (!document) return props;

  const paths = { ...document.paths };
  for (const { path, method } of props.operations ?? []) {
    const item = paths[path];
    const operation = item?.[method];
    if (!operation) continue;
    paths[path] = {
      ...item,
      [method]: {
        ...operation,
        "x-codeSamples": generateSDKCodeSamples(manifest, operation.operationId),
      },
    };
  }
  return {
    ...props,
    preloaded: {
      ...props.preloaded,
      docs: { ...props.preloaded.docs, [props.document]: { ...document, paths } },
    },
  };
}

export function appAllowedOperations(manifest: SDKSampleManifest): [string, string[]][] {
  const allowed = new Map<string, string[]>();
  for (const [operationId, operation] of Object.entries(manifest.operations)) {
    if (operation.app_access !== "allowed") continue;
    const operations = allowed.get(operation.resource) ?? [];
    operations.push(operationId);
    allowed.set(operation.resource, operations);
  }
  for (const operations of allowed.values()) operations.sort();
  return [...allowed].sort(([left], [right]) => left.localeCompare(right));
}
