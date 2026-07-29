import { describe, expect, it } from "vitest";
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
