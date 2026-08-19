// Suite: cmd-palette structural catalog cache
// Invariant: the persisted record carries STRUCTURE only — descriptors,
// sources, bindings and aliases, never client-resolved availability — is scoped
// to one canonical workspace id and contract version, and drops itself rather
// than serving a corrupt or stale-shaped entry.
// Boundary IN: the record projection and the IndexedDB read/write/drop wrapper.
// Boundary OUT: hydration policy, availability evaluation, and network reads.
import "fake-indexeddb/auto";

import { beforeEach, describe, expect, it } from "vitest";

import {
  dropCatalogRecord,
  readCatalogRecord,
  writeCatalogRecord,
} from "../cmd-palette-catalog-cache";
import {
  CMD_PALETTE_CATALOG_CONTRACT_VERSION,
  parseCatalogRecord,
  toCatalogRecord,
} from "../cmd-palette-catalog-record";
import type { CmdPaletteCatalogResponse } from "../cmd-palette-types";

const WORKSPACE = "ws-hq";

function servedCatalog(): CmdPaletteCatalogResponse {
  return {
    catalog_revision: "sha256:catalog-1",
    context_revision: "7",
    commands: [
      {
        id: "window.close",
        title: "Close window",
        section: "Window",
        icon: "x-square",
        source: "core",
        keywords: ["dismiss"],
        available: false,
        reason: "requires a focused window",
        bindings: ["meta+KeyW"],
        alias: null,
        destructive: false,
        availability_exempt: false,
        arguments: [],
        action: { kind: "client_op", op: "window.close" },
        execution: { retry_safe: false, single_flight: true },
        when: [{ key: "window.focused", value: true, reason: "requires a focused window" }],
      },
      {
        id: "shortcuts.cheatsheet",
        title: "Keyboard shortcuts",
        section: "Shell",
        icon: "keyboard",
        source: "core",
        available: true,
        bindings: ["meta+Slash"],
        alias: "keys",
        destructive: false,
        availability_exempt: true,
        arguments: [],
        action: { kind: "client_op", op: "shortcuts.cheatsheet" },
        execution: { retry_safe: true, single_flight: false },
      },
    ],
    sources: [{ source: "core", status: "healthy" }],
  };
}

async function clearStore(): Promise<void> {
  await dropCatalogRecord(WORKSPACE);
  await dropCatalogRecord("ws-other");
}

describe("cmd-palette catalog record", () => {
  it("Should keep structure and strip every client-resolved field", () => {
    const record = toCatalogRecord(WORKSPACE, servedCatalog());
    expect(record.catalogRevision).toBe("sha256:catalog-1");
    expect(record.commands).toHaveLength(2);
    for (const command of record.commands) {
      expect(command).not.toHaveProperty("available");
      expect(command).not.toHaveProperty("reason");
    }
    const persisted = JSON.stringify(record);
    for (const volatileField of ["rank_signals", "weights", "usage", "query_hits", "pins"]) {
      expect(persisted).not.toContain(`"${volatileField}"`);
    }
    const [closeWindow] = record.commands;
    // Structure the projection needs on a cold open survives verbatim.
    expect(closeWindow).toMatchObject({
      id: "window.close",
      bindings: ["meta+KeyW"],
      alias: null,
      availability_exempt: false,
      action: { kind: "client_op", op: "window.close" },
      when: [{ key: "window.focused", value: true, reason: "requires a focused window" }],
    });
    expect(record.sources).toEqual([{ source: "core", status: "healthy" }]);
  });

  it("Should reject a record that carries resolved availability", () => {
    const record = toCatalogRecord(WORKSPACE, servedCatalog());
    const poisoned = {
      ...record,
      commands: [{ ...record.commands[0], available: true, reason: "" }],
    };
    expect(parseCatalogRecord(poisoned)).toBeNull();
  });

  it("Should reject a record written under another contract version", () => {
    const record = toCatalogRecord(WORKSPACE, servedCatalog());
    expect(
      parseCatalogRecord({
        ...record,
        contractVersion: CMD_PALETTE_CATALOG_CONTRACT_VERSION + 1,
      })
    ).toBeNull();
  });
});

describe("cmd-palette catalog cache", () => {
  beforeEach(async () => {
    await clearStore();
  });

  it("Should round-trip the last-known catalog for a workspace", async () => {
    await writeCatalogRecord(toCatalogRecord(WORKSPACE, servedCatalog()));
    const stored = await readCatalogRecord(WORKSPACE);
    expect(stored?.catalogRevision).toBe("sha256:catalog-1");
    expect(stored?.commands.map(command => command.id)).toEqual([
      "window.close",
      "shortcuts.cheatsheet",
    ]);
  });

  it("Should keep one bounded record per workspace", async () => {
    await writeCatalogRecord(toCatalogRecord(WORKSPACE, servedCatalog()));
    await writeCatalogRecord(
      toCatalogRecord(WORKSPACE, { ...servedCatalog(), catalog_revision: "sha256:catalog-2" })
    );
    const stored = await readCatalogRecord(WORKSPACE);
    expect(stored?.catalogRevision).toBe("sha256:catalog-2");
  });

  it("Should never serve one workspace's catalog to another", async () => {
    await writeCatalogRecord(toCatalogRecord(WORKSPACE, servedCatalog()));
    expect(await readCatalogRecord("ws-other")).toBeNull();
  });

  it("Should drop a corrupt record and report a cold open", async () => {
    await writeCatalogRecord({
      ...toCatalogRecord(WORKSPACE, servedCatalog()),
      // A record that lost its revision is unusable: the projection could not
      // tell whether the daemon had moved past it.
      catalogRevision: "",
    });
    expect(await readCatalogRecord(WORKSPACE)).toBeNull();
    // The drop is durable — a second read does not resurrect it.
    expect(await readCatalogRecord(WORKSPACE)).toBeNull();
  });

  it("Should report a cold open when no record exists", async () => {
    expect(await readCatalogRecord(WORKSPACE)).toBeNull();
  });
});
