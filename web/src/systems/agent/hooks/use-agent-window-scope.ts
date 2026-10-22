import {
  USER_SCOPE_SELECTOR,
  useActiveWorkspace,
  useWindowBindingSearch,
  useWindowDataWorkspace,
} from "@/systems/workspace";

/**
 * The agent scope of the window rendering this subtree: its binding (an agent
 * opened in place from All projects), else its desktop. Reads go to the
 * project, User (`@user`), or the user layer under All projects.
 */
export function useAgentWindowScope() {
  const scope = useWindowDataWorkspace();
  const bindingSearch = useWindowBindingSearch();
  const { registeredWorkspaces } = useActiveWorkspace();
  const projectId = scope?.kind === "project" ? scope.workspaceId : null;
  const workspace = projectId ?? (scope?.kind === "user" ? USER_SCOPE_SELECTOR : null);
  return {
    /** `null` while the destination is unsettled. */
    scope,
    /** The workspace selector agent reads send: a project id, `@user`, or none (user layer). */
    workspace,
    /** The owning project for project-only actions (delete, providers, saves). */
    projectId,
    projectName: registeredWorkspaces.find(item => item.id === projectId)?.name ?? null,
    /** Spread into in-window navigations so the window keeps its binding. */
    bindingSearch,
  };
}
