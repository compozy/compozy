import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import type { OperationObject } from "fumadocs-openapi";
import type { OpenAPIPageProps_Preloaded } from "fumadocs-openapi/ui";
import { AppAllowedOperations } from "@/components/docs/app-allowed-operations";
import { generateSDKCodeSamples, withSDKCodeSamples, type SDKSampleManifest } from "../sdk-samples";
import { API_TAG_ICONS, DOCS_ICONS } from "../docs-icons";
import { API_SECTIONS } from "../docs-navigation";

const HERE = dirname(fileURLToPath(import.meta.url));
const SITE_ROOT = resolve(HERE, "..", "..");
const REPO_ROOT = resolve(SITE_ROOT, "../..");
const OPENAPI_PATH = resolve(REPO_ROOT, "openapi/compozy.json");

type OpenAPIDocument = {
  paths?: Record<string, Record<string, { tags?: string[] }>>;
  tags?: { name: string }[];
};

function loadOpenAPI(): OpenAPIDocument {
  return JSON.parse(readFileSync(OPENAPI_PATH, "utf8"));
}

function tagSlug(name: string): string {
  return name.toLowerCase().replace(/\s+/g, "-");
}

function collectUsedTags(doc: OpenAPIDocument): string[] {
  const tags = new Set<string>();
  for (const path of Object.values(doc.paths ?? {})) {
    for (const op of Object.values(path)) {
      for (const tag of op.tags ?? []) tags.add(tag);
    }
  }
  return [...tags].sort();
}

describe("api reference", () => {
  it("Should partition every used tag into exactly one navigation section", () => {
    const usedTags = collectUsedTags(loadOpenAPI()).map(tagSlug);
    const sectionByTag = new Map<string, string[]>();
    for (const section of API_SECTIONS) {
      for (const id of section.ids) {
        const list = sectionByTag.get(id) ?? [];
        list.push(section.label);
        sectionByTag.set(id, list);
      }
    }

    const duplicates = [...sectionByTag.entries()].filter(([, labels]) => labels.length > 1);
    expect(duplicates).toEqual([]);

    const unmapped = usedTags.filter(tag => !sectionByTag.has(tag));
    expect(unmapped).toEqual([]);
  });

  it("Should map every used tag to an icon registered in the docs icon registry", () => {
    const usedTags = collectUsedTags(loadOpenAPI()).map(tagSlug);
    const missing = usedTags.filter(tag => !(tag in API_TAG_ICONS));
    expect(missing).toEqual([]);

    const dangling = Object.entries(API_TAG_ICONS)
      .filter(([, icon]) => !(icon in DOCS_ICONS))
      .map(([tag, icon]) => `${tag} -> ${icon}`);
    expect(dangling).toEqual([]);
  });

  it("Should use the generated SDK calls for JSON, prompt, and streaming operations", () => {
    const manifest: SDKSampleManifest = {
      version: 1,
      operations: {
        listTasks: {
          typescript:
            'const compozy = await Compozy.connect();\nawait compozy.tasks.listTasks({ query: { status: "in_progress" } });',
          go: "c.ListTasks(ctx, client.ListTasksParams{})",
          kind: "json",
          app_access: "allowed",
          resource: "tasks",
        },
        sendSessionPrompt: {
          typescript: "await compozy.prompt({ workspaceId, sessionId, message });",
          go: "c.Prompt(ctx, input)",
          kind: "prompt",
          app_access: "allowed",
          resource: "sessions",
        },
        streamTask: {
          typescript: 'for await (const frame of compozy.events("streamTask", options)) {}',
          go: "frame, err := stream.Next()",
          kind: "sse",
          app_access: "allowed",
          resource: "tasks",
        },
        unsupportedGo: {
          typescript: "// use the documented transport",
          kind: "json",
          app_access: "denied",
          resource: "unsupported",
        },
      },
    };

    for (const [operationId, expected] of Object.entries(manifest.operations)) {
      const samples = generateSDKCodeSamples(manifest, operationId);
      expect(samples.map(sample => sample.id)).toEqual(["sdk-typescript", "sdk-go"]);
      expect(samples[0]).toMatchObject({ label: "TypeScript SDK", source: expected.typescript });
      expect(samples[1]).toMatchObject({ label: "Go SDK", source: expected.go || false });
    }
    expect(
      generateSDKCodeSamples(manifest, "missing").every(sample => sample.source === false)
    ).toBe(true);
  });

  it("Should preload only this page's SDK samples without mutating shared documents", () => {
    const samples: SDKSampleManifest = {
      version: 1,
      operations: {
        listTasks: {
          resource: "tasks",
          kind: "json",
          app_access: "allowed",
          typescript: "list()",
          go: "List()",
        },
        createTask: {
          resource: "tasks",
          kind: "json",
          app_access: "allowed",
          typescript: "create()",
          go: "Create()",
        },
        listSessions: {
          resource: "sessions",
          kind: "json",
          app_access: "allowed",
          typescript: "unrelated sample",
        },
        websocket: {
          resource: "sessions",
          kind: "websocket",
          app_access: "denied",
          typescript: "",
          go: "",
        },
      },
    };
    const props: OpenAPIPageProps_Preloaded = {
      document: "compozy",
      operations: [
        { path: "/tasks", method: "get" },
        { path: "/tasks", method: "post" },
        { path: "/socket", method: "get" },
        { path: "/missing", method: "get" },
      ],
      preloaded: {
        docs: {
          compozy: {
            openapi: "3.2.0",
            info: { title: "Test", version: "1" },
            paths: {
              "/tasks": {
                summary: "Task operations",
                get: { operationId: "listTasks", responses: {} },
                post: { operationId: "createTask", responses: {} },
              },
              "/sessions": { get: { operationId: "listSessions", responses: {} } },
              "/socket": { get: { operationId: "websocket", responses: {} } },
              "/missing": { get: { operationId: "missing", responses: {} } },
            },
          },
        },
      },
    };
    const before = JSON.stringify(props);
    const result = withSDKCodeSamples(props, samples);
    const paths = result.preloaded.docs.compozy.paths!;
    for (const { path, method } of props.operations!) {
      const operation = paths[path]![method] as OperationObject;
      expect(operation["x-codeSamples"]).toEqual(
        generateSDKCodeSamples(samples, operation.operationId)
      );
    }
    expect(paths["/tasks"]?.summary).toBe("Task operations");
    expect(paths["/sessions"]).not.toHaveProperty("get.x-codeSamples");
    expect(JSON.stringify(result)).not.toContain("unrelated sample");
    expect(JSON.stringify(props)).toBe(before);
    expect(
      generateSDKCodeSamples(samples, "websocket").every(sample => sample.source === false)
    ).toBe(true);
  });

  it("Should group only browser-allowed operations by their generated resource", () => {
    const samples: SDKSampleManifest = {
      version: 1,
      operations: {
        listTasks: { resource: "tasks", kind: "json", app_access: "allowed" },
        getTask: { resource: "tasks", kind: "json", app_access: "allowed" },
        listSessions: { resource: "sessions", kind: "json", app_access: "allowed" },
        execTerminal: { resource: "terminals", kind: "json", app_access: "denied" },
      },
    };
    const html = renderToStaticMarkup(createElement(AppAllowedOperations, { samples }));
    expect(html).toContain("listTasks");
    expect(html).not.toContain("execTerminal");
    expect(html).not.toContain("terminals");
    expect(html.match(/<table\b/g)).toHaveLength(2);
    expect(html).toMatch(/<caption[^>]*>sessions<\/caption>/);
    expect(html).toMatch(/<caption[^>]*>tasks<\/caption>/);
    expect(html.indexOf("listSessions")).toBeLessThan(html.indexOf("getTask"));
    expect(html.indexOf("getTask")).toBeLessThan(html.indexOf("listTasks"));
  });
});
