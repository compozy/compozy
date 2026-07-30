import {
  existsSync,
  mkdirSync,
  mkdtempSync,
  readdirSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
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
import { MarketplaceHero } from "@/components/marketplace/marketplace-hero";
import MarketplaceEntryPage, {
  generateStaticParams as generateMarketplaceEntryParams,
} from "@/app/marketplace/[kind]/[entryId]/page";
import MarketplaceKindPage, {
  generateStaticParams as generateMarketplaceKindParams,
} from "@/app/marketplace/[kind]/page";
import { BRIDGE_LOGOS } from "../marketplace-bridge-logos";
import { bridgeProviders, findBridgeProvider, readBridgeProviders } from "../marketplace-bridges";
import {
  bundledSkills,
  devCycleExtension,
  parseBundledSkillFrontmatter,
} from "../marketplace-bundled";
import {
  entriesForKind,
  extensionEntries,
  extensionEntrySchema,
  findEntry,
  installCommand,
  isMarketplaceKind,
  MARKETPLACE_KINDS,
  mcpEntries,
  mcpEntrySchema,
  marketplaceSearchCommand,
  parseMarketplaceCatalog,
  skillEntries,
  skillEntrySchema,
} from "../marketplace-catalog";

function validMCPEntry(overrides: Record<string, unknown> = {}) {
  return {
    entry_id: "acme-mcp",
    name: "Acme MCP",
    description: "A validated MCP server",
    launch: { type: "npm", package: "acme-mcp", version: "1.2.3" },
    default_scope: "workspace",
    ...overrides,
  };
}

describe("marketplace catalog", () => {
  it("parses every real feed at build time with at least one entry per kind", () => {
    expect(skillEntries.length).toBeGreaterThan(0);
    expect(extensionEntries.length).toBeGreaterThan(0);
    expect(mcpEntries.length).toBeGreaterThan(0);
  });

  it("exposes exactly the three daemon catalog kinds (D9 — no bundles)", () => {
    expect([...MARKETPLACE_KINDS]).toEqual(["skills", "extensions", "mcp"]);
    expect(isMarketplaceKind("bundles")).toBe(false);
    expect(isMarketplaceKind("mcp")).toBe(true);
  });

  it("derives the CLI install command per kind from real feed fields", () => {
    const skill = skillEntries[0];
    const extension = extensionEntries[0];
    const mcp = mcpEntries[0];

    expect(installCommand("skills", skill)).toBe(`compozy skill install ${skill.install_slug}`);
    expect(installCommand("extensions", extension)).toBe(
      `compozy extension install ${extension.install_slug}`
    );
    expect(installCommand("mcp", mcp)).toBe(`compozy mcp install ${mcp.entry_id}`);
  });

  it("resolves entries by kind and id for detail routes", () => {
    for (const kind of MARKETPLACE_KINDS) {
      for (const entry of entriesForKind(kind)) {
        expect(findEntry(kind, entry.entry_id)).toBe(entry);
      }
    }
    expect(findEntry("skills", "does-not-exist")).toBeUndefined();
  });

  it("enumerates and renders every public Marketplace kind route", async () => {
    expect(generateMarketplaceKindParams()).toEqual(MARKETPLACE_KINDS.map(kind => ({ kind })));

    render(
      await MarketplaceKindPage({
        params: Promise.resolve({ kind: "skills" }),
      })
    );
    expect(screen.getByRole("heading", { level: 1, name: "All skills" })).toBeTruthy();

    await expect(
      MarketplaceKindPage({ params: Promise.resolve({ kind: "unknown" }) })
    ).rejects.toThrow("NEXT_HTTP_ERROR_FALLBACK;404");
  });

  it("enumerates known detail routes and rejects unknown catalog identities", async () => {
    const expectedParams = MARKETPLACE_KINDS.flatMap(kind =>
      entriesForKind(kind).map(entry => ({ kind, entryId: entry.entry_id }))
    );
    expect(generateMarketplaceEntryParams()).toEqual(expectedParams);

    const entry = skillEntries[0];
    if (!entry) {
      throw new Error("skills catalog fixture must not be empty");
    }
    render(
      await MarketplaceEntryPage({
        params: Promise.resolve({ kind: "skills", entryId: entry.entry_id }),
      })
    );
    expect(screen.getByRole("heading", { level: 1, name: entry.name })).toBeTruthy();

    await expect(
      MarketplaceEntryPage({
        params: Promise.resolve({ kind: "skills", entryId: "does-not-exist" }),
      })
    ).rejects.toThrow("NEXT_HTTP_ERROR_FALLBACK;404");
  });

  it("rejects feed drift the daemon would reject", () => {
    expect(() =>
      skillEntrySchema.parse({
        entry_id: "x",
        name: "X",
        description: "d",
        // install_slug missing
      })
    ).toThrow(/install_slug/);

    expect(() =>
      extensionEntrySchema.parse({
        entry_id: "x",
        name: "X",
        description: "d",
        version: "1.0.0",
        install_slug: "org/x",
        artifact_url: "http://insecure.example/artifact.tar.gz",
        digest_sha256: "a".repeat(64),
        tier: "official",
      })
    ).toThrow(/HTTPS/);

    expect(() =>
      extensionEntrySchema.parse({
        entry_id: "x",
        name: "X",
        description: "d",
        version: "1.0.0",
        install_slug: "org/x",
        artifact_url: "https://example.com/artifact.tar.gz",
        digest_sha256: "not-a-digest",
        tier: "official",
      })
    ).toThrow(/hex/);

    expect(() =>
      mcpEntrySchema.parse({
        entry_id: "x",
        name: "X",
        description: "d",
        default_scope: "workspace",
        // launch missing
      })
    ).toThrow(/launch/);

    expect(() =>
      mcpEntrySchema.parse({
        entry_id: "x",
        name: "X",
        description: "d",
        launch: { type: "remote", url: "https://example.com/mcp" },
        default_scope: "workspace",
        unknown_field: true,
      })
    ).toThrow(/unrecognized|unknown_field/);
  });

  it("omits unsupported trust fields from parsed catalog entries", () => {
    // Truthfulness (§7.3): ratings/downloads/featured have no source fields and strict schemas reject them.
    const allEntries = MARKETPLACE_KINDS.flatMap(kind => entriesForKind(kind));
    for (const entry of allEntries) {
      expect(entry).not.toHaveProperty("rating");
      expect(entry).not.toHaveProperty("downloads");
      expect(entry).not.toHaveProperty("featured");
    }
  });

  it("requires the generated timestamp required by the daemon catalog decoder", () => {
    expect(() =>
      parseMarketplaceCatalog("skills", {
        manifest_version: 2,
        entries: [
          {
            entry_id: "writer",
            name: "Writer",
            description: "Writes documentation",
            install_slug: "writer",
          },
        ],
      })
    ).toThrow(/generated_at/);
  });

  it("rejects a skill identifier reserved for registry-only entries", () => {
    expect(() =>
      parseMarketplaceCatalog("skills", {
        manifest_version: 2,
        generated_at: "2026-07-17T00:40:00Z",
        entries: [
          {
            entry_id: "skill_writer",
            name: "Writer",
            description: "Writes documentation",
            install_slug: "writer",
          },
        ],
      })
    ).toThrow(/reserved/);
  });

  it("rejects duplicate identifiers before publishing a catalog feed", () => {
    expect(() =>
      parseMarketplaceCatalog("skills", {
        manifest_version: 2,
        generated_at: "2026-07-17T00:40:00Z",
        entries: [
          {
            entry_id: "writer",
            name: "Writer",
            description: "Writes documentation",
            install_slug: "writer",
          },
          {
            entry_id: "writer",
            name: "Other Writer",
            description: "Also writes documentation",
            install_slug: "other-writer",
          },
        ],
      })
    ).toThrow(/duplicated/);
  });

  it("rejects duplicate install slugs before publishing a catalog feed", () => {
    expect(() =>
      parseMarketplaceCatalog("skills", {
        manifest_version: 2,
        generated_at: "2026-07-17T00:40:00Z",
        entries: [
          {
            entry_id: "writer",
            name: "Writer",
            description: "Writes documentation",
            install_slug: "writer",
          },
          {
            entry_id: "editor",
            name: "Editor",
            description: "Edits documentation",
            install_slug: "writer",
          },
        ],
      })
    ).toThrow(/install_slug is duplicated/);
  });

  it("rejects secret MCP input defaults that the daemon would never persist", () => {
    expect(() =>
      parseMarketplaceCatalog("mcp", {
        manifest_version: 2,
        generated_at: "2026-07-17T00:40:00Z",
        entries: [
          {
            entry_id: "private-mcp",
            name: "Private MCP",
            description: "Uses a secret",
            launch: { type: "npm", package: "private-mcp", version: "1.0.0" },
            default_scope: "workspace",
            inputs: [
              {
                id: "private_token",
                prompt: "Private token",
                type: "secret",
                required: true,
                default: "must-not-publish",
                binding: { type: "env", name: "PRIVATE_TOKEN" },
              },
            ],
          },
        ],
      })
    ).toThrow(/must not set default/);
  });

  it("rejects OAuth on a local MCP launch", () => {
    expect(() =>
      parseMarketplaceCatalog("mcp", {
        manifest_version: 2,
        generated_at: "2026-07-17T00:40:00Z",
        entries: [
          {
            entry_id: "remote-mcp",
            name: "Remote MCP",
            description: "Uses OAuth",
            launch: { type: "npm", package: "local-mcp", version: "1.0.0" },
            default_scope: "workspace",
            auth: { method: "oauth", registration: "auto" },
          },
        ],
      })
    ).toThrow(/only allowed for remote/);
  });

  it.each([
    [
      "option-like package names",
      { type: "npm", package: "-y", version: "1.2.3" },
      /exact package name/,
    ],
    [
      "tagged Docker images",
      {
        type: "docker",
        image: "ghcr.io/acme/server:latest",
        digest: `sha256:${"a".repeat(64)}`,
      },
      /exact untagged image name/,
    ],
    [
      "uppercase Docker digests",
      {
        type: "docker",
        image: "ghcr.io/acme/server",
        digest: `sha256:${"A".repeat(64)}`,
      },
      /verified sha256 digest/,
    ],
    [
      "NUL bytes in launch arguments",
      { type: "uvx", package: "acme-mcp", version: "1.2.3", args: ["--mode\0unsafe"] },
      /non-empty argument/,
    ],
  ])("rejects %s that the daemon launch decoder rejects", (_name, launch, message) => {
    expect(() => mcpEntrySchema.parse(validMCPEntry({ launch }))).toThrow(message);
  });

  it.each([
    "https://mcp.localhost/server",
    "https://mcp.internal/server",
    "https://mcp/server",
    "https://127.0.0.1/server",
    "https://10.0.0.8/server",
    "https://169.254.169.254/server",
    "https://192.0.2.1/server",
    "https://100.64.0.1/server",
    "https://[::1]/server",
    "https://example.com/server#credentials",
  ])("rejects non-public MCP remote destination %s", url => {
    expect(() => mcpEntrySchema.parse(validMCPEntry({ launch: { type: "remote", url } }))).toThrow(
      /public HTTPS destination/
    );
  });

  it("rejects MCP inputs that target the same binding", () => {
    expect(() =>
      mcpEntrySchema.parse(
        validMCPEntry({
          inputs: [
            {
              id: "first",
              prompt: "First value",
              type: "string",
              required: false,
              binding: { type: "env", name: "SERVER_MODE" },
            },
            {
              id: "second",
              prompt: "Second value",
              type: "string",
              required: false,
              binding: { type: "env", name: "SERVER_MODE" },
            },
          ],
        })
      )
    ).toThrow(/binding env\/SERVER_MODE is duplicated/);
  });

  it("rejects MCP query inputs that override launch configuration", () => {
    expect(() =>
      mcpEntrySchema.parse(
        validMCPEntry({
          launch: { type: "remote", url: "https://mcp.example.com/server?read_only=true" },
          inputs: [
            {
              id: "read_only",
              prompt: "Read only",
              type: "boolean",
              required: false,
              binding: { type: "url_query", name: "read_only" },
            },
          ],
        })
      )
    ).toThrow(/conflicts with launch URL/);
  });

  it("reports MCP string and identifier default violations accurately", () => {
    const identifierInput = {
      id: "profile",
      prompt: "Profile",
      type: "identifier",
      required: false,
      binding: { type: "env", name: "PROFILE" },
    };

    expect(() =>
      mcpEntrySchema.parse(validMCPEntry({ inputs: [{ ...identifierInput, default: "  " }] }))
    ).toThrow(/identifier defaults must be non-empty strings/);
    expect(() =>
      mcpEntrySchema.parse(
        validMCPEntry({ inputs: [{ ...identifierInput, type: "string", default: false }] })
      )
    ).toThrow(/string defaults must be strings/);
  });
});

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..", "..", "..");

function bridgeManifests() {
  const bridgesRoot = resolve(repoRoot, "extensions", "bridges");
  const manifests: Array<{
    platform: string;
    displayName: string;
    version: string;
    description: string;
    requiredSecrets: number;
    totalSecrets: number;
  }> = [];
  for (const entry of readdirSync(bridgesRoot, { withFileTypes: true })) {
    if (!entry.isDirectory()) continue;
    const manifestPath = resolve(bridgesRoot, entry.name, "extension.toml");
    if (!existsSync(manifestPath)) continue;
    const manifest = parseToml(readFileSync(manifestPath, "utf8")) as {
      bridge: {
        platform: string;
        display_name: string;
        secret_slots: Array<{ required: boolean }>;
      };
      extension: { version: string; description: string };
    };
    manifests.push({
      platform: manifest.bridge.platform,
      displayName: manifest.bridge.display_name,
      version: manifest.extension.version,
      description: manifest.extension.description,
      requiredSecrets: manifest.bridge.secret_slots.filter(slot => slot.required).length,
      totalSecrets: manifest.bridge.secret_slots.length,
    });
  }
  return manifests;
}

function manifestDirectories(root: string, directories: string[]): string[] {
  return directories
    .flatMap(directory =>
      readdirSync(resolve(root, directory), { withFileTypes: true })
        .filter(entry => entry.isDirectory())
        .map(entry => entry.name)
    )
    .sort();
}

function writeBridgeManifest(root: string, directory: string, platform: string): void {
  const manifestRoot = resolve(root, directory);
  mkdirSync(manifestRoot, { recursive: true });
  writeFileSync(
    resolve(manifestRoot, "extension.toml"),
    `[extension]
name = "${directory}"
version = "0.1.0"
description = "${directory} bridge"

[capabilities]
provides = ["bridge.adapter"]

[bridge]
platform = "${platform}"
display_name = "${directory}"

[[bridge.secret_slots]]
name = "token"
description = "Bridge token"
required = true
`
  );
}

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

  it("keeps bridges out of the catalog feed kinds", () => {
    // Bridges cannot be feed entries: each manifest points [subprocess] at a locally built binary,
    // so there is no cross-platform artifact to publish with a digest.
    expect(isMarketplaceKind("bridges")).toBe(false);
    for (const provider of bridgeProviders) {
      expect(findEntry("extensions", provider.platform)).toBeUndefined();
    }
  });
});

describe("marketplace bundled resources", () => {
  it("derives the dev-cycle inventory from its manifest and directories", () => {
    const devCycleRoot = resolve(repoRoot, "extensions", "dev-cycle");
    const manifest = JSON.parse(readFileSync(resolve(devCycleRoot, "extension.json"), "utf8")) as {
      extension: {
        name: string;
        version: string;
        description: string;
        min_compozy_version: string;
      };
      capabilities: { provides: string[] };
      resources: {
        loops: string[];
        skills: string[];
        agents: string[];
        tools: Record<string, unknown>;
      };
    };
    const loopDirectories = manifest.resources.loops.flatMap(parent =>
      readdirSync(resolve(devCycleRoot, parent), { withFileTypes: true })
        .filter(entry => entry.isDirectory())
        .map(entry => ({ parent, name: entry.name }))
    );
    const loops = loopDirectories.map(({ parent, name }) => {
      const loop = parseYaml(
        readFileSync(resolve(devCycleRoot, parent, name, "loop.yaml"), "utf8")
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

    expect(devCycleExtension).toMatchObject({
      name: manifest.extension.name,
      version: manifest.extension.version,
      description: manifest.extension.description,
      minCompozyVersion: manifest.extension.min_compozy_version,
      provides: manifest.capabilities.provides,
      loops,
      skills: manifestDirectories(devCycleRoot, manifest.resources.skills),
      agents: manifestDirectories(devCycleRoot, manifest.resources.agents),
    });
    expect(devCycleExtension.tools).toHaveLength(Object.keys(manifest.resources.tools).length);
  });

  it("offers inspection instead of an install command for bundled resources", () => {
    // dev-cycle is enrolled from the binary at first boot (SourceBundled), so an install command
    // would be false and a feed entry would collide with that managed install.
    expect(devCycleExtension.statusCommand).toBe(
      `compozy extension status ${devCycleExtension.name}`
    );
    expect(findEntry("extensions", devCycleExtension.name)).toBeUndefined();
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
  it("renders feed dates truthfully without invented trust signals", () => {
    const source = skillEntries[0];
    if (!source) throw new Error("the checked-in skill feed must not be empty");
    const entry = {
      ...source,
      published_at: "2026-07-17T00:40:00Z",
      updated_at: undefined,
    };

    const card = render(<MarketplaceEntryCard kind="skills" entry={entry} />);
    expect(card.getByText(/^Published /)).toBeDefined();
    expect(card.container.textContent).not.toMatch(/\b(rating|downloads|featured)\b/i);
    card.unmount();

    const detail = render(<MarketplaceEntryDetail kind="skills" entry={entry} />);
    expect(detail.getByText("Published")).toBeDefined();
    expect(detail.getByText("Jul 17, 2026")).toBeDefined();
    expect(detail.container.textContent).not.toMatch(/\b(rating|downloads|featured)\b/i);
  });

  it("directs a static listing to search the daemon's active catalog", () => {
    render(<MarketplaceHero />);

    expect(screen.getAllByText("compozy marketplace search").length).toBeGreaterThan(0);
    expect(screen.getByText(/checked-in catalog snapshot/)).toBeDefined();
  });

  it("pairs MCP input flags with the install command that owns them", () => {
    const entry = mcpEntries.find(candidate =>
      candidate.inputs?.some(input => input.type === "secret")
    );
    if (!entry) throw new Error("the checked-in MCP feed requires a secret-bearing entry");

    render(<MarketplaceEntryDetail kind="mcp" entry={entry} />);

    expect(screen.getByText(installCommand("mcp", entry))).toBeDefined();
    expect(screen.getByText("--secret id")).toBeDefined();
    expect(screen.getByText("--set id=value")).toBeDefined();
    expect(screen.getByText(marketplaceSearchCommand("mcp", entry))).toBeDefined();
  });
});
