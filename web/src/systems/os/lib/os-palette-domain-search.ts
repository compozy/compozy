import type { VaultSecret } from "@/systems/vault";
import type { WorkspaceScopeMode } from "@/systems/workspace";
import { networkChannelLocation } from "@/systems/network/lib/network-window-location";

import { compareStatusAttentionFirst } from "@/lib/status-tone";

import type { CmdPaletteRankSignals } from "./cmd-palette-types";
import type { OsAppId, OsWindowRoute } from "./os-types";
import { rankCandidates } from "./ranking/rank";
import type { RankingCandidate } from "./ranking/types";

export interface OsPaletteDomainRow {
  readonly key: string;
  readonly label: string;
  readonly detail?: string;
  readonly workspaceLabel?: string;
  readonly app: OsAppId;
  readonly route: OsWindowRoute;
  readonly status?: string;
}

/** Names and non-secret metadata are the entire Vault palette contract. */
export interface OsPaletteVaultRow extends OsPaletteDomainRow {
  readonly namespace: string;
  readonly kind: string;
}

export interface OsPaletteDomainSection {
  readonly title: string;
  readonly rows: readonly OsPaletteDomainRow[];
  readonly total: number;
  readonly loading: boolean;
  readonly error: string | null;
}

export interface DomainSeed extends RankingCandidate {
  readonly row: OsPaletteDomainRow;
}

export interface QueryState {
  readonly isError: boolean;
  readonly isLoading: boolean;
  readonly error: unknown;
}

export function isPaletteDomainSearchEnabled(
  open: boolean,
  query: string,
  weights: CmdPaletteRankSignals["weights"] | null
): boolean {
  if (!open || weights === null) return false;
  const normalizedLength = query.trim().normalize("NFKD").length;
  return (
    normalizedLength >= weights.min_entity_query_length &&
    normalizedLength <= weights.max_query_length
  );
}

export function workspaceLabel(
  scope: WorkspaceScopeMode,
  workspaceId: string | null | undefined,
  names: ReadonlyMap<string, string>
): string | undefined {
  if (scope !== "global") return undefined;
  if (!workspaceId) return "Global";
  return names.get(workspaceId) ?? workspaceId;
}

export function searchRoute(pathname: string, q: string): OsWindowRoute {
  return { pathname, search: { q } };
}

export function tasksRoute(title: string): OsWindowRoute {
  return { pathname: "/tasks", search: { query: title } };
}

export function networkChannelRoute(workspaceId: string, channel: string): OsWindowRoute {
  return networkChannelLocation(workspaceId, channel, "threads");
}

export function knowledgeRoute(input: {
  filename: string;
  scope: "global" | "workspace";
  workspaceId?: string;
}): OsWindowRoute {
  return {
    pathname: "/knowledge",
    search: {
      memory: input.filename,
      scope: input.scope,
      ...(input.workspaceId ? { workspace: input.workspaceId } : {}),
    },
  };
}

export function marketplaceCatalogTotal(
  kinds: ReadonlyArray<{ items: readonly unknown[]; total?: number | null }> | undefined
): number {
  return (kinds ?? []).reduce((sum, kind) => sum + (kind.total ?? kind.items.length), 0);
}

export function rowSeed(
  title: string,
  row: OsPaletteDomainRow,
  keywords: readonly string[] = []
): DomainSeed {
  return {
    stableKey: row.key,
    id: row.key,
    label: row.label,
    description: row.detail,
    group: title,
    keywords,
    subtype: "path",
    row,
  };
}

export function section(
  title: string,
  seeds: readonly DomainSeed[],
  state: QueryState,
  enabled: boolean,
  query: string,
  signals: CmdPaletteRankSignals,
  options: { limit?: number; catalogTotal?: number } = {}
): OsPaletteDomainSection {
  const attentionOrdered = [...seeds].sort((left, right) =>
    compareStatusAttentionFirst(left.row.status, right.row.status)
  );
  const matched =
    query.trim() === ""
      ? attentionOrdered
      : rankCandidates(query, attentionOrdered, signals).map(candidate => candidate.candidate);
  const rows =
    options.limit === undefined
      ? matched.map(candidate => candidate.row)
      : matched.slice(0, options.limit).map(candidate => candidate.row);
  const emptyQuery = query.trim() === "";
  return {
    title,
    rows,
    total: emptyQuery ? (options.catalogTotal ?? matched.length) : matched.length,
    loading: enabled && state.isLoading,
    error: enabled && state.isError ? errorMessage(title, state.error) : null,
  };
}

export function projectVaultRows(
  secrets: readonly VaultSecret[],
  scope: WorkspaceScopeMode,
  names: ReadonlyMap<string, string>
): readonly OsPaletteVaultRow[] {
  return secrets.map(secret => {
    const name = secret.ref.split("/").at(-1) || secret.ref;
    return {
      key: `vault:${secret.ref}`,
      label: name,
      detail: [secret.namespace, secret.kind].filter(Boolean).join(" · "),
      workspaceLabel: workspaceLabel(scope, undefined, names),
      app: "vault",
      route: searchRoute("/vault", name),
      namespace: secret.namespace,
      kind: secret.kind ?? "",
    };
  });
}

export function paletteTaskFilters(
  scope: WorkspaceScopeMode,
  workspaceId: string | null
): { scope: "all" | "workspace"; workspace?: string } {
  if (scope === "workspace" && workspaceId) {
    return { scope: "workspace", workspace: workspaceId };
  }
  return { scope: "all" };
}

/**
 * Automation lists have no `scope: "all"` token. Omitting scope and workspace
 * is the complete population — not a stand-in for the home runtime id.
 */
export function paletteWorkspaceCatalogFilters(
  scope: WorkspaceScopeMode,
  workspaceId: string | null
): { scope?: "workspace"; workspace_id?: string } {
  if (scope === "workspace" && workspaceId) {
    return { scope: "workspace", workspace_id: workspaceId };
  }
  return {};
}

function errorMessage(title: string, error: unknown): string {
  const message = error instanceof Error ? error.message.trim() : "";
  return message === "" ? `${title} unavailable` : `${title}: ${message}`;
}
