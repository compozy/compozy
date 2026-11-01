import type { LucideIcon } from "@compozy/ui/icons";

import {
  automationSentenceText,
  AUTOMATION_START_ICON,
  compareAutomationViews,
  toAutomationView,
  type AutomationJob,
  type AutomationTrigger,
  type AutomationView,
} from "@/systems/automation";
import type { VaultSecret } from "@/systems/vault";
import { ownerFromRow, type ProfileOwner, type ProfileOwnerLabel } from "@/systems/profiles";
import {
  breadthQuery,
  DESTINATION_COPY,
  taskBreadthParams,
  type DataScope,
} from "@/systems/workspace";

import { compareStatusAttentionFirst } from "@/lib/status-tone";

import type { CmdPaletteRankSignals } from "./cmd-palette-types";
import type { OsAppId, OsWindowRoute } from "./os-types";
import { rankCandidates } from "./ranking/rank";
import type { RankingCandidate } from "./ranking/types";
import { humanizeAgentSlug } from "@/systems/agent/lib/agent-identity";

export interface OsPaletteDomainRow {
  readonly key: string;
  readonly label: string;
  readonly detail?: string;
  readonly workspaceLabel?: string;
  /** Owning workspace for global rows that need a workspace switch before opening. */
  readonly workspaceId?: string;
  readonly app: OsAppId;
  /** Kind glyph that replaces the app mark (automation start kinds). */
  readonly icon?: LucideIcon;
  readonly route: OsWindowRoute;
  /** Worktree rows scope the shell instead of opening an application route. */
  readonly worktreeSelection?: {
    readonly workspaceId: string;
    readonly worktreeId: string;
  };
  readonly status?: string;
  /** Worktrees always name their profile because disk state crosses profile lenses. */
  readonly owner?: ProfileOwner;
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

export interface PaletteWorktreeRow extends ProfileOwnerLabel {
  readonly id: string;
  readonly name: string;
  readonly branch: string;
  readonly state: string;
  readonly workspace_id: string;
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

/** Owner label under All projects only: the project's name, or "User" with no project. */
export function workspaceLabel(
  scope: DataScope,
  workspaceId: string | null | undefined,
  names: ReadonlyMap<string, string>
): string | undefined {
  if (scope.kind !== "all") return undefined;
  if (!workspaceId) return DESTINATION_COPY.user;
  return names.get(workspaceId) ?? workspaceId;
}

function encodedSegment(value: string): string {
  return encodeURIComponent(value.trim());
}

export function taskRoute(id: string): OsWindowRoute {
  return { pathname: `/tasks/${encodedSegment(id)}`, search: {} };
}

export function loopRoute(name: string, workspaceId?: string | null): OsWindowRoute {
  const workspace = workspaceId?.trim();
  return {
    pathname: `/loops/${encodedSegment(name)}`,
    search: workspace ? { workspace } : {},
  };
}

export function agentRoute(name: string): OsWindowRoute {
  return { pathname: `/agents/${encodedSegment(name)}`, search: {} };
}

export function terminalRoute(id: string): OsWindowRoute {
  return { pathname: `/terminal/${encodedSegment(id)}`, search: {} };
}

export function marketplaceEntryRoute(input: {
  source?: string;
  profileName?: string;
  entryId: string;
  scope: DataScope;
  workspaceId?: string | null;
  installedName?: string | null;
}): OsWindowRoute {
  const workspace = input.workspaceId?.trim();
  const installedName = input.installedName?.trim();
  const search =
    input.scope.kind === "project" && workspace
      ? {
          scope: "workspace",
          workspace_id: workspace,
          ...(installedName ? { installed_name: installedName } : {}),
        }
      : {
          scope: "user",
          ...(installedName ? { installed_name: installedName } : {}),
        };
  return {
    pathname: `/marketplace/${encodedSegment(input.entryId)}`,
    search: {
      ...search,
      ...(input.source ? { source: input.source } : {}),
      ...(input.profileName ? { profile: input.profileName } : {}),
    },
  };
}

export function vaultRoute(ref: string): OsWindowRoute {
  return { pathname: "/vault", search: { ref: ref.trim() } };
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

export function worktreeRowSeed(worktree: PaletteWorktreeRow, workspaceName?: string): DomainSeed {
  return rowSeed(
    "Worktrees",
    {
      key: `worktree:${worktree.id}`,
      label: worktree.name,
      detail: worktree.branch,
      workspaceLabel: workspaceName,
      status: worktree.state,
      owner: ownerFromRow(worktree),
      app: "dashboard",
      route: { pathname: "/", search: {} },
      worktreeSelection: {
        workspaceId: worktree.workspace_id,
        worktreeId: worktree.id,
      },
    },
    [worktree.profile_name]
  );
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
  scope: DataScope,
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
      route: vaultRoute(secret.ref),
      namespace: secret.namespace,
      kind: secret.kind ?? "",
    };
  });
}

/** Both automation kinds as one section: start glyph, name, and the sentence as subtitle. */
/**
 * Palette automation views with the same sentence as the listing: the project name
 * resolves from every registered workspace, not only the All projects label index.
 */
export function paletteAutomationViews(
  jobs: readonly AutomationJob[],
  triggers: readonly AutomationTrigger[],
  ctx: {
    timeZone?: string;
    names: ReadonlyMap<string, string>;
    projects: readonly { id: string; name: string }[];
  }
): AutomationView[] {
  const sentence = {
    timeZone: ctx.timeZone,
    agentLabel: humanizeAgentSlug,
    workspaceName: (id: string) =>
      ctx.names.get(id) ?? ctx.projects.find(project => project.id === id)?.name,
  };
  return [
    ...jobs.map(job => toAutomationView(job, sentence)),
    ...triggers.map(trigger => toAutomationView(trigger, sentence)),
  ].sort(compareAutomationViews);
}

export function projectAutomationRows(
  views: readonly AutomationView[],
  scope: DataScope,
  names: ReadonlyMap<string, string>
): readonly OsPaletteDomainRow[] {
  return views.map(view => ({
    key: `${view.kind}:${view.id}`,
    label: view.name,
    detail: automationSentenceText(view.sentence),
    workspaceLabel: workspaceLabel(scope, view.workspaceId, names),
    app: "automations",
    icon: AUTOMATION_START_ICON[view.start],
    route: { pathname: view.detailPath, search: {} },
    ...(view.workspaceId ? { workspaceId: view.workspaceId } : {}),
  }));
}

/** Tasks: the project's, User's, or every project's (`all_workspaces`), through the task encoder. */
export function paletteTaskFilters(scope: DataScope) {
  return taskBreadthParams(breadthQuery(scope));
}

/**
 * Automations: the project's, User's (`scope=global`), or — with no `scope:
 * "all"` token — the complete population by omitting scope and workspace.
 */
export function paletteWorkspaceCatalogFilters(scope: DataScope): {
  scope?: "global" | "workspace";
  workspace_id?: string;
} {
  if (scope.kind === "project") return { scope: "workspace", workspace_id: scope.workspaceId };
  if (scope.kind === "user") return { scope: "global" };
  return {};
}

function errorMessage(title: string, error: unknown): string {
  const message = error instanceof Error ? error.message.trim() : "";
  return message === "" ? `${title} unavailable` : `${title}: ${message}`;
}
