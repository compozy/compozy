import { useProfileReadScope } from "@/systems/profiles";
import { ALL_DATA_SCOPE, focusedWorkspaceSelector, type DataScope } from "@/systems/workspace";

import type { CmdPaletteRankSignals } from "../lib/cmd-palette-types";
import {
  paletteDomainEnabled,
  type OsPaletteDomainContext,
} from "../lib/os-palette-domain-context";
import {
  isPaletteDomainSearchEnabled,
  type OsPaletteDomainSection,
} from "../lib/os-palette-domain-search";
import { useOsPaletteEntitySections } from "./use-os-palette-entity-sections";
import { useOsPaletteResourceSections } from "./use-os-palette-resource-sections";
import { useOsPaletteTerminalSection } from "./use-os-palette-terminal-search";
import { useOsPaletteWorkspaceCatalogs } from "./use-os-palette-workspace-catalogs";

export type {
  OsPaletteDomainRow,
  OsPaletteDomainSection,
  OsPaletteVaultRow,
} from "../lib/os-palette-domain-search";
export { isPaletteDomainSearchEnabled, projectVaultRows } from "../lib/os-palette-domain-search";

export interface UseOsPaletteDomainSearchOptions {
  readonly open: boolean;
  readonly query: string;
  readonly workspaceId: string | null;
  /** `null` while the destination is unsettled: nothing is read. */
  readonly scope: DataScope | null;
  readonly workspaceNames: ReadonlyMap<string, string>;
  readonly signals: CmdPaletteRankSignals | null;
  /** A pushed view reads one full domain; root search reads every gated domain. */
  readonly targetDomain?: string;
}

/**
 * Palette-open domain reads. Each domain hook owns one catalog and one section
 * projection; this root only supplies the shared scope and preserves ordering.
 */
export function useOsPaletteDomainSearch({
  open,
  query,
  workspaceId,
  scope: requestedScope,
  workspaceNames,
  signals,
  targetDomain,
}: UseOsPaletteDomainSearchOptions): readonly OsPaletteDomainSection[] {
  const { destination: profile } = useProfileReadScope();
  const settled = open && requestedScope !== null;
  const scope = requestedScope ?? ALL_DATA_SCOPE;
  const all = scope.kind === "all";
  const scopedWorkspace = all ? null : focusedWorkspaceSelector(scope);
  const context: OsPaletteDomainContext = {
    domainLimit:
      targetDomain === undefined ? signals?.weights.entity_section_visible_cap : undefined,
    open: settled,
    profile,
    query,
    rootEnabled: isPaletteDomainSearchEnabled(settled, query, signals?.weights ?? null),
    scope,
    scopedWorkspace,
    signals,
    targetDomain,
    workspaceId,
    workspaceNames,
  };
  const workspaceIds = [...workspaceNames.keys()];
  const catalogs = useOsPaletteWorkspaceCatalogs({
    profile,
    workspaceIds,
    loopsEnabled: paletteDomainEnabled(context, "Loops") && all,
    agentsEnabled: paletteDomainEnabled(context, "Agents") && all,
    extensionsEnabled: paletteDomainEnabled(context, "Extensions") && all,
    worktreesEnabled: settled && targetDomain === "Worktrees" && all,
  });
  const sections = [
    ...useOsPaletteEntitySections(context, catalogs),
    ...useOsPaletteResourceSections(context, catalogs),
    useOsPaletteTerminalSection({
      enabled: paletteDomainEnabled(context, "Terminals"),
      workspaceId: context.scopedWorkspace,
      profile,
      query,
      signals,
      workspaceLabel: context.scopedWorkspace
        ? workspaceNames.get(context.scopedWorkspace)
        : undefined,
      limit: context.domainLimit,
    }),
  ];
  if (signals === null) return [];
  const order = new Map(signals.weights.group_order.map((group, index) => [group, index]));
  return sections.sort(
    (left, right) =>
      (order.get(left.title) ?? order.size) - (order.get(right.title) ?? order.size) ||
      left.title.localeCompare(right.title)
  );
}
