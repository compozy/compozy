import {
  existsSync,
  mkdirSync,
  mkdtempSync,
  readdirSync,
  statSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { basename, dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { render, screen } from "@testing-library/react";
import { parse as parseToml } from "smol-toml";
import { parse as parseYaml } from "yaml";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/link", () => ({
  default: ({ href, children }: { href: string; children: React.ReactNode }) => (
    <a href={href}>{children}</a>
  ),
}));

vi.mock("@/lib/source", () => ({
  docsSource: {
    getPage: ([section, slug]: string[]) =>
      section === "bridges" && slug?.startsWith("setup-")
        ? { url: `/docs/${section}/${slug}` }
        : undefined,
  },
}));

import { MarketplaceEntryDetail } from "@/components/marketplace/marketplace-entry-detail";
import { MarketplaceEntryCard } from "@/components/marketplace/marketplace-entry-card";
import { MarketplaceBundledSection } from "@/components/marketplace/marketplace-bundled-section";
import BundledExtensionPage, {
  generateStaticParams as generateBundledExtensionParams,
} from "@/app/marketplace/bundled/[name]/page";
import { MarketplaceHero } from "@/components/marketplace/marketplace-hero";
import MarketplaceEntryPage, {
  generateStaticParams as generateMarketplaceEntryParams,
} from "@/app/marketplace/[entryId]/page";
import { BRIDGE_LOGOS } from "../marketplace-bridge-logos";
import { bridgeProviders, findBridgeProvider, readBridgeProviders } from "../marketplace-bridges";
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

// Invariant: only the current extension feed reaches rendering, with validated identity, inputs and icons.
// Owner: site catalog boundary. Canonical suite: marketplace-catalog.test.tsx.
describe("marketplace catalog", () => {
  const feed = () => ({
    manifest_version: 3,
    generated_at: "2026-09-12T10:00:00Z",
    entries: extensionEntries,
  });

  it("Should load all twenty current packages without standalone skill entries", () => {
    expect(parseMarketplaceCatalog(feed())).toEqual(extensionEntries);
    expect(extensionEntries).toHaveLength(20);
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
    expect(installCommand(findEntry("batuta")!)).toBe(
      "compozy extension install franciscpd/batuta-compozy"
    );
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
});

describe("marketplace bridge providers", () => {
  it("derives one provider per in-tree bridge manifest", () => {
    const manifests = bridgeManifests();

    expect(bridgeProviders).toHaveLength(manifests.length);
    for (const manifest of manifests) {
      expect(findBridgeProvider(manifest.platform)).toMatchObject({
        platform: manifest.platform,
        displayName: manifest.displayName,
        version: manifest.version,
        description: manifest.description,
        secretSlots: { required: manifest.requiredSecrets, total: manifest.totalSecrets },
        setupUrl: `/docs/bridges/setup-${manifest.platform}`,
      });
    }
  });

  it("keeps each manifest-derived secret-slot count internally consistent", () => {
    for (const provider of bridgeProviders) {
      expect(provider.secretSlots.total).toBeGreaterThan(0);
      expect(provider.secretSlots.required).toBeGreaterThan(0);
      expect(provider.secretSlots.required).toBeLessThanOrEqual(provider.secretSlots.total);
      expect(provider.setupUrl).toBe(`/docs/bridges/setup-${provider.platform}`);
    }
  });

  it("rejects a provider whose setup guide is missing from the docs source", () => {
    expect(() => readBridgeProviders({ resolveSetupPage: () => undefined })).toThrow(
      /setup guide is missing/
    );
  });

  it("rejects duplicate bridge platforms before publishing providers", () => {
    const root = mkdtempSync(join(tmpdir(), "compozy-bridge-providers-"));
    try {
      writeBridgeManifest(root, "first", "duplicate");
      writeBridgeManifest(root, "second", "duplicate");

      expect(() =>
        readBridgeProviders({
          root,
          resolveSetupPage: ([, slug]) => ({ url: `/docs/bridges/${slug}` }),
        })
      ).toThrow(/duplicate bridge platform/);
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
  });

  it("has a real platform mark for every provider", () => {
    // Tiles render the `@compozy/ui` logo inventory, the same marks the landing page uses. A
    // provider without one would silently fall back to a neutral glyph.
    const marks = new Set(Object.keys(BRIDGE_LOGOS));
    for (const provider of bridgeProviders) {
      expect(marks.has(provider.platform)).toBe(true);
    }
  });

  it("Should keep bridge setup independent of packaged MCP servers with the same brand", () => {
    // Task02 packages the GitHub/Linear MCP servers. A shared brand is not bridge identity:
    // bridges retain setup guides, while the distinct extension packages carry install artifacts.
    for (const platform of ["github", "linear"]) {
      const bridge = findBridgeProvider(platform);
      const packaged = findEntry(platform);
      expect(bridge?.setupUrl).toBe(`/docs/bridges/setup-${platform}`);
      expect(packaged).toMatchObject({
        entry_id: platform,
        install_slug: `compozy/${platform}`,
        repository: `https://github.com/compozy/compozy/tree/main/catalog/packages/${platform}`,
      });
      expect(packaged?.description).not.toBe(bridge?.description);
    }
  });
});

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
          loops: ResourcePath[];
          skills: ResourcePath[];
          agents: ResourcePath[];
          tools: Record<string, unknown>;
        };
      };
      const loopDirectories = manifest.resources.loops.flatMap(({ path: parent }) =>
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
      expect(extension.tools).toHaveLength(Object.keys(manifest.resources.tools).length);
    }
  );

  it("offers inspection and a detail page for each bundled extension", async () => {
    render(<MarketplaceBundledSection />);
    expect(generateBundledExtensionParams()).toEqual([
      { name: "spec-cycle" },
      { name: "open-design" },
    ]);
    for (const extension of bundledExtensions) {
      expect(
        screen.getByRole("link", { name: new RegExp(extension.displayName) }).getAttribute("href")
      ).toBe(extension.path);
      expect(extension.statusCommand).toBe(`compozy extension status ${extension.name}`);
      expect(findEntry("extensions", extension.name)).toBeUndefined();
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
  it("Should render truthful dates and current detail links without invented trust signals", () => {
    const entry = {
      ...extensionEntries[0],
      published_at: "2026-07-17T00:40:00Z",
      updated_at: undefined,
    };
    const card = render(<MarketplaceEntryCard entry={entry} />);
    expect(card.getByText(/^Published /)).toBeDefined();
    expect(card.getByRole("link", { name: "View details" }).getAttribute("href")).toBe(
      marketplaceEntryPath(entry)
    );
    expect(card.container.textContent).not.toMatch(/\b(rating|downloads|featured)\b/i);
    card.unmount();
    const detail = render(<MarketplaceEntryDetail entry={entry} />);
    expect(detail.getAllByText(/Jul 17, 2026/).length).toBeGreaterThan(0);
    expect(detail.container.textContent).not.toMatch(/\b(rating|downloads|featured)\b/i);
  });

  it("Should direct the static listing to search the active daemon catalog", () => {
    render(<MarketplaceHero />);
    expect(screen.getAllByText("compozy marketplace search").length).toBeGreaterThan(0);
    expect(screen.getByText(/checked-in catalog snapshot/)).toBeDefined();
  });

  it("Should show declared extension inputs with the owning install command", () => {
    const entry = findEntry("context7");
    if (!entry?.inputs?.length) throw new Error("Context7 fixture lacks its declared input");
    render(<MarketplaceEntryDetail entry={entry} />);
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
