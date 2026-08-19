/**
 * Client-side shapes for the `cmd_palette` domain (ADR-005).
 *
 * The daemon owns the catalog (ADR-006), so every wire shape here is an alias
 * over the generated OpenAPI operation types rather than a hand-written mirror —
 * a contract change fails typecheck instead of drifting silently. Only the types
 * the daemon has no opinion about (the resolved projection the client renders)
 * are declared locally.
 */
import type { operations } from "@/generated/compozy-openapi";

type CommandsResponse =
  operations["listCmdPaletteCommands"]["responses"][200]["content"]["application/json"];
type ClientsResponse =
  operations["listCmdPaletteClients"]["responses"][200]["content"]["application/json"];
type InvokeResponse =
  operations["invokeCmdPaletteCommand"]["responses"][200]["content"]["application/json"];

/** One command exactly as the daemon serves it. */
export type CmdPaletteCommand = CommandsResponse["commands"][number];
export type CmdPaletteAction = CmdPaletteCommand["action"];
export type CmdPaletteArgument = CmdPaletteCommand["arguments"][number];
export type CmdPalettePredicate = NonNullable<CmdPaletteCommand["when"]>[number];
export type CmdPaletteConfirmation = NonNullable<CmdPaletteCommand["confirmation"]>;
export type CmdPaletteSourceStatus = CommandsResponse["sources"][number];
export type CmdPaletteCatalogResponse = CommandsResponse;
export type CmdPaletteAttachedClient = ClientsResponse[number];
export type CmdPaletteInvokeResult = InvokeResponse;

/** The action kinds the dispatch seam routes (BR-2: closed union). */
export type CmdPaletteActionKind = "client_op" | "tool" | "view" | "navigate" | "url";

/**
 * The structural half of the catalog — what survives in the IndexedDB record.
 * Resolved availability is deliberately absent: it belongs to the client whose
 * context produced it and can never be replayed for another one (BR-19).
 */
export interface CmdPaletteStructuralCommand extends Omit<
  CmdPaletteCommand,
  "available" | "reason"
> {}

export interface CmdPaletteStructuralCatalog {
  readonly commands: readonly CmdPaletteStructuralCommand[];
  readonly sources: readonly CmdPaletteSourceStatus[];
  readonly catalogRevision: string;
}

/** How a command reads once the local context evaluator has judged it. */
export interface CmdPaletteAvailability {
  /** False hides the row entirely — fully irrelevant to this surface (US-037.AC-2). */
  readonly visible: boolean;
  readonly available: boolean;
  /** Verbatim runtime reason; empty when available. */
  readonly reason: string;
}

/** One command as every surface renders it. */
export interface ResolvedPaletteCommand
  extends CmdPaletteStructuralCommand, CmdPaletteAvailability {
  /** Effective chord labels from the daemon keymap; never a TS literal (BR-6). */
  readonly chords: readonly string[];
}

/**
 * The one client registry (`_uiux.md` Component plan): palette root, destination
 * mode, menubar, cheatsheet and the settings table all read this object.
 */
export interface PaletteRegistry {
  readonly commands: readonly ResolvedPaletteCommand[];
  readonly byId: ReadonlyMap<string, ResolvedPaletteCommand>;
  readonly sources: readonly CmdPaletteSourceStatus[];
  readonly catalogRevision: string;
  /** True while the projection is serving a cached catalog the daemon has not confirmed. */
  readonly stale: boolean;
  readonly daemonReachable: boolean;
}
