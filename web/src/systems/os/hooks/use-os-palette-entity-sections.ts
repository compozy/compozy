import { useAgents } from "@/systems/agent";
import {
  compareAutomationViews,
  toAutomationView,
  useAutomationJobs,
  useAutomationTimeZone,
  useAutomationTriggers,
} from "@/systems/automation";
import { useLoops } from "@/systems/loops";
import { useTasks } from "@/systems/tasks";
import { useWorktrees } from "@/systems/workspace";

import {
  agentRoute,
  loopRoute,
  paletteTaskFilters,
  paletteWorkspaceCatalogFilters,
  projectAutomationRows,
  rowSeed,
  section,
  taskRoute,
  worktreeRowSeed,
  workspaceLabel,
  type OsPaletteDomainSection,
} from "../lib/os-palette-domain-search";
import {
  paletteDomainEnabled,
  type OsPaletteDomainContext,
} from "../lib/os-palette-domain-context";
import type { OsPaletteWorkspaceCatalogs } from "./use-os-palette-workspace-catalogs";
import { usePaletteInfiniteCatalog } from "./use-palette-infinite-catalog";

const EMPTY_SECTION = (title: string): OsPaletteDomainSection => ({
  title,
  rows: [],
  total: 0,
  loading: false,
  error: null,
});

export function useOsPaletteEntitySections(
  context: OsPaletteDomainContext,
  catalogs: OsPaletteWorkspaceCatalogs
): readonly OsPaletteDomainSection[] {
  return [
    useWorktreeSection(context, catalogs),
    useAgentSection(context, catalogs),
    useTaskSection(context),
    useLoopSection(context, catalogs),
    useAutomationSection(context),
  ];
}

function useWorktreeSection(context: OsPaletteDomainContext, catalogs: OsPaletteWorkspaceCatalogs) {
  const enabled =
    context.open && context.targetDomain === "Worktrees" && context.scopedWorkspace !== null;
  const worktrees = useWorktrees(context.scopedWorkspace, { enabled });
  if (context.signals === null) return EMPTY_SECTION("Worktrees");
  const rows = context.scope === "global" ? catalogs.worktrees : (worktrees.data?.worktrees ?? []);
  return section(
    "Worktrees",
    rows.map(worktree =>
      worktreeRowSeed(
        worktree,
        workspaceLabel(context.scope, worktree.workspace_id, context.workspaceNames)
      )
    ),
    context.scope === "global" ? catalogs.worktreeState : worktrees,
    paletteDomainEnabled(context, "Worktrees"),
    context.query,
    context.signals,
    {
      limit: context.domainLimit,
      catalogTotal:
        context.scope === "global" ? catalogs.worktreeTotal : worktrees.data?.worktrees.length,
    }
  );
}

function useAgentSection(context: OsPaletteDomainContext, catalogs: OsPaletteWorkspaceCatalogs) {
  const globalEnabled = paletteDomainEnabled(context, "Agents") && context.scope === "global";
  const globalAgents = useAgents(null, { enabled: globalEnabled });
  const workspaceAgents = useAgents(context.scopedWorkspace, {
    enabled: paletteDomainEnabled(context, "Agents") && context.scope === "workspace",
  });
  if (context.signals === null) return EMPTY_SECTION("Agents");
  const rows =
    context.scope === "global"
      ? [
          ...(globalAgents.data ?? []).map(agent => ({ agent, workspaceId: agent.workspace_id })),
          ...catalogs.workspaceAgents,
        ]
      : (workspaceAgents.data ?? []).map(agent => ({
          agent,
          workspaceId: agent.workspace_id ?? context.workspaceId,
        }));
  return section(
    "Agents",
    rows.map(({ agent, workspaceId }) =>
      rowSeed("Agents", {
        key: `agent:${workspaceId ?? "global"}:${agent.name}`,
        label: agent.name,
        detail: agent.provider,
        workspaceLabel: workspaceLabel(
          context.scope,
          agent.origin === "workspace" ? workspaceId : undefined,
          context.workspaceNames
        ),
        app: "agents",
        route: agentRoute(agent.name),
        ...(agent.origin === "workspace" && workspaceId ? { workspaceId } : {}),
      })
    ),
    context.scope === "global"
      ? {
          isLoading: globalAgents.isLoading || catalogs.workspaceAgentState.isLoading,
          isError: globalAgents.isError || catalogs.workspaceAgentState.isError,
          error: globalAgents.error ?? catalogs.workspaceAgentState.error,
        }
      : workspaceAgents,
    paletteDomainEnabled(context, "Agents"),
    context.query,
    context.signals,
    { limit: context.domainLimit, catalogTotal: rows.length }
  );
}

function useTaskSection(context: OsPaletteDomainContext) {
  const enabled = paletteDomainEnabled(context, "Tasks");
  const tasks = useTasks(paletteTaskFilters(context.scope, context.workspaceId), { enabled });
  usePaletteInfiniteCatalog(tasks, enabled);
  if (context.signals === null) return EMPTY_SECTION("Tasks");
  return section(
    "Tasks",
    tasks.data.map(task =>
      rowSeed(
        "Tasks",
        {
          key: `task:${task.id}`,
          label: task.title,
          detail: task.identifier,
          status: task.status,
          workspaceLabel: workspaceLabel(context.scope, task.workspace_id, context.workspaceNames),
          app: "tasks",
          route: taskRoute(task.id),
          ...(task.workspace_id ? { workspaceId: task.workspace_id } : {}),
        },
        task.identifier ? [task.identifier] : []
      )
    ),
    tasks,
    enabled,
    context.query,
    context.signals,
    { limit: context.domainLimit, catalogTotal: tasks.total }
  );
}

function useLoopSection(context: OsPaletteDomainContext, catalogs: OsPaletteWorkspaceCatalogs) {
  const workspaceEnabled =
    paletteDomainEnabled(context, "Loops") && Boolean(context.scopedWorkspace);
  const globalEnabled = paletteDomainEnabled(context, "Loops") && context.scope === "global";
  const loops = useLoops(context.scopedWorkspace ?? "", {}, workspaceEnabled);
  usePaletteInfiniteCatalog(loops, workspaceEnabled);
  if (context.signals === null) return EMPTY_SECTION("Loops");
  const rows =
    context.scope === "global"
      ? catalogs.loops
      : loops.loops.map(loop => ({ loop, workspaceId: context.scopedWorkspace }));
  return section(
    "Loops",
    rows.map(({ loop, workspaceId }) =>
      rowSeed("Loops", {
        key: `loop:${workspaceId ?? "global"}:${loop.name}`,
        label: loop.name,
        detail: loop.contract.goal,
        workspaceLabel: workspaceLabel(context.scope, workspaceId, context.workspaceNames),
        app: "loops",
        route: loopRoute(loop.name, workspaceId),
        ...(workspaceId ? { workspaceId } : {}),
      })
    ),
    context.scope === "global" ? catalogs.loopState : loops,
    workspaceEnabled || globalEnabled,
    context.query,
    context.signals,
    {
      limit: context.domainLimit,
      catalogTotal: context.scope === "global" ? catalogs.loopTotal : loops.total,
    }
  );
}

function useAutomationSection(context: OsPaletteDomainContext) {
  const enabled = paletteDomainEnabled(context, "Automations");
  const filters = paletteWorkspaceCatalogFilters(context.scope, context.workspaceId);
  const jobs = useAutomationJobs(filters, { enabled });
  const triggers = useAutomationTriggers(filters, { enabled });
  const timeZone = useAutomationTimeZone();
  usePaletteInfiniteCatalog(jobs, enabled);
  usePaletteInfiniteCatalog(triggers, enabled);
  if (context.signals === null) return EMPTY_SECTION("Automations");
  const sentence = { timeZone, workspaceName: (id: string) => context.workspaceNames.get(id) };
  const views = [
    ...jobs.jobs.map(job => toAutomationView(job, sentence)),
    ...triggers.triggers.map(trigger => toAutomationView(trigger, sentence)),
  ].sort(compareAutomationViews);
  const rows = projectAutomationRows(views, context.scope, context.workspaceNames);
  const failed = jobs.isError ? jobs : triggers;
  return section(
    "Automations",
    rows.map(row => rowSeed("Automations", row)),
    {
      isError: jobs.isError || triggers.isError,
      isLoading: jobs.isLoading || triggers.isLoading,
      error: failed.error,
    },
    enabled,
    context.query,
    context.signals,
    { limit: context.domainLimit, catalogTotal: jobs.total + triggers.total }
  );
}
