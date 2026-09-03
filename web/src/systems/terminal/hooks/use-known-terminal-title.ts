import { useQuery } from "@tanstack/react-query";

import { terminalCatalogQuery, terminalScope } from "../lib/query-options";

/**
 * Live catalog title for a terminal id, when this session's profile already
 * knows that terminal. Returns undefined when the id is missing, the catalog
 * has not loaded, or the catalog has no title — never the id as a stand-in.
 */
export function useKnownTerminalTitle(
  workspaceId: string,
  profile: string | undefined,
  terminalId: string | null
): string | undefined {
  const enabled = Boolean(workspaceId && profile && terminalId);
  const query = useQuery({
    ...terminalCatalogQuery(terminalScope(workspaceId, profile ?? "", false)),
    enabled,
  });
  if (!terminalId || !query.data) return undefined;
  const title = query.data.find(terminal => terminal.id === terminalId)?.title?.trim();
  return title || undefined;
}
