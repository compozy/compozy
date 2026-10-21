import { useAgents } from "@/systems/agent";
import {
  useAutomationJobs,
  useAutomationTimeZone,
  useAutomationTriggers,
} from "@/systems/automation";
import { useLoops } from "@/systems/loops";
import { useTasks } from "@/systems/tasks";
import { useActiveWorkspace, useWorktrees } from "@/systems/workspace";

import {
  agentRoute,
  loopRoute,
  paletteTaskFilters,
  paletteWorkspaceCatalogFilters,
  paletteAutomationViews,
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

/** Worktrees belong to projects: the focused project's, or every project's; none in User. */
function useWorktreeSection(context: OsPaletteDomainContext, catalogs: OsPaletteWorkspaceCatalogs) {
  const projectId = context.scope.kind === "project" ? context.scope.workspaceId : null;
  const enabled = context.open && context.targetDomain === "Worktrees" && projectId !== null;
  const worktrees = useWorktrees(projectId, { enabled });
  if (context.signals === null) return EMPTY_SECTION("Worktrees");
  const all = context.scope.kind === "all";
  const rows = all ? catalogs.worktrees : (worktrees.data?.worktrees ?? []);
  return section(
    "Worktrees",
    rows.map(worktree =>
      worktreeRowSeed(
        worktree,
        workspaceLabel(context.scope, worktree.workspace_id, context.workspaceNames)
      )
    ),
    all ? catalogs.worktreeState : worktrees,
    paletteDomainEnabled(context, "Worktrees") && context.scope.kind !== "user",
    context.query,
    context.signals,
    {
      limit: context.domainLimit,
      catalogTotal: all ? catalogs.worktreeTotal : worktrees.data?.worktrees.length,
    }
  );
}

/**
 * Agents: a project's, or the user-layer agents (`workspace` omitted) — alone
 * under User, joined by every project's under All projects.
 */
function useAgentSection(context: OsPaletteDomainContext, catalogs: OsPaletteWorkspaceCatalogs) {
  const all = context.scope.kind === "all";
  const project = context.scope.kind === "project";
  const userLayerAgents = useAgents(null, {
    enabled: paletteDomainEnabled(context, "Agents") && !project,
  });
  const workspaceAgents = useAgents(context.scopedWorkspace, {
    enabled: paletteDomainEnabled(context, "Agents") && project,
  });
  if (context.signals === null) return EMPTY_SECTION("Agents");
  const rows = !project
    ? [
        ...(userLayerAgents.data ?? []).map(agent => ({
          agent,
          workspaceId: agent.workspace_id,
        })),
        ...(all ? catalogs.workspaceAgents : []),
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
    all
      ? {
          isLoading: userLayerAgents.isLoading || catalogs.workspaceAgentState.isLoading,
          isError: userLayerAgents.isError || catalogs.workspaceAgentState.isError,
          error: userLayerAgents.error ?? catalogs.workspaceAgentState.error,
        }
      : project
        ? workspaceAgents
        : userLayerAgents,
    paletteDomainEnabled(context, "Agents"),
    context.query,
    context.signals,
    { limit: context.domainLimit, catalogTotal: rows.length }
  );
}

function useTaskSection(context: OsPaletteDomainContext) {
  const enabled = paletteDomainEnabled(context, "Tasks");
  const tasks = useTasks(paletteTaskFilters(context.scope), { enabled });
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

/** Loops: the focused project's or User's (`@user`), or every project's under All projects. */
function useLoopSection(context: OsPaletteDomainContext, catalogs: OsPaletteWorkspaceCatalogs) {
  const all = context.scope.kind === "all";
  const workspaceEnabled =
    paletteDomainEnabled(context, "Loops") && Boolean(context.scopedWorkspace);
  const globalEnabled = paletteDomainEnabled(context, "Loops") && all;
  const loops = useLoops(context.scopedWorkspace ?? "", {}, workspaceEnabled);
  usePaletteInfiniteCatalog(loops, workspaceEnabled);
  if (context.signals === null) return EMPTY_SECTION("Loops");
  // A User loop opens in place on the User desktop: it carries no project to switch to.
  const ownerWorkspaceId = context.scope.kind === "project" ? context.scope.workspaceId : null;
  const rows = all
    ? catalogs.loops
    : loops.loops.map(loop => ({ loop, workspaceId: ownerWorkspaceId }));
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
    all ? catalogs.loopState : loops,
    workspaceEnabled || globalEnabled,
    context.query,
    context.signals,
    {
      limit: context.domainLimit,
      catalogTotal: all ? catalogs.loopTotal : loops.total,
    }
  );
}

function useAutomationSection(context: OsPaletteDomainContext) {
  const enabled = paletteDomainEnabled(context, "Automations");
  const filters = paletteWorkspaceCatalogFilters(context.scope);
  const jobs = useAutomationJobs(filters, { enabled });
  const triggers = useAutomationTriggers(filters, { enabled });
  const timeZone = useAutomationTimeZone();
  // `context.workspaceNames` is an All projects label index; the sentence names the
  // project in every scope, exactly as the listing does.
  const { registeredWorkspaces } = useActiveWorkspace();
  usePaletteInfiniteCatalog(jobs, enabled);
  usePaletteInfiniteCatalog(triggers, enabled);
  if (context.signals === null) return EMPTY_SECTION("Automations");
  const views = paletteAutomationViews(jobs.jobs, triggers.triggers, {
    timeZone,
    names: context.workspaceNames,
    projects: registeredWorkspaces,
  });
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
