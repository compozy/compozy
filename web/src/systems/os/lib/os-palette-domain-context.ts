import type { CmdPaletteRankSignals } from "./cmd-palette-types";
import type { WorkspaceScopeMode } from "@/systems/workspace";

export interface OsPaletteDomainContext {
  domainLimit?: number;
  open: boolean;
  profile: string;
  query: string;
  rootEnabled: boolean;
  scope: WorkspaceScopeMode;
  scopedWorkspace: string | null;
  signals: CmdPaletteRankSignals | null;
  targetDomain?: string;
  workspaceId: string | null;
  workspaceNames: ReadonlyMap<string, string>;
}

export function paletteDomainEnabled(context: OsPaletteDomainContext, title: string): boolean {
  return context.targetDomain === undefined
    ? context.rootEnabled
    : context.open && context.targetDomain === title;
}
