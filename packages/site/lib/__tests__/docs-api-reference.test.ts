import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { AppAllowedOperations } from "@/components/docs/app-allowed-operations";
import { generateSDKCodeSamples, type SDKSampleManifest } from "../sdk-samples";
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
        },
        sendSessionPrompt: {
          typescript: "await compozy.prompt({ workspaceId, sessionId, message });",
          go: "c.Prompt(ctx, input)",
          kind: "prompt",
          app_access: "allowed",
        },
        streamTask: {
          typescript: 'for await (const frame of compozy.events("streamTask", options)) {}',
          go: "frame, err := stream.Next()",
          kind: "sse",
          app_access: "allowed",
        },
        unsupportedGo: {
          typescript: "// use the documented transport",
          kind: "json",
          app_access: "denied",
        },
      },
    };

    for (const [operationId, expected] of Object.entries(manifest.operations)) {
      const samples = generateSDKCodeSamples(manifest, operationId);
      expect(samples[0]).toMatchObject({ label: "TypeScript SDK", source: expected.typescript });
      expect(samples[1]).toMatchObject({ label: "Go SDK", source: expected.go || false });
    }
    expect(
      generateSDKCodeSamples(manifest, "missing").every(sample => sample.source === false)
    ).toBe(true);
  });

  it("Should render only operations that the generated contract allows for browser apps", () => {
    const samples: SDKSampleManifest = {
      version: 1,
      operations: {
        listTasks: { kind: "json", app_access: "allowed" },
        execTerminal: { kind: "json", app_access: "denied" },
      },
    };
    const html = renderToStaticMarkup(createElement(AppAllowedOperations, { samples }));
    expect(html).toContain("listTasks");
    expect(html).not.toContain("execTerminal");
  });
});
