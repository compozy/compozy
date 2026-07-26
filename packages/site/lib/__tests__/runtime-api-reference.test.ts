import { readFileSync, existsSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { API_SECTIONS } from "../runtime-navigation";

const HERE = dirname(fileURLToPath(import.meta.url));
const SITE_ROOT = resolve(HERE, "..", "..");
const REPO_ROOT = resolve(SITE_ROOT, "../..");
const OPENAPI_PATH = resolve(REPO_ROOT, "openapi/agh.json");
const API_REF_DIR = resolve(SITE_ROOT, "content/runtime/api-reference");

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
  it("Should generate one MDX page for every OpenAPI tag with operations", () => {
    const usedTags = collectUsedTags(loadOpenAPI());
    expect(usedTags.length).toBeGreaterThan(0);

    const missing = usedTags.filter(
      tag => !existsSync(resolve(API_REF_DIR, `${tagSlug(tag)}.mdx`))
    );
    expect(missing).toEqual([]);
  });

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
});
