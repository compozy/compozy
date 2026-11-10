import { existsSync, readdirSync, statSync, readFileSync } from "node:fs";
import { basename, dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { parse as parseYaml } from "yaml";
import { useState } from "react";
import { renderToString } from "react-dom/server";
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

/** The marketplace browser reads `?category=`; tests drive it through this value. */
const urlSearch = vi.hoisted(() => ({ current: new URLSearchParams() }));
vi.mock("next/navigation", async importOriginal => ({
  ...(await importOriginal<typeof import("next/navigation")>()),
  useSearchParams: () => urlSearch.current,
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
import {
  MarketplaceBrowse,
  MarketplaceBrowseFromUrl,
} from "@/components/marketplace/marketplace-browse";
import BundledExtensionPage, {
  generateStaticParams as generateBundledExtensionParams,
} from "@/app/marketplace/bundled/[name]/page";
import MarketplaceEntryPage, {
  generateStaticParams as generateMarketplaceEntryParams,
} from "@/app/marketplace/[entryId]/page";
import presetsFeed from "../../../../catalog/v3/marketplaces.json";
import { bundledExtensions, parseBundledSkillFrontmatter } from "../marketplace-bundled";
import {
  extensionEntries,
  findEntry,
  marketplaceEntryPath,
  installCommand,
  marketplaceSearchCommand,
} from "../marketplace-catalog";
import {
  extensionEntrySchema,
  extensionFeedSchema,
  marketplacePresetsSchema,
} from "../marketplace-catalog-schema";
import {
  isCategoryId,
  listedPlugins,
  marketplacePlugins,
  pluginDisplayName,
  pluginEntryIds,
  pluginSnapshot,
  resolveCategory,
} from "../marketplace-plugins";
import { svgHints } from "@/scripts/marketplace-icon-hints";
import { pluginSnapshotSchema } from "../marketplace-plugins-schema";
import {
  categoriesWithEntries,
  marketplaceCards,
  matchesCard,
  relatedCards,
  sortCards,
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
    expect(extensionFeedSchema.parse(feed()).entries).toEqual(extensionEntries);
    expect(extensionEntries).toHaveLength(18);
    expect(findEntry("context7")?.inputs).toEqual(
      expect.arrayContaining([expect.objectContaining({ id: "context7_api_key", type: "secret" })])
    );
    expect(findEntry("documentation-writer")).toBeUndefined();
  });

  it.each([undefined, 1, 2, 4])("Should reject unsupported feed version %s", manifest_version => {
    expect(() => extensionFeedSchema.parse({ ...feed(), manifest_version })).toThrow(
      /manifest_version/
    );
  });

  it("Should reject missing timestamps and duplicate catalog identities", () => {
    expect(() => extensionFeedSchema.parse({ ...feed(), generated_at: undefined })).toThrow(
      /generated_at/
    );
    const entry = extensionEntries[0];
    expect(() => extensionFeedSchema.parse({ ...feed(), entries: [entry, entry] })).toThrow(
      /duplicated/
    );
    expect(() =>
      extensionFeedSchema.parse({ ...feed(), entries: [entry, { ...entry, entry_id: "other" }] })
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
    // The plugin snapshot generator reads the shipped presets through this schema.
    const marketplacePresets = marketplacePresetsSchema.parse(presetsFeed).entries;
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
    for (const entry of readdirSync(skillRoot, { withFileTypes: true })) {
      if (!entry.isDirectory()) continue;
      const source = readFileSync(resolve(skillRoot, entry.name, "SKILL.md"), "utf8");
      const match = /^---\r?\n([\s\S]*?)\r?\n---(?:\r?\n|$)/.exec(source);
      if (!match?.[1]) throw new Error(`skills/${entry.name}/SKILL.md has no frontmatter`);
      const metadata = parseYaml(match[1]) as { name?: string; description?: string };
      expect(parseBundledSkillFrontmatter(source), entry.name).toEqual({
        name: metadata.name,
        description: metadata.description,
      });
    }
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

  // Invariant: the page view model lists every curated extension plus every plugin left after the
  // supersede rule, each in exactly one category, under unique ids, with only non-empty categories.
  // Owner: lib/marketplace-view. Canonical suite: this file.
  it("Should merge extensions and listed plugins into one card per entry", () => {
    expect(marketplaceCards).toHaveLength(extensionEntries.length + marketplacePlugins.length);
    const ids = marketplaceCards.map(card => card.id);
    expect(new Set(ids).size).toBe(ids.length);
    expect(new Set(marketplaceCards.map(card => card.href)).size).toBe(ids.length);
    for (const entry of extensionEntries) {
      expect(marketplaceCards.find(card => card.id === entry.entry_id)?.kind).toBe("extension");
    }
    const categories = categoriesWithEntries(marketplaceCards);
    expect(categories.every(category => category.count > 0)).toBe(true);
    expect(categories.reduce((sum, category) => sum + category.count, 0)).toBe(ids.length);
    expect(marketplaceCards.filter(card => card.featured).map(card => card.id)).toEqual(
      sortCards(marketplaceCards.filter(card => card.featured)).map(card => card.id)
    );
    expect(marketplaceCards.findIndex(card => !card.featured)).toBe(
      marketplaceCards.filter(card => card.featured).length
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
    expect(screen.getAllByRole("link")).toHaveLength(
      marketplaceCards.filter(card => matchesCard(card, "context7")).length
    );
    fireEvent.change(search, { target: { value: "no-such-package" } });
    expect(screen.queryAllByRole("link")).toHaveLength(0);
    expect(screen.getByText(/Nothing matches/)).toBeTruthy();
    fireEvent.change(search, { target: { value: "" } });
    expect(screen.getByRole("heading", { name: `All ${marketplaceCards.length}` })).toBeDefined();
    const nav = screen.getByRole("navigation", { name: "Categories" });
    expect(within(nav).getAllByRole("button")).toHaveLength(
      categoriesWithEntries(marketplaceCards).length + 1
    );
    fireEvent.click(within(nav).getByRole("button", { name: /Data & Analytics/ }));
    expect(onCategory).toHaveBeenCalledWith("data");
  }, 15_000);

  // Invariant: the rail count of a category equals the entries its view lists once fully loaded.
  it("Should list every entry of a category with the count the rail shows", () => {
    renderBrowser("data");
    const expected = marketplaceCards.filter(card => card.category === "data");
    const rail = screen.getByRole("navigation", { name: "Categories" });
    expect(within(rail).getByRole("button", { name: /Data & Analytics/ }).textContent).toContain(
      String(expected.length)
    );
    expect(
      screen.getByRole("heading", { name: `Data & Analytics ${expected.length}` })
    ).toBeDefined();
    let more = screen.queryByRole("button", { name: "Load more" });
    while (more) {
      fireEvent.click(more);
      more = screen.queryByRole("button", { name: "Load more" });
    }
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

  // Invariant: every catalog extension shows its install command, inputs or not.
  it("Should show the install command for an extension that declares no inputs", () => {
    const entry = extensionEntries.find(item => !item.inputs?.length);
    if (!entry) throw new Error("The catalog needs an extension without inputs for this case");
    render(<EntryDetail entry={entry} />);
    expect(screen.getByText(installCommand(entry))).toBeDefined();
    expect(screen.getByText(marketplaceSearchCommand(entry))).toBeDefined();
    expect(screen.queryByRole("heading", { name: "Inputs" })).toBeNull();
  });

  // Invariant: the selected category follows `?category=` in both directions, without navigating.
  it("Should keep the selected category in step with the URL", () => {
    const replaceState = vi.spyOn(window.history, "replaceState");
    const rail = () => screen.getByRole("navigation", { name: "Categories" });
    const current = () =>
      within(rail())
        .getAllByRole("button")
        .find(button => button.getAttribute("aria-current") === "true")?.textContent;
    urlSearch.current = new URLSearchParams("category=data");
    const view = render(<MarketplaceBrowseFromUrl entries={marketplaceCards} />);
    expect(current()).toMatch(/^Data & Analytics/);

    fireEvent.click(within(rail()).getByRole("button", { name: /^Design/ }));
    expect(String(replaceState.mock.lastCall?.[2])).toMatch(/\?category=design$/);

    urlSearch.current = new URLSearchParams();
    view.rerender(<MarketplaceBrowseFromUrl entries={marketplaceCards} />);
    expect(current()).toMatch(/^All/);
    replaceState.mockRestore();
    urlSearch.current = new URLSearchParams();
  });

  // Invariant: the hero logos and Featured, above the fold, are visible in the served HTML.
  it("Should serve the browse view without content hidden until hydration", () => {
    const html = renderToString(<MarketplaceBrowse entries={marketplaceCards} category={null} />);
    expect(html).toContain("Featured");
    expect(html).not.toMatch(/style="[^"]*opacity:0[;"]/);
  });

  // Invariant: a category pick scrolls to the catalog, once the render that shows the category lands.
  it("Should jump to the catalog after a category pick", () => {
    const scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView = scrollIntoView;
    const replaceState = vi.spyOn(window.history, "replaceState");
    urlSearch.current = new URLSearchParams();
    const view = render(<MarketplaceBrowseFromUrl entries={marketplaceCards} />);
    const rail = screen.getByRole("navigation", { name: "Categories" });

    fireEvent.click(within(rail).getByRole("button", { name: /^Design/ }));
    // The category reaches the view through the URL; jumping before would aim at the old layout.
    expect(scrollIntoView).not.toHaveBeenCalled();
    urlSearch.current = new URLSearchParams("category=design");
    view.rerender(<MarketplaceBrowseFromUrl entries={marketplaceCards} />);
    expect(scrollIntoView).toHaveBeenCalledOnce();
    expect(scrollIntoView.mock.contexts[0]).toBe(document.getElementById("catalog"));
    expect(scrollIntoView).toHaveBeenCalledWith({ behavior: "smooth", block: "start" });

    replaceState.mockRestore();
    Reflect.deleteProperty(Element.prototype, "scrollIntoView");
    urlSearch.current = new URLSearchParams();
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
    expect(generateMarketplaceEntryParams()).toEqual([
      ...extensionEntries.map(entry => ({ entryId: entry.entry_id })),
      ...marketplacePlugins.map(entry => ({ entryId: entry.id })),
    ]);
    const element = await MarketplaceEntryPage({
      params: Promise.resolve({ entryId: extensionEntries[0].entry_id }),
    });
    const detail = render(element);
    expect(detail.getByRole("heading", { level: 1, name: extensionEntries[0].name })).toBeDefined();
    detail.unmount();
    const plugin = marketplacePlugins.find(entry => entry.plugin.source === "compozy-plugins")!;
    const pluginPage = render(
      await MarketplaceEntryPage({ params: Promise.resolve({ entryId: plugin.id }) })
    );
    expect(
      pluginPage.getByRole("heading", { level: 1, name: pluginDisplayName(plugin.plugin) })
    ).toBeDefined();
    expect(pluginPage.getByText("Plugin")).toBeDefined();
    expect(pluginPage.getByText("Unverified")).toBeDefined();
    expect(pluginPage.getByText(`compozy marketplace search ${plugin.plugin.name}`)).toBeDefined();
    await expect(
      MarketplaceEntryPage({ params: Promise.resolve({ entryId: "not-in-catalog" }) })
    ).rejects.toThrow();
  });
});

// Invariant: the plugin snapshot is projected as the daemon projects preset marketplaces: category
// resolution, curated-only featured flags, superseded entries hidden, collision-free route ids.
// Owner: lib/marketplace-plugins. Canonical suite: marketplace-catalog.test.tsx.
describe("marketplace plugin snapshot", () => {
  const plugin = (fields: Partial<(typeof pluginSnapshot.plugins)[number]>) => ({
    source: "compozy-plugins",
    name: "example",
    ...fields,
  });

  it("Should parse the checked-in snapshot with every source's plugins accounted for", () => {
    expect(pluginSnapshotSchema.parse(pluginSnapshot)).toEqual(pluginSnapshot);
    expect(pluginSnapshot.sources.map(source => source.name)).toEqual([
      "claude-plugins-official",
      "compozy-plugins",
    ]);
    for (const source of pluginSnapshot.sources) {
      expect(pluginSnapshot.plugins.filter(item => item.source === source.name)).toHaveLength(
        source.plugins
      );
    }
    expect(
      pluginSnapshot.plugins.some(
        item => item.source !== "compozy-plugins" && (item.featured || item.compozy_category)
      )
    ).toBe(false);
    expect(() =>
      pluginSnapshotSchema.parse({
        ...pluginSnapshot,
        plugins: [pluginSnapshot.plugins[0], pluginSnapshot.plugins[0]],
      })
    ).toThrow(/duplicated/);
  });

  it.each([
    ["design", "productivity", "design"],
    [undefined, "Developer Tools", "development"],
    [undefined, "database", "data"],
    [undefined, "monitoring", "cloud"],
    [undefined, "location", "more"],
    [undefined, "unheard-of", "more"],
    ["invented", undefined, "more"],
  ])("Should resolve explicit %s / publisher %s to %s", (explicit, source, expected) => {
    expect(resolveCategory(explicit, source)).toBe(expected);
  });

  it("Should place every listed plugin in exactly one known category", () => {
    for (const entry of marketplacePlugins) {
      expect(isCategoryId(entry.category)).toBe(true);
      expect(marketplaceCards.filter(card => card.id === entry.id)).toHaveLength(1);
    }
  });

  it("Should hide every plugin a listed extension or curated plugin supersedes", () => {
    const listed = new Set(
      marketplacePlugins.map(entry => `${entry.plugin.source}/${entry.plugin.name}`)
    );
    for (const entry of extensionEntries) {
      for (const key of entry["x-compozy"]?.supersedes ?? []) expect(listed.has(key)).toBe(false);
    }
    for (const item of pluginSnapshot.plugins) {
      for (const key of item.supersedes ?? []) expect(listed.has(key)).toBe(false);
    }
    const kept = listedPlugins([
      plugin({ name: "kept" }),
      plugin({ name: "replacer", supersedes: ["claude-plugins-official/old"] }),
      plugin({ source: "claude-plugins-official", name: "old" }),
    ]);
    expect(kept.map(entry => entry.plugin.name)).toEqual(["kept", "replacer"]);
  });

  it.each([
    [{ name: "figma", display_name: "Figma" }, "Figma"],
    [{ name: "asana" }, "Asana"],
    [{ name: "aws-serverless" }, "AWS Serverless"],
    [{ name: "agent_sdk-dev" }, "Agent SDK Dev"],
    [{ name: "postHog-mcp" }, "Posthog MCP"],
    [{ name: "PagerDuty" }, "PagerDuty"],
  ])("Should show %o as %s", (fields, expected) => {
    expect(pluginDisplayName(fields)).toBe(expected);
  });

  it("Should keep raw slugs for routes and commands while search also matches the shown name", () => {
    const entry = marketplacePlugins.find(item => item.plugin.name === "aws-serverless");
    if (!entry) throw new Error("aws-serverless is expected in the current snapshot");
    const card = marketplaceCards.find(item => item.id === entry.id)!;
    expect(card.name).toBe(pluginDisplayName(entry.plugin));
    expect(card.href).toBe(`/marketplace/${entry.id}`);
    expect(matchesCard(card, "aws-serverless")).toBe(true);
    expect(matchesCard(card, card.name.toLowerCase())).toBe(true);
  });

  // Invariant: inside every category featured entries lead, then entries with an icon, so a
  // section's first cards are never monograms while logos remain.
  it("Should order featured entries, then entries with an icon, then the rest", () => {
    for (const category of categoriesWithEntries(marketplaceCards)) {
      const ranks = marketplaceCards
        .filter(card => card.category === category.id)
        .map(card => (card.featured ? 0 : card.icon ? 1 : 2));
      expect(ranks).toEqual(ranks.toSorted((a, b) => a - b));
    }
    const [plain, logo] = sortCards([
      { ...marketplaceCards.at(-1)!, id: "a", name: "A", icon: null, featured: false },
      { ...marketplaceCards.at(-1)!, id: "z", name: "Z", icon: "/x.png", featured: false },
    ]);
    expect([plain.id, logo.id]).toEqual(["z", "a"]);
  });

  it.each([
    ['<svg><path d="M0"/></svg>', true],
    ['<svg><path fill="#000"/><path fill="currentColor"/></svg>', true],
    ['<svg color="white"><path fill="currentColor"/></svg>', false],
    ["<svg><path fill='#005EEB'/><path fill='#7CFF41'/></svg>", false],
    ['<svg><path style="fill: #111111"/></svg>', true],
  ])("Should put SVG %s on the light tile: %s", (svg, light) => {
    expect(svgHints(svg)).toEqual({ bleed: false, light });
  });

  it("Should record tile hints only for plugins that have an icon", () => {
    for (const item of pluginSnapshot.plugins) {
      if (item.bleed || item.light) expect(item.icon).toBeDefined();
      expect(item.bleed && item.light).toBeFalsy();
    }
    for (const entry of extensionEntries) {
      if (entry.icon?.includes("/catalog/icons/")) {
        expect(pluginSnapshot.extension_icons[entry.entry_id]).toBeDefined();
      }
    }
  });

  it("Should keep extension ids and suffix only plugin names that collide", () => {
    const shared = plugin({ name: "shared" });
    const twin = plugin({ source: "claude-plugins-official", name: "shared" });
    const taken = plugin({ name: "context7" });
    const solo = plugin({ name: "solo" });
    const ids = pluginEntryIds([shared, twin, taken, solo], new Set(["context7"]));
    expect([...ids.values()]).toEqual([
      "shared~compozy-plugins",
      "shared~claude-plugins-official",
      "context7~compozy-plugins",
      "solo",
    ]);
  });
});
