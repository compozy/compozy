import { z } from "zod";

import type {
  CmdPaletteCatalogResponse,
  CmdPaletteStructuralCatalog,
  CmdPaletteStructuralCommand,
} from "./cmd-palette-types";

/**
 * Bumped whenever the persisted shape stops matching what the daemon serves. A
 * record written under any other version is dropped rather than migrated —
 * greenfield alpha, and the daemon is one refetch away (BR-19).
 */
export const CMD_PALETTE_CATALOG_CONTRACT_VERSION = 1;

/**
 * Fields the record must never carry. Availability is resolved against *one*
 * client's volatile context, so replaying it would let one tab cold-hydrate
 * another's answer; rank signals are session-memory only (Key Decisions).
 */
const NON_STRUCTURAL_COMMAND_FIELDS = ["available", "reason"] as const;

const structuralCommandSchema = z
  .looseObject({
    id: z.string().min(1),
    title: z.string(),
    section: z.string(),
    icon: z.string(),
    source: z.string(),
    bindings: z.array(z.string()),
    alias: z.string().nullable(),
    destructive: z.boolean(),
    availability_exempt: z.boolean(),
    arguments: z.array(z.unknown()),
    action: z.looseObject({ kind: z.string() }),
    execution: z.looseObject({ retry_safe: z.boolean(), single_flight: z.boolean() }),
  })
  .refine(command => NON_STRUCTURAL_COMMAND_FIELDS.every(field => !(field in command)), {
    message: "persisted command carries client-resolved availability",
  });

const catalogRecordSchema = z.strictObject({
  contractVersion: z.literal(CMD_PALETTE_CATALOG_CONTRACT_VERSION),
  workspaceId: z.string().min(1),
  catalogRevision: z.string().min(1),
  commands: z.array(structuralCommandSchema),
  sources: z.array(
    z.looseObject({ source: z.string(), status: z.string(), reason: z.string().optional() })
  ),
});

export interface CmdPaletteCatalogRecord extends CmdPaletteStructuralCatalog {
  readonly contractVersion: typeof CMD_PALETTE_CATALOG_CONTRACT_VERSION;
  readonly workspaceId: string;
}

/** Strips the resolved half of a served catalog so only structure is persisted. */
export function toCatalogRecord(
  workspaceId: string,
  catalog: CmdPaletteCatalogResponse
): CmdPaletteCatalogRecord {
  const commands = catalog.commands.map(command => {
    const structural: Record<string, unknown> = { ...command };
    for (const field of NON_STRUCTURAL_COMMAND_FIELDS) delete structural[field];
    return structural as unknown as CmdPaletteStructuralCommand;
  });
  return {
    contractVersion: CMD_PALETTE_CATALOG_CONTRACT_VERSION,
    workspaceId,
    catalogRevision: catalog.catalog_revision,
    commands,
    sources: catalog.sources,
  };
}

/**
 * Validates a stored record. A corrupt or version-mismatched entry returns
 * `null` so the caller drops it and refetches in full — never a partial merge
 * (Safety Invariant 3).
 */
export function parseCatalogRecord(value: unknown): CmdPaletteCatalogRecord | null {
  const parsed = catalogRecordSchema.safeParse(value);
  if (!parsed.success) return null;
  return parsed.data as unknown as CmdPaletteCatalogRecord;
}
