import { existsSync, readdirSync, statSync, readFileSync } from "node:fs";
import { basename, dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { parse as parseYaml } from "yaml";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

vi.unmock("next/image");

vi.mock("@/lib/source", () => ({
  docsSource: {
    getPage: (slugs: string[]) => {
      const path = resolve(import.meta.dirname, "../../content/docs", `${slugs.join("/")}.mdx`);
      return existsSync(path) ? { url: `/docs/${slugs.join("/")}` } : undefined;
    },
  },
}));

vi.mock("next/link", () => ({
  default: ({ href, children }: { href: string; children: React.ReactNode }) => (
    <a href={href}>{children}</a>
  ),
}));

import { Browser } from "@/components/marketplace/browser";
import { SourcesAndIncluded } from "@/components/marketplace/closing";
import { EntryCard, EntryLogo } from "@/components/marketplace/entry-card";
import { EntryDetail } from "@/components/marketplace/entry-detail";
import { fieldTiles } from "@/components/marketplace/hero-field";
import BundledExtensionPage, {
  generateStaticParams as generateBundledExtensionParams,
} from "@/app/marketplace/bundled/[name]/page";
import MarketplaceEntryPage, {
  generateStaticParams as generateMarketplaceEntryParams,
} from "@/app/marketplace/[entryId]/page";
import {
  bundledSkills,
  bundledExtensions,
  parseBundledSkillFrontmatter,
} from "../marketplace-bundled";
import {
  extensionEntries,
  extensionEntrySchema,
  extensionFeedSchema,
  marketplacePresets,
  marketplacePresetsSchema,
  findEntry,
  marketplaceEntryPath,
  installCommand,
  marketplaceSearchCommand,
  parseMarketplaceCatalog,
} from "../marketplace-catalog";
import {
  categoriesWithEntries,
  marketplaceCards,
  relatedCards,
  toCard,
  type MarketplaceCategoryId,
} from "../marketplace-view";

// Invariant: only the current extension feed reaches rendering, with validated identity, inputs and icons.
// Owner: site catalog boundary. Canonical suite: marketplace-catalog.test.tsx.
describe("marketplace catalog", () => {
  const feed = () => ({
    manifest_version: 3,
    generated_at: "2026-09-12T10:00:00Z",
    entries: extensionEntries,
  });

  it("Should optimize published raster icons and retain the fallback after a failed load", () => {
    const entry = findEntry("context7");
    expect(entry).toBeDefined();
    if (!entry) throw new Error("The curated Context7 package is required for this journey");
    const { container } = render(<EntryLogo entry={toCard(entry)} size="lg" />);
    const icon = container.querySelector("img");
    expect(icon).not.toBeNull();
    if (!icon) throw new Error("The catalog icon did not render");
    const request = new URL(icon.src);
    expect(request.pathname).toBe("/_next/image");
    expect(request.searchParams.get("url")).toBe(entry.icon);
    expect(icon.srcset.split(",")).toHaveLength(2);
    fireEvent.error(icon);
    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector('[data-rung="monogram"]')?.textContent).toBe("C");
  });

  it("Should load all eighteen current packages without standalone skill entries", () => {
    expect(parseMarketplaceCatalog(feed())).toEqual(extensionEntries);
    expect(extensionEntries).toHaveLength(18);
    expect(findEntry("context7")?.inputs).toEqual(
      expect.arrayContaining([expect.objectContaining({ id: "context7_api_key", type: "secret" })])
    );
    expect(findEntry("documentation-writer")).toBeUndefined();
  });

  it.each([undefined, 1, 2, 4])("Should reject unsupported feed version %s", manifest_version => {
    expect(() => parseMarketplaceCatalog({ ...feed(), manifest_version })).toThrow(
      /manifest_version/
    );
  });

  it("Should reject missing timestamps and duplicate catalog identities", () => {
    expect(() => extensionFeedSchema.parse({ ...feed(), generated_at: undefined })).toThrow(
      /generated_at/
    );
    const entry = extensionEntries[0];
    expect(() => parseMarketplaceCatalog({ ...feed(), entries: [entry, entry] })).toThrow(
      /duplicated/
    );
    expect(() =>
      parseMarketplaceCatalog({ ...feed(), entries: [entry, { ...entry, entry_id: "other" }] })
    ).toThrow(/install_slug/);
  });

  it.each([
    { artifact_url: "http://example.com/archive.tar.gz" },
    { artifact_url: "https://user:pass@example.com/archive.tar.gz" },
    { artifact_url: "https://example.com/archive.tar.gz#fragment" },
    { digest_sha256: "bad-digest" },
    { tier: "invented" },
    { version: "" },
    { entry_id: "../outside" },
    { downloads: 100 },
    { rating: 5 },
    { published_at: "2026-02-31T00:00:00Z" },
  ])("Should reject invalid catalog metadata %o", fields => {
    expect(() => extensionEntrySchema.parse({ ...extensionEntries[0], ...fields })).toThrow();
  });

  it("Should derive current commands and canonical paths without changing acquisition refs", () => {
    for (const entry of extensionEntries) {
      expect(findEntry(entry.entry_id)).toEqual(entry);
      expect(marketplaceEntryPath(entry)).toBe(`/marketplace/${entry.entry_id}`);
      expect(marketplaceSearchCommand(entry)).toBe(`compozy marketplace search ${entry.entry_id}`);
      expect(installCommand(entry)).toBe(`compozy extension install ${entry.install_slug}`);
    }
    expect(installCommand(findEntry("herdr-bridge")!)).toBe(
      "compozy extension install AlexandreAkao/herdr-bridge-compozy"
    );
    expect(findEntry("batuta")).toBeUndefined();
    expect(findEntry("unknown-package")).toBeUndefined();
  });

  it.each([
    { icon: "https://example.com/icon.svg" },
    { icon: "https://example.com/icon.webp?revision=2" },
    { icon: "data:image/svg+xml,%3Csvg%2F%3E" },
    { icon: "data:image/png;base64,YWJj" },
  ])("Should accept the supported icon reference $icon", ({ icon }) => {
    expect(extensionEntrySchema.parse({ ...extensionEntries[0], icon }).icon).toBe(icon);
  });

  it.each([
    "http://example.com/icon.svg",
    "https://example.com/icon.gif",
    "https://user:pass@example.com/icon.svg",
    "data:text/html,%3Cscript%3E",
    "data:image/png;base64,invalid",
    "data:image/svg+xml,%GG",
    "x".repeat(65537),
  ])("Should reject an unsupported icon reference (%#)", icon => {
    expect(() => extensionEntrySchema.parse({ ...extensionEntries[0], icon })).toThrow(/icon/);
  });

  it("Should reject invalid typed defaults, secret query bindings and duplicate inputs", () => {
    const input = {
      id: "project",
      prompt: "Project",
      type: "identifier",
      required: true,
      binding: { type: "url_query", name: "project" },
    };
    const parse = (inputs: unknown[]) =>
      extensionEntrySchema.parse({ ...extensionEntries[0], inputs });
    expect(parse([{ ...input, default: "demo-project" }]).inputs?.[0].default).toBe("demo-project");
    expect(() => parse([input, input])).toThrow(/unique/);
    expect(() => parse([input, { ...input, id: "another" }])).toThrow(/bindings/);
    expect(() => parse([{ ...input, type: "secret" }])).toThrow(/secret/);
    expect(() => parse([{ ...input, default: "project with spaces" }])).toThrow(/URL-safe/);
    expect(() => parse([{ ...input, type: "boolean", default: "true" }])).toThrow(/boolean/);
    expect(() => parse([{ ...input, type: "string", default: "a".repeat(8193) }])).toThrow(/8 KiB/);
    expect(() => parse([{ ...input, type: "string", default: "a\0b" }])).toThrow(/NUL-free/);
  });

  it("Should preserve preset order and reject duplicate or reserved names", () => {
    expect(marketplacePresets.map(entry => [entry.name, entry.default])).toEqual([
      ["claude-plugins-official", "on"],
      ["compozy-plugins", "on"],
      ["openai-codex", "off"],
    ]);
    const feed = {
      manifest_version: 3,
      generated_at: "2026-09-12T10:00:00Z",
      entries: marketplacePresets,
    };
    expect(() =>
      marketplacePresetsSchema.parse({
        ...feed,
        entries: [marketplacePresets[0], marketplacePresets[0]],
      })
    ).toThrow(/unique/);
    expect(() =>
      marketplacePresetsSchema.parse({
        ...feed,
        entries: [{ ...marketplacePresets[0], name: "compozy" }],
      })
    ).toThrow(/reserved/);
  });

  it("accepts and normalizes the optional extension format marker", () => {
    const entry = extensionEntrySchema.parse({
      entry_id: "agent-plugin",
      name: "Agent Plugin",
      description: "A portable extension package",
      version: "1.0.0",
      install_slug: "acme/agent-plugin",
      artifact_url: "https://example.com/agent-plugin.tar.gz",
      digest_sha256: "a".repeat(64),
      tier: "official",
      format: " AGENT-PLUGIN ",
    });

    expect(entry.format).toBe("agent-plugin");
    expect(() => extensionEntrySchema.parse({ ...entry, format: "client-specific" })).toThrow(
      /format/
    );
  });

  it("Should accept curated x-compozy metadata and reject unknown curation fields", () => {
    const curated = {
      category: "development",
      featured: true,
      supersedes: ["compozy-plugins/context7", "claude-plugins-official/context7"],
    };
    const entry = extensionEntrySchema.parse({ ...extensionEntries[0], "x-compozy": curated });
    expect(entry["x-compozy"]).toEqual(curated);
    expect(findEntry("context7")?.["x-compozy"]?.supersedes).toContain(
      "claude-plugins-official/context7"
    );

    for (const invalid of [
      { category: "invented" },
      { category: "development", rating: 5 },
      { category: "development", featured: "yes" },
      { category: "development", supersedes: ["context7"] },
      { category: "development", supersedes: ["a/b/c"] },
      { featured: true },
    ]) {
      expect(() =>
        extensionEntrySchema.parse({ ...extensionEntries[0], "x-compozy": invalid })
      ).toThrow();
    }
  });
});

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..", "..", "..");

function manifestDirectories(root: string, directories: string[]): string[] {
  return directories
    .flatMap(directory => {
      const path = resolve(root, directory);
      if (statSync(path).isFile()) return [basename(dirname(path))];
      return readdirSync(path, { withFileTypes: true })
        .filter(entry => entry.isDirectory())
        .map(entry => entry.name);
    })
    .sort();
}

describe("marketplace bundled resources", () => {
  it.each(["spec-cycle", "open-design"])(
    "derives the %s inventory from its declared resources",
    name => {
      const extension = bundledExtensions.find(item => item.name === name)!;
      const extensionRoot = resolve(repoRoot, "extensions", name);
      type ResourcePath = { path: string; profile?: string };
      const manifest = JSON.parse(
        readFileSync(resolve(extensionRoot, "extension.json"), "utf8")
      ) as {
        extension: {
          name: string;
          version: string;
          description: string;
          min_compozy_version: string;
        };
        capabilities: { provides: string[] };
        resources: {
          loops?: ResourcePath[];
          skills: ResourcePath[];
          agents: ResourcePath[];
          tools?: Record<string, unknown>;
        };
      };
      const loopDirectories = (manifest.resources.loops ?? []).flatMap(({ path: parent }) =>
        readdirSync(resolve(extensionRoot, parent), { withFileTypes: true })
          .filter(entry => entry.isDirectory())
          .map(entry => ({ parent, name: entry.name }))
      );
      const loops = loopDirectories.map(({ parent, name }) => {
        const loop = parseYaml(
          readFileSync(resolve(extensionRoot, parent, name, "loop.yaml"), "utf8")
        ) as {
          meta: {
            name: string;
            description: string;
            catalog?: { use_when?: string; category?: string };
          };
        };
        return {
          name: loop.meta.name,
          description: loop.meta.description,
          useWhen: loop.meta.catalog?.use_when,
          category: loop.meta.catalog?.category,
        };
      });

      expect(extension).toMatchObject({
        name: manifest.extension.name,
        version: manifest.extension.version,
        description: manifest.extension.description,
        minCompozyVersion: manifest.extension.min_compozy_version,
        provides: manifest.capabilities.provides,
        loops,
        skills: manifestDirectories(
          extensionRoot,
          manifest.resources.skills.map(resource => resource.path)
        ),
        agents: manifestDirectories(
          extensionRoot,
          manifest.resources.agents.map(resource => resource.path)
        ),
      });
      expect(extension.tools).toHaveLength(Object.keys(manifest.resources.tools ?? {}).length);
    }
  );

  it("offers inspection and a detail page for each bundled extension", async () => {
    render(<SourcesAndIncluded />);
    expect(generateBundledExtensionParams()).toEqual([
      { name: "spec-cycle" },
      { name: "open-design" },
    ]);
    for (const extension of bundledExtensions) {
      expect(
        screen.getByRole("link", { name: new RegExp(extension.displayName) }).getAttribute("href")
      ).toBe(extension.path);
      expect(extension.statusCommand).toBe(`compozy extension status ${extension.name}`);
      expect(findEntry(extension.name)).toBeUndefined();
      const detail = render(
        await BundledExtensionPage({ params: Promise.resolve({ name: extension.name }) })
      );
      expect(screen.getByRole("heading", { level: 1, name: extension.displayName })).toBeDefined();
      detail.unmount();
    }
  });

  it("reads every bundled skill's identity from its SKILL.md", () => {
    const skillRoot = resolve(repoRoot, "skills");
    const expected = readdirSync(skillRoot, { withFileTypes: true })
      .filter(entry => entry.isDirectory())
      .map(entry => {
        const source = readFileSync(resolve(skillRoot, entry.name, "SKILL.md"), "utf8");
        const match = /^---\r?\n([\s\S]*?)\r?\n---(?:\r?\n|$)/.exec(source);
        if (!match?.[1]) throw new Error(`skills/${entry.name}/SKILL.md has no frontmatter`);
        const metadata = parseYaml(match[1]) as { name?: string; description?: string };
        return {
          name: metadata.name,
          description: metadata.description,
        };
      })
      .sort((left, right) => left.name?.localeCompare(right.name ?? "") ?? 0);

    expect(
      bundledSkills.map(skill => ({ name: skill.name, description: skill.description }))
    ).toEqual(expected);
  });

  it("reads skill identity only from YAML frontmatter", () => {
    expect(
      parseBundledSkillFrontmatter(`---
name: canonical-skill
description: >-
  Folded YAML description
---

name: body-content
description: body content must not override metadata
`)
    ).toEqual({
      name: "canonical-skill",
      description: "Folded YAML description",
    });
  });
});

describe("marketplace rendering boundary", () => {
  function renderBrowser(category: MarketplaceCategoryId | null = null) {
    const onCategory = vi.fn();
    function Harness() {
      const [query, setQuery] = useState("");
      return (
        <Browser
          entries={marketplaceCards}
          category={category}
          query={query}
          onCategory={onCategory}
          onQuery={setQuery}
        />
      );
    }
    return { ...render(<Harness />), onCategory };
  }

  // Invariant: the page view model keeps every current entry, in the daemon's category order, with
  // only categories that have entries. Owner: lib/marketplace-view. Canonical suite: this file.
  it("Should derive cards, categories with counts and related entries from the current catalog", () => {
    expect(marketplaceCards.map(card => card.id)).toEqual(
      extensionEntries.map(entry => entry.entry_id)
    );
    const categories = categoriesWithEntries(marketplaceCards);
    expect(categories.every(category => category.count > 0)).toBe(true);
    expect(categories.reduce((sum, category) => sum + category.count, 0)).toBe(
      extensionEntries.length
    );
    expect(marketplaceCards.filter(card => card.featured).map(card => card.id)).toEqual(
      extensionEntries.filter(entry => entry["x-compozy"]?.featured).map(entry => entry.entry_id)
    );
    const postgres = toCard(findEntry("postgres")!);
    expect(postgres).toMatchObject({ category: "data", href: "/marketplace/postgres" });
    const related = relatedCards(postgres);
    expect(related.some(card => card.id === "postgres")).toBe(false);
    expect(related.every(card => card.category === "data")).toBe(true);
  });

  it("Should fill the hero field with distinct live entries that have icons", () => {
    const { left, right } = fieldTiles(marketplaceCards);
    const ids = [...left, ...right].map(tile => tile.entry.id);
    expect(new Set(ids).size).toBe(ids.length);
    expect([...left, ...right].every(tile => tile.entry.icon)).toBe(true);
    expect(ids).toContain("linear");
  });

  // Invariant: catalog search filters current entries and recovers from an empty result.
  // Owner: public catalog browser. Canonical suite: marketplace-catalog.test.tsx.
  // eslint-disable-next-line compozy-testing/deterministic-tests -- Full-catalog accessible-name queries are slow in jsdom under CI contention.
  it("Should filter the current catalog and restore browsing when search is cleared", () => {
    const { onCategory } = renderBrowser();
    const search = screen.getByRole("searchbox", { name: "Search the catalog" });
    fireEvent.change(search, { target: { value: "  CONTEXT7  " } });
    expect(screen.getByRole("link", { name: "Context7" }).getAttribute("href")).toBe(
      "/marketplace/context7"
    );
    expect(screen.getAllByRole("link")).toHaveLength(1);
    fireEvent.change(search, { target: { value: "no-such-package" } });
    expect(screen.queryAllByRole("link")).toHaveLength(0);
    expect(screen.getByText(/Nothing matches/)).toBeTruthy();
    fireEvent.change(search, { target: { value: "" } });
    expect(screen.getByRole("heading", { name: /^All/ })).toBeDefined();
    const nav = screen.getByRole("navigation", { name: "Categories" });
    expect(within(nav).getAllByRole("button")).toHaveLength(
      categoriesWithEntries(marketplaceCards).length + 1
    );
    fireEvent.click(within(nav).getByRole("button", { name: /Data & Analytics/ }));
    expect(onCategory).toHaveBeenCalledWith("data");
  }, 15_000);

  it("Should list every entry of a category with its count", () => {
    renderBrowser("data");
    const expected = marketplaceCards.filter(card => card.category === "data");
    expect(
      screen.getByRole("heading", { name: `Data & Analytics ${expected.length}` })
    ).toBeDefined();
    expect(screen.getAllByRole("link").map(link => link.getAttribute("href"))).toEqual(
      expected.map(card => card.href)
    );
  });

  it("Should link cards to their detail page without invented trust signals", () => {
    const entry = extensionEntries[0];
    const card = render(<EntryCard entry={toCard(entry)} />);
    expect(card.getByRole("link", { name: entry.name }).getAttribute("href")).toBe(
      marketplaceEntryPath(entry)
    );
    expect(card.container.textContent).not.toMatch(/\b(rating|downloads|featured)\b/i);
    card.unmount();
    const detail = render(<EntryDetail entry={entry} />);
    expect(detail.container.textContent).not.toMatch(/\b(rating|downloads|featured)\b/i);
  });

  it("Should show declared extension inputs with the owning install command", () => {
    const entry = findEntry("context7");
    if (!entry?.inputs?.length) throw new Error("Context7 fixture lacks its declared input");
    const withDefaults = extensionEntrySchema.parse({
      ...entry,
      inputs: [
        ...entry.inputs,
        {
          id: "region",
          prompt: "Region",
          type: "string",
          required: false,
          default: "west",
          binding: { type: "env", name: "REGION" },
        },
        {
          id: "read_only",
          prompt: "Read only",
          type: "boolean",
          required: false,
          default: false,
          binding: { type: "env", name: "READ_ONLY" },
        },
      ],
    });
    render(<EntryDetail entry={withDefaults} />);
    const inputs = screen.getByRole("heading", { name: "Inputs" }).closest("section")!;
    expect(within(inputs).getByText("west")).toBeDefined();
    expect(within(inputs).getByText("false")).toBeDefined();
    expect(within(inputs).getByText("Secret")).toBeDefined();
    expect(within(inputs).getByText("Boolean")).toBeDefined();
    expect(screen.getByText(installCommand(entry))).toBeDefined();
    expect(screen.getByText(marketplaceSearchCommand(entry))).toBeDefined();
    expect(screen.getByText(entry.inputs[0].prompt)).toBeDefined();
    expect(screen.queryByText(/compozy mcp install/)).toBeNull();
  });

  it("Should enumerate current detail routes and reject unknown identities", async () => {
    expect(generateMarketplaceEntryParams()).toEqual(
      extensionEntries.map(entry => ({ entryId: entry.entry_id }))
    );
    const element = await MarketplaceEntryPage({
      params: Promise.resolve({ entryId: extensionEntries[0].entry_id }),
    });
    const detail = render(element);
    expect(detail.getByRole("heading", { level: 1, name: extensionEntries[0].name })).toBeDefined();
    await expect(
      MarketplaceEntryPage({ params: Promise.resolve({ entryId: "not-in-catalog" }) })
    ).rejects.toThrow();
  });
});
