import { useQuery } from "@tanstack/react-query";

import type { ComputerId } from "@/lib/computer-scope";
import {
  terminalCatalogQuery,
  terminalScope,
  useTerminalCatalogStream,
  type TerminalInfo,
} from "@/systems/terminal/parts";

/**
 * The catalog row for a supervised terminal in a known session scope.
 *
 * Subscribes to that workspace + session profile only — never the Terminal
 * app's all-profiles aggregate. The stream hook refcounts per query client, so
 * many transcript blocks share one EventSource with the window that already
 * holds the same cache.
 */
export function useSessionTerminalCatalogEntry(
  terminalId: string,
  scope: { workspaceId: string; profile: string; computerId?: ComputerId }
): TerminalInfo | undefined {
  useTerminalCatalogStream({
    workspaceId: scope.workspaceId,
    profileKey: scope.profile,
    computerId: scope.computerId,
    allProfiles: false,
  });
  const catalog = useQuery(
    terminalCatalogQuery(terminalScope(scope.workspaceId, scope.profile, false, scope.computerId))
  );
  if (!catalog.data) return undefined;
  return catalog.data.find(terminal => terminal.id === terminalId);
}
