import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { OperationStrategy, toCase, type UserConfig } from "@hey-api/openapi-ts";

type Operation = {
  operationId: string;
  tags?: string[];
  "x-compozy-stream"?: string;
  "x-stability"?: string;
  responses?: Record<string, { content?: Record<string, unknown> }>;
};

const methods = new Set(["get", "post", "put", "patch", "delete", "head", "options", "trace"]);
const promptOperation = "sendSessionPrompt";

export function sdkOperations(input: string) {
  const document = JSON.parse(readFileSync(input, "utf8")) as {
    paths: Record<string, Record<string, Operation>>;
  };
  const seen = new Set<string>();
  return Object.entries(document.paths)
    .flatMap(([path, item]) =>
      Object.entries(item).flatMap(([method, operation]) => {
        if (!methods.has(method)) return [];
        const id = operation.operationId;
        if (!id || !/^[a-zA-Z_$][\w$]*$/.test(id) || seen.has(id)) {
          throw new Error(
            `Invalid or duplicate SDK operation id at ${method.toUpperCase()} ${path}: ${id}`
          );
        }
        seen.add(id);
        const stream =
          Boolean(operation["x-compozy-stream"]) ||
          Object.values(operation.responses ?? {}).some(response =>
            Object.hasOwn(response.content ?? {}, "text/event-stream")
          );
        const websocket =
          Object.hasOwn(operation.responses ?? {}, "101") &&
          !Object.keys(operation.responses ?? {}).some(status => /^2(?:\d\d|XX)$/.test(status));
        return [
          {
            id,
            method,
            path,
            stream,
            websocket,
            prompt: id === promptOperation,
            stability: operation["x-stability"],
          },
        ];
      })
    )
    .sort((left, right) => left.id.localeCompare(right.id));
}

export function createSDKConfigs(input: string, output: string): UserConfig[] {
  const operations = sdkOperations(input);
  const select = (stream: boolean) =>
    operations
      .filter(operation => !operation.prompt && !operation.websocket && operation.stream === stream)
      .map(operation => `${operation.method.toUpperCase()} ${operation.path}`);
  return [false, true].map(stream => ({
    input,
    output: {
      path: resolve(output, stream ? "sse" : "sdk"),
      case: "preserve",
      module: { extension: ".js" },
      tsConfigPath: null,
    },
    logs: { level: "error" },
    parser: {
      filters: { operations: { include: select(stream) } },
      patch: {
        operations: (_method, _path, operation) => {
          const stability = (operation as { "x-stability"?: string })["x-stability"];
          if (stability) {
            operation.description = [operation.description, `@stability ${stability}`]
              .filter(Boolean)
              .join("\n\n");
          }
        },
      },
    },
    plugins: [
      { name: "@hey-api/client-fetch", throwOnError: true },
      {
        name: "@hey-api/typescript",
        definitions: { case: "preserve" },
        requests: {
          case: "preserve",
          name: name => `${name[0]!.toUpperCase()}${name.slice(1)}Data`,
        },
        errors: {
          case: "preserve",
          name: name => `${name[0]!.toUpperCase()}${name.slice(1)}Errors`,
          error: name => `${name[0]!.toUpperCase()}${name.slice(1)}Error`,
        },
        responses: {
          case: "preserve",
          name: name => `${name[0]!.toUpperCase()}${name.slice(1)}Responses`,
          response: name => `${name[0]!.toUpperCase()}${name.slice(1)}Response`,
        },
      },
      {
        name: "@hey-api/sdk",
        responseStyle: "data",
        operations: {
          // Keeping this preset preserves operation-id type names in Hey API 0.99.0.
          nesting: "operationId",
          methods: "instance",
          methodName: { casing: "preserve" },
          segmentName: { casing: "camelCase" },
          strategy: stream
            ? "flat"
            : OperationStrategy.single({
                root: "Compozy",
                path: operation => [
                  toCase(operation.tags?.[0] ?? "misc", "camelCase"),
                  operation.operationId!,
                ],
              }),
        },
      },
    ],
  }));
}

const directory = dirname(fileURLToPath(import.meta.url));
export default () =>
  createSDKConfigs(
    resolve(directory, "../../openapi/compozy.json"),
    resolve(directory, "src/generated")
  );
