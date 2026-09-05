import { useQuery } from "@tanstack/react-query";

import { terminalCatalogQuery, terminalScope, type TerminalInfo } from "@/systems/terminal/parts";

import type { CmdPaletteRankSignals } from "../lib/cmd-palette-types";
import {
  rowSeed,
  section,
  terminalRoute,
  type OsPaletteDomainSection,
} from "../lib/os-palette-domain-search";

export interface UseOsPaletteTerminalSearchOptions {
  readonly enabled: boolean;
  readonly workspaceId: string | null;
  readonly profile: string;
  readonly query: string;
  readonly signals: CmdPaletteRankSignals | null;
  readonly workspaceLabel?: string;
  readonly limit?: number;
}

/**
 * Live terminal jump rows for palette domain search.
 *
 * Each row is a real catalog terminal with its id, title, and state.
 * Create/open lives on `app.open.terminal`, not here.
 */
export function useOsPaletteTerminalSection({
  enabled,
  workspaceId,
  profile,
  query,
  signals,
  workspaceLabel,
  limit,
}: UseOsPaletteTerminalSearchOptions): OsPaletteDomainSection {
  const catalog = useQuery({
    ...terminalCatalogQuery(terminalScope(workspaceId ?? "", profile)),
    enabled: enabled && workspaceId !== null && workspaceId !== "",
  });
  if (signals === null) {
    return { title: "Terminals", rows: [], total: 0, loading: false, error: null };
  }
  return section(
    "Terminals",
    (catalog.data ?? []).map(terminal =>
      rowSeed("Terminals", terminalPaletteRow(terminal, workspaceId, workspaceLabel))
    ),
    catalog,
    enabled,
    query,
    signals,
    { limit, catalogTotal: catalog.data?.length }
  );
}

function terminalPaletteRow(
  terminal: TerminalInfo,
  workspaceId: string | null,
  workspaceLabel: string | undefined
) {
  return {
    key: `terminal:${terminal.id}`,
    label: terminal.title,
    detail: terminalPaletteDetail(terminal),
    status: terminal.state,
    workspaceLabel,
    app: "terminal" as const,
    route: terminalRoute(terminal.id),
    ...(workspaceId ? { workspaceId } : {}),
  };
}

function terminalPaletteDetail(terminal: TerminalInfo): string {
  return terminal.state;
}
