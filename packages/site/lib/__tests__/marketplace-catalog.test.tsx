import { readdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { BRIDGE_LOGOS } from "../marketplace-bridge-logos";
import { bridgeProviders, findBridgeProvider } from "../marketplace-bridges";
import { bundledSkills, devCycleExtension } from "../marketplace-bundled";
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
  skillEntries,
  skillEntrySchema,
} from "../marketplace-catalog";

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

  it("rejects feed drift the daemon would reject", () => {
    expect(() =>
      skillEntrySchema.parse({
        entry_id: "x",
        name: "X",
        description: "d",
        // install_slug missing
      })
    ).toThrow();

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
        transport: "stdio",
        // command missing for stdio
      })
    ).toThrow(/command/);

    expect(() =>
      mcpEntrySchema.parse({
        entry_id: "x",
        name: "X",
        description: "d",
        transport: "http",
        url: "https://example.com/mcp",
        unknown_field: true,
      })
    ).toThrow();
  });

  it("never renders trust fields that do not exist in the feeds", () => {
    // Truthfulness (§7.3): ratings/downloads/featured have no source fields; the schemas are strict,
    // so their absence here proves they cannot reach the page layer.
    const allEntries = MARKETPLACE_KINDS.flatMap(kind => entriesForKind(kind));
    for (const entry of allEntries) {
      expect(entry).not.toHaveProperty("rating");
      expect(entry).not.toHaveProperty("downloads");
      expect(entry).not.toHaveProperty("featured");
    }
  });
});

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..", "..", "..");

describe("marketplace bridge providers", () => {
  it("derives one provider per in-tree bridge manifest", () => {
    const manifestDirectories = readdirSync(resolve(repoRoot, "extensions", "bridges"), {
      withFileTypes: true,
    })
      .filter(entry => entry.isDirectory())
      .map(entry => entry.name);

    expect(bridgeProviders).toHaveLength(manifestDirectories.length);
    for (const directory of manifestDirectories) {
      expect(findBridgeProvider(directory)).toBeDefined();
    }
  });

  it("reads secret-slot counts and setup guides from the manifests", () => {
    for (const provider of bridgeProviders) {
      expect(provider.secretSlots.total).toBeGreaterThan(0);
      expect(provider.secretSlots.required).toBeGreaterThan(0);
      expect(provider.secretSlots.required).toBeLessThanOrEqual(provider.secretSlots.total);
      expect(provider.setupUrl).toBe(`/docs/bridges/setup-${provider.platform}`);
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
    expect(devCycleExtension.name).toBe("dev-cycle");
    expect(devCycleExtension.loops.map(loop => loop.name)).toEqual([
      "review-and-fix",
      "software-delivery",
    ]);
    expect(devCycleExtension.skills.length).toBeGreaterThan(0);
    expect(devCycleExtension.agents.length).toBeGreaterThan(0);
    expect(devCycleExtension.tools.length).toBeGreaterThan(0);
    for (const loop of devCycleExtension.loops) {
      expect(loop.description.length).toBeGreaterThan(0);
      expect(loop.useWhen?.length ?? 0).toBeGreaterThan(0);
    }
  });

  it("offers inspection instead of an install command for bundled resources", () => {
    // dev-cycle is enrolled from the binary at first boot (SourceBundled), so an install command
    // would be false and a feed entry would collide with that managed install.
    expect(devCycleExtension.statusCommand).toBe("compozy extension status dev-cycle");
    expect(findEntry("extensions", "dev-cycle")).toBeUndefined();
    expect(extensionEntries.map(entry => entry.install_slug)).not.toContain("compozy/dev-cycle");
    expect(skillEntries.map(entry => entry.install_slug)).not.toContain("cy-execute-task");
  });

  it("reads every bundled skill's identity from its SKILL.md", () => {
    expect(bundledSkills.length).toBeGreaterThan(0);
    for (const skill of bundledSkills) {
      expect(skill.name.length).toBeGreaterThan(0);
      expect(skill.description.length).toBeGreaterThan(0);
      expect(skill.repositoryUrl).toContain(`/skills/${skill.name}`);
    }
  });
});
