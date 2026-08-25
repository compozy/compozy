import { useAgents } from "@/systems/agent";
import { useAutomationJobs, useAutomationTriggers } from "@/systems/automation";
import { useBridges } from "@/systems/bridges";
import { useExtensionInventory } from "@/systems/extensions";
import { useMemories } from "@/systems/knowledge";
import { useLoops } from "@/systems/loops";
import { useMarketplaceSearch } from "@/systems/marketplace";
import { useNetworkChannels } from "@/systems/network";
import { useProfileReadScope } from "@/systems/profiles";
import { useTasks } from "@/systems/tasks";
import { useVaultSecrets } from "@/systems/vault";
import { useWorktrees, type WorkspaceScopeMode } from "@/systems/workspace";

import { usePaletteInfiniteCatalog } from "./use-palette-infinite-catalog";
import {
  isPaletteDomainSearchEnabled,
  agentRoute,
  bridgeRoute,
  jobRoute,
  knowledgeRoute,
  loopRoute,
  marketplaceCatalogTotal,
  marketplaceEntryRoute,
  networkChannelRoute,
  paletteTaskFilters,
  paletteWorkspaceCatalogFilters,
  projectVaultRows,
  rowSeed,
  section,
  taskRoute,
  triggerRoute,
  workspaceLabel,
  type OsPaletteDomainSection,
} from "../lib/os-palette-domain-search";
import type { CmdPaletteRankSignals } from "../lib/cmd-palette-types";
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
  readonly scope: WorkspaceScopeMode;
  readonly workspaceNames: ReadonlyMap<string, string>;
  readonly signals: CmdPaletteRankSignals | null;
  /** A pushed view reads one full domain; root search reads every gated domain. */
  readonly targetDomain?: string;
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
  targetDomain,
}: UseOsPaletteDomainSearchOptions): readonly OsPaletteDomainSection[] {
  const { destination: profile } = useProfileReadScope();
  const rootEnabled = isPaletteDomainSearchEnabled(open, query, signals?.weights ?? null);
  const domainEnabled = (title: string) =>
    targetDomain === undefined ? rootEnabled : open && targetDomain === title;
  const domainLimit =
    targetDomain === undefined ? signals?.weights.entity_section_visible_cap : undefined;
  const scopedWorkspace = scope === "workspace" ? workspaceId : null;
  const workspaceIds = [...workspaceNames.keys()];
  const workspaceCatalog = paletteWorkspaceCatalogFilters(scope, workspaceId);
  const taskFilters = paletteTaskFilters(scope, workspaceId);
  const loopsWorkspaceEnabled =
    domainEnabled("Loops") && scopedWorkspace !== null && scopedWorkspace !== "";
  const loopsGlobalEnabled = domainEnabled("Loops") && scope === "global";
  const networkWorkspaceEnabled =
    domainEnabled("Network channels") && scopedWorkspace !== null && scopedWorkspace !== "";
  const networkGlobalEnabled = domainEnabled("Network channels") && scope === "global";
  const workspaceKnowledgeEnabled = domainEnabled("Knowledge") && scopedWorkspace !== null;
  const globalKnowledgeEnabled = domainEnabled("Knowledge") && scope === "global";
  const agentsGlobalEnabled = domainEnabled("Agents") && scope === "global";
  const extensionsEnabled = domainEnabled("Extensions");
  const worktreesWorkspaceEnabled =
    open && targetDomain === "Worktrees" && scopedWorkspace !== null;
  const worktreesGlobalEnabled = open && targetDomain === "Worktrees" && scope === "global";

  const worktrees = useWorktrees(scopedWorkspace, {
    enabled: worktreesWorkspaceEnabled,
  });
  const globalAgents = useAgents(null, { enabled: agentsGlobalEnabled });
  const workspaceAgents = useAgents(scopedWorkspace, {
    enabled: domainEnabled("Agents") && scope === "workspace",
  });
  const tasks = useTasks(taskFilters, { enabled: domainEnabled("Tasks") });
  const loops = useLoops(scopedWorkspace ?? "", {}, loopsWorkspaceEnabled);
  const jobs = useAutomationJobs(workspaceCatalog, { enabled: domainEnabled("Jobs") });
  const triggers = useAutomationTriggers(workspaceCatalog, { enabled: domainEnabled("Triggers") });
  const bridges = useBridges(
    scope === "workspace" && workspaceId
      ? { scope: "workspace", workspace_id: workspaceId }
      : { scope: "all" },
    { enabled: domainEnabled("Bridges") }
  );
  const globalMemories = useMemories(
    { profile, scope: "profile" },
    { enabled: globalKnowledgeEnabled }
  );
  const workspaceMemories = useMemories(
    { profile, scope: "workspace", workspaceId: scopedWorkspace ?? "" },
    { enabled: workspaceKnowledgeEnabled }
  );
  const vault = useVaultSecrets({}, { enabled: domainEnabled("Vault") });
  const channels = useNetworkChannels({
    enabled: networkWorkspaceEnabled,
    workspaceId: scopedWorkspace,
  });
  const marketplace = useMarketplaceSearch(
    {
      q: query,
      ...(scopedWorkspace ? { workspaceId: scopedWorkspace } : {}),
    },
    domainEnabled("Marketplace")
  );
  const publishedExtensions = useExtensionInventory(extensionsEnabled && scope === "workspace");
  const catalogs = useOsPaletteWorkspaceCatalogs({
    profile,
    workspaceIds,
    loopsEnabled: loopsGlobalEnabled,
    networkEnabled: networkGlobalEnabled,
    knowledgeEnabled: globalKnowledgeEnabled,
    agentsEnabled: agentsGlobalEnabled,
    extensionsEnabled: extensionsEnabled && scope === "global",
    worktreesEnabled: worktreesGlobalEnabled,
  });

  usePaletteInfiniteCatalog(tasks, domainEnabled("Tasks"));
  usePaletteInfiniteCatalog(jobs, domainEnabled("Jobs"));
  usePaletteInfiniteCatalog(triggers, domainEnabled("Triggers"));
  usePaletteInfiniteCatalog(bridges, domainEnabled("Bridges"));
  usePaletteInfiniteCatalog(loops, loopsWorkspaceEnabled);
  usePaletteInfiniteCatalog(globalMemories, globalKnowledgeEnabled);
  usePaletteInfiniteCatalog(workspaceMemories, workspaceKnowledgeEnabled);

  if (signals === null) return [];
  const wsLabel = (id?: string | null) => workspaceLabel(scope, id, workspaceNames);
  const limit = { limit: domainLimit };
  const worktreeRows = scope === "global" ? catalogs.worktrees : (worktrees.data?.worktrees ?? []);
  const agentRows =
    scope === "global"
      ? [
          ...(globalAgents.data ?? []).map(agent => ({
            agent,
            workspaceId: agent.workspace_id,
          })),
          ...catalogs.workspaceAgents,
        ]
      : (workspaceAgents.data ?? []).map(agent => ({
          agent,
          workspaceId: agent.workspace_id ?? workspaceId,
        }));
  const loopsEnabled = loopsWorkspaceEnabled || loopsGlobalEnabled;
  const loopRows =
    scope === "global"
      ? catalogs.loops.map(({ loop, workspaceId: loopWorkspaceId }) => ({
          loop,
          workspaceId: loopWorkspaceId,
        }))
      : loops.loops.map(loop => ({ loop, workspaceId: scopedWorkspace }));
  const loopState = scope === "global" ? catalogs.loopState : loops;
  const loopTotal = scope === "global" ? catalogs.loopTotal : loops.total;
  const domainSections = [
    section(
      "Worktrees",
      worktreeRows.map(worktree =>
        rowSeed("Worktrees", {
          key: `worktree:${worktree.id}`,
          label: worktree.name,
          detail: worktree.branch,
          workspaceLabel: wsLabel(worktree.workspace_id),
          status: worktree.state,
          app: "dashboard",
          route: { pathname: "/", search: {} },
          worktreeSelection: {
            workspaceId: worktree.workspace_id,
            worktreeId: worktree.id,
          },
        })
      ),
      scope === "global" ? catalogs.worktreeState : worktrees,
      domainEnabled("Worktrees"),
      query,
      signals,
      {
        ...limit,
        catalogTotal:
          scope === "global" ? catalogs.worktreeTotal : worktrees.data?.worktrees.length,
      }
    ),
    section(
      "Agents",
      agentRows.map(({ agent, workspaceId: agentWorkspaceId }) =>
        rowSeed("Agents", {
          key: `agent:${agentWorkspaceId ?? "global"}:${agent.name}`,
          label: agent.name,
          detail: agent.provider,
          workspaceLabel: wsLabel(agent.origin === "workspace" ? agentWorkspaceId : undefined),
          app: "agents",
          route: agentRoute(agent.name),
          ...(agent.origin === "workspace" && agentWorkspaceId
            ? { workspaceId: agentWorkspaceId }
            : {}),
        })
      ),
      scope === "global"
        ? {
            isLoading: globalAgents.isLoading || catalogs.workspaceAgentState.isLoading,
            isError: globalAgents.isError || catalogs.workspaceAgentState.isError,
            error: globalAgents.error ?? catalogs.workspaceAgentState.error,
          }
        : workspaceAgents,
      domainEnabled("Agents"),
      query,
      signals,
      { ...limit, catalogTotal: agentRows.length }
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
            status: task.status,
            workspaceLabel: wsLabel(task.workspace_id),
            app: "tasks",
            route: taskRoute(task.id),
            ...(task.workspace_id ? { workspaceId: task.workspace_id } : {}),
          },
          task.identifier ? [task.identifier] : []
        )
      ),
      tasks,
      domainEnabled("Tasks"),
      query,
      signals,
      { ...limit, catalogTotal: tasks.total }
    ),
    section(
      "Loops",
      loopRows.map(({ loop, workspaceId: loopWorkspaceId }) =>
        rowSeed("Loops", {
          key: `loop:${loopWorkspaceId ?? "global"}:${loop.name}`,
          label: loop.name,
          detail: loop.contract.goal,
          workspaceLabel: wsLabel(loopWorkspaceId),
          app: "loops",
          route: loopRoute(loop.name, loopWorkspaceId),
          ...(loopWorkspaceId ? { workspaceId: loopWorkspaceId } : {}),
        })
      ),
      loopState,
      loopsEnabled,
      query,
      signals,
      { ...limit, catalogTotal: loopTotal }
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
          route: jobRoute(job.id),
          ...(job.workspace_id ? { workspaceId: job.workspace_id } : {}),
        })
      ),
      jobs,
      domainEnabled("Jobs"),
      query,
      signals,
      { ...limit, catalogTotal: jobs.total }
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
          route: triggerRoute(trigger.id),
          ...(trigger.workspace_id ? { workspaceId: trigger.workspace_id } : {}),
        })
      ),
      triggers,
      domainEnabled("Triggers"),
      query,
      signals,
      { ...limit, catalogTotal: triggers.total }
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
          route: bridgeRoute(bridge.id),
          ...(bridge.workspace_id ? { workspaceId: bridge.workspace_id } : {}),
        })
      ),
      bridges,
      domainEnabled("Bridges"),
      query,
      signals,
      { ...limit, catalogTotal: bridges.total }
    ),
  ];

  const memories =
    scope === "global"
      ? [...(globalMemories.data ?? []), ...catalogs.workspaceMemories]
      : (workspaceMemories.data ?? []);
  const knowledgeTotal =
    scope === "global"
      ? globalMemories.total + catalogs.workspaceMemoryTotal
      : workspaceMemories.total;
  domainSections.push(
    section(
      "Knowledge",
      memories.map(memory =>
        rowSeed("Knowledge", {
          key: `knowledge:${memory.scope}:${memory.workspace_id ?? ""}:${memory.filename}`,
          label: memory.name,
          detail: memory.description,
          workspaceLabel: wsLabel(memory.workspace_id),
          app: "knowledge",
          route: knowledgeRoute({
            filename: memory.filename,
            scope: memory.scope === "workspace" ? "workspace" : "global",
            workspaceId: memory.workspace_id,
          }),
          workspaceId: memory.scope === "workspace" ? memory.workspace_id : undefined,
        })
      ),
      {
        isLoading:
          scope === "global"
            ? globalMemories.isLoading || catalogs.workspaceMemoryState.isLoading
            : workspaceMemories.isLoading,
        isError:
          scope === "global"
            ? globalMemories.isError || catalogs.workspaceMemoryState.isError
            : workspaceMemories.isError,
        error:
          scope === "global"
            ? (globalMemories.error ?? catalogs.workspaceMemoryState.error)
            : workspaceMemories.error,
      },
      domainEnabled("Knowledge"),
      query,
      signals,
      { ...limit, catalogTotal: knowledgeTotal }
    )
  );
  const vaultRows = projectVaultRows(vault.data ?? [], scope, workspaceNames);
  domainSections.push(
    section(
      "Vault",
      vaultRows.map(row => rowSeed("Vault", row, [row.namespace, row.kind])),
      vault,
      domainEnabled("Vault"),
      query,
      signals,
      { ...limit, catalogTotal: vaultRows.length }
    ),
    section(
      "Network channels",
      (scope === "global" ? catalogs.channels : channels.channels).flatMap(channel => {
        const channelWorkspaceId = channel.workspace_id ?? scopedWorkspace;
        if (!channelWorkspaceId) return [];
        return [
          rowSeed("Network channels", {
            key: `network-channel:${channelWorkspaceId}:${channel.channel}`,
            label: channel.channel,
            detail: channel.purpose,
            workspaceLabel: wsLabel(channelWorkspaceId),
            app: "network",
            route: networkChannelRoute(channelWorkspaceId, channel.channel),
            workspaceId: channelWorkspaceId,
          }),
        ];
      }),
      scope === "global" ? catalogs.channelState : channels,
      networkWorkspaceEnabled || networkGlobalEnabled,
      query,
      signals,
      {
        ...limit,
        catalogTotal: scope === "global" ? catalogs.channelTotal : channels.channels.length,
      }
    ),
    section(
      "Marketplace",
      (marketplace.data?.kinds ?? []).flatMap(kind =>
        kind.items.map(item =>
          rowSeed("Marketplace", {
            key: `marketplace:${item.kind}:${item.entry_id}`,
            label: item.name,
            detail: item.description,
            workspaceLabel: wsLabel(scopedWorkspace),
            app: "marketplace",
            route: marketplaceEntryRoute({
              kind: item.kind,
              entryId: item.entry_id,
              scope,
              workspaceId: scopedWorkspace,
              installedName: item.installed_name,
            }),
            ...(scopedWorkspace ? { workspaceId: scopedWorkspace } : {}),
          })
        )
      ),
      marketplace,
      domainEnabled("Marketplace"),
      query,
      signals,
      { ...limit, catalogTotal: marketplaceCatalogTotal(marketplace.data?.kinds) }
    ),
    section(
      "Extensions",
      (scope === "global"
        ? [
            ...catalogs.publishedExtensions.map(extension => ({
              extension,
              workspaceId: undefined as string | undefined,
            })),
            ...catalogs.workspaceExtensions,
          ]
        : (publishedExtensions.data ?? []).map(item => ({
            extension: item.extension,
            workspaceId: publishedExtensions.workspaceId ?? undefined,
          }))
      ).map(({ extension, workspaceId: extensionWorkspaceId }) =>
        rowSeed("Extensions", {
          key: `extension:${extensionWorkspaceId ?? "published"}:${extension.name}`,
          label: extension.name,
          detail: extension.health,
          workspaceLabel: wsLabel(extensionWorkspaceId),
          app: "marketplace",
          route: marketplaceEntryRoute({
            kind: "extension",
            entryId: extension.marketplace?.entry_id ?? extension.name,
            scope: extensionWorkspaceId ? "workspace" : "global",
            workspaceId: extensionWorkspaceId,
            installedName: extension.name,
          }),
          ...(extensionWorkspaceId ? { workspaceId: extensionWorkspaceId } : {}),
        })
      ),
      scope === "global" ? catalogs.workspaceExtensionState : publishedExtensions,
      extensionsEnabled,
      query,
      signals,
      {
        ...limit,
        catalogTotal:
          scope === "global"
            ? catalogs.publishedExtensions.length + catalogs.workspaceExtensions.length
            : publishedExtensions.data?.length,
      }
    )
  );

  const order = new Map(signals.weights.group_order.map((group, index) => [group, index]));
  return domainSections.sort(
    (left, right) =>
      (order.get(left.title) ?? order.size) - (order.get(right.title) ?? order.size) ||
      left.title.localeCompare(right.title)
  );
}
