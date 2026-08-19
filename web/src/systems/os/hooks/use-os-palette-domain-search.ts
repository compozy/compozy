import { useAgents } from "@/systems/agent";
import { useAutomationJobs, useAutomationTriggers } from "@/systems/automation";
import { useBridges } from "@/systems/bridges";
import { useExtensionInventory } from "@/systems/extensions";
import { useMemories } from "@/systems/knowledge";
import { useLoops } from "@/systems/loops";
import { useMarketplaceSearch } from "@/systems/marketplace";
import { useNetworkChannels } from "@/systems/network";
import { useTasks } from "@/systems/tasks";
import { useVaultSecrets, type VaultSecret } from "@/systems/vault";

import type { CmdPaletteRankSignals } from "../lib/cmd-palette-types";
import type { OsAppId, OsWindowRoute } from "../lib/os-types";
import { rankCandidates } from "../lib/ranking/rank";
import type { RankingCandidate } from "../lib/ranking/types";

export interface OsPaletteDomainRow {
  readonly key: string;
  readonly label: string;
  readonly detail?: string;
  readonly workspaceLabel?: string;
  readonly app: OsAppId;
  readonly route: OsWindowRoute;
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

interface DomainSeed extends RankingCandidate {
  readonly row: OsPaletteDomainRow;
}

interface QueryState {
  readonly isError: boolean;
  readonly isLoading: boolean;
  readonly error: unknown;
}

export interface UseOsPaletteDomainSearchOptions {
  readonly open: boolean;
  readonly query: string;
  readonly workspaceId: string | null;
  readonly scope: "workspace" | "global";
  readonly workspaceNames: ReadonlyMap<string, string>;
  readonly signals: CmdPaletteRankSignals | null;
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

function errorMessage(title: string, error: unknown): string {
  const message = error instanceof Error ? error.message.trim() : "";
  return message === "" ? `${title} unavailable` : `${title}: ${message}`;
}

function workspaceLabel(
  scope: "workspace" | "global",
  workspaceId: string | null | undefined,
  names: ReadonlyMap<string, string>
): string | undefined {
  if (scope !== "global") return undefined;
  if (!workspaceId) return "Global";
  return names.get(workspaceId) ?? workspaceId;
}

function route(pathname: string, q: string): OsWindowRoute {
  return { pathname, search: { q } };
}

function rowSeed(
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

function section(
  title: string,
  seeds: readonly DomainSeed[],
  state: QueryState,
  enabled: boolean,
  query: string,
  signals: CmdPaletteRankSignals
): OsPaletteDomainSection {
  const ranked = rankCandidates(query, seeds, signals);
  return {
    title,
    rows: ranked
      .slice(0, signals.weights.entity_section_visible_cap)
      .map(candidate => candidate.candidate.row),
    total: ranked.length,
    loading: enabled && state.isLoading,
    error: enabled && state.isError ? errorMessage(title, state.error) : null,
  };
}

export function projectVaultRows(
  secrets: readonly VaultSecret[],
  scope: "workspace" | "global",
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
      route: route("/vault", name),
      namespace: secret.namespace,
      kind: secret.kind ?? "",
    };
  });
}

/**
 * Palette-open domain reads. Every hook remains mounted, but its request gate is
 * derived from the daemon's query-length policy so typing one character causes
 * no domain traffic.
 */
export function useOsPaletteDomainSearch({
  open,
  query,
  workspaceId,
  scope,
  workspaceNames,
  signals,
}: UseOsPaletteDomainSearchOptions): readonly OsPaletteDomainSection[] {
  const enabled = isPaletteDomainSearchEnabled(open, query, signals?.weights ?? null);
  const workspace = workspaceId ?? "";

  const agents = useAgents(workspaceId, { enabled });
  const tasks = useTasks({}, { enabled });
  const loops = useLoops(workspace, {}, enabled && workspace !== "");
  const jobs = useAutomationJobs({}, { enabled });
  const triggers = useAutomationTriggers({}, { enabled });
  const bridges = useBridges({}, { enabled });
  const globalMemories = useMemories({ scope: "global" }, { enabled });
  const workspaceMemories = useMemories(
    { scope: "workspace", workspaceId: workspace },
    { enabled: enabled && workspace !== "" }
  );
  const vault = useVaultSecrets({}, { enabled });
  const channels = useNetworkChannels({ enabled, workspaceId });
  const marketplace = useMarketplaceSearch({ q: query, workspaceId }, enabled);
  const extensions = useExtensionInventory(enabled);

  if (signals === null) return [];
  const wsLabel = (id?: string | null) => workspaceLabel(scope, id, workspaceNames);
  const domainSections = [
    section(
      "Agents",
      (agents.data ?? []).map(agent =>
        rowSeed("Agents", {
          key: `agent:${agent.name}`,
          label: agent.name,
          detail: agent.provider,
          workspaceLabel: wsLabel(agent.origin === "workspace" ? workspaceId : undefined),
          app: "agents",
          route: route("/agents", agent.name),
        })
      ),
      agents,
      enabled,
      query,
      signals
    ),
    section(
      "Tasks",
      tasks.data.map(task =>
        rowSeed(
          "Tasks",
          {
            key: `task:${task.id}`,
            label: task.title,
            detail: task.identifier,
            workspaceLabel: wsLabel(task.workspace_id),
            app: "tasks",
            route: route("/tasks", task.title),
          },
          task.identifier ? [task.identifier] : []
        )
      ),
      tasks,
      enabled,
      query,
      signals
    ),
    section(
      "Loops",
      loops.loops.map(loop =>
        rowSeed("Loops", {
          key: `loop:${loop.name}`,
          label: loop.name,
          detail: loop.contract.goal,
          workspaceLabel: wsLabel(workspaceId),
          app: "loops",
          route: route("/loops", loop.name),
        })
      ),
      loops,
      enabled,
      query,
      signals
    ),
    section(
      "Jobs",
      jobs.jobs.map(job =>
        rowSeed("Jobs", {
          key: `job:${job.id}`,
          label: job.name,
          detail: job.agent_name,
          workspaceLabel: wsLabel(job.workspace_id),
          app: "jobs",
          route: route("/jobs", job.name),
        })
      ),
      jobs,
      enabled,
      query,
      signals
    ),
    section(
      "Triggers",
      triggers.triggers.map(trigger =>
        rowSeed("Triggers", {
          key: `trigger:${trigger.id}`,
          label: trigger.name,
          detail: trigger.event,
          workspaceLabel: wsLabel(trigger.workspace_id),
          app: "triggers",
          route: route("/triggers", trigger.name),
        })
      ),
      triggers,
      enabled,
      query,
      signals
    ),
    section(
      "Bridges",
      bridges.bridges.map(bridge =>
        rowSeed("Bridges", {
          key: `bridge:${bridge.id}`,
          label: bridge.display_name,
          detail: bridge.platform,
          workspaceLabel: wsLabel(bridge.workspace_id),
          app: "bridges",
          route: route("/bridges", bridge.display_name),
        })
      ),
      bridges,
      enabled,
      query,
      signals
    ),
  ];

  const memories = [...(globalMemories.data ?? []), ...(workspaceMemories.data ?? [])];
  domainSections.push(
    section(
      "Knowledge",
      memories.map(memory =>
        rowSeed("Knowledge", {
          key: `knowledge:${memory.scope}:${memory.filename}`,
          label: memory.name,
          detail: memory.description,
          workspaceLabel: wsLabel(memory.workspace_id),
          app: "knowledge",
          route: route("/knowledge", memory.name),
        })
      ),
      {
        isLoading: globalMemories.isLoading || workspaceMemories.isLoading,
        isError: globalMemories.isError || workspaceMemories.isError,
        error: globalMemories.error ?? workspaceMemories.error,
      },
      enabled,
      query,
      signals
    )
  );
  const vaultRows = projectVaultRows(vault.data ?? [], scope, workspaceNames);
  domainSections.push(
    section(
      "Vault",
      vaultRows.map(row => rowSeed("Vault", row, [row.namespace, row.kind])),
      vault,
      enabled,
      query,
      signals
    ),
    section(
      "Network channels",
      channels.channels.map(channel =>
        rowSeed("Network channels", {
          key: `network-channel:${channel.workspace_id}:${channel.channel}`,
          label: channel.channel,
          detail: channel.purpose,
          workspaceLabel: wsLabel(channel.workspace_id),
          app: "network",
          route: route("/network", channel.channel),
        })
      ),
      channels,
      enabled,
      query,
      signals
    ),
    section(
      "Marketplace",
      (marketplace.data?.kinds ?? []).flatMap(kind =>
        kind.items.map(item =>
          rowSeed("Marketplace", {
            key: `marketplace:${item.kind}:${item.entry_id}`,
            label: item.name,
            detail: item.description,
            workspaceLabel: wsLabel(workspaceId),
            app: "marketplace",
            route: { pathname: item.manage_path ?? "/marketplace", search: { q: item.name } },
          })
        )
      ),
      marketplace,
      enabled,
      query,
      signals
    ),
    section(
      "Extensions",
      (extensions.data ?? []).map(item =>
        rowSeed("Extensions", {
          key: `extension:${item.extension.name}`,
          label: item.extension.name,
          detail: item.extension.health,
          workspaceLabel: wsLabel(extensions.workspaceId),
          app: "marketplace",
          route: route("/marketplace/extensions", item.extension.name),
        })
      ),
      extensions,
      enabled,
      query,
      signals
    )
  );

  const order = new Map(signals.weights.group_order.map((group, index) => [group, index]));
  return domainSections.sort(
    (left, right) =>
      (order.get(left.title) ?? order.size) - (order.get(right.title) ?? order.size) ||
      left.title.localeCompare(right.title)
  );
}
