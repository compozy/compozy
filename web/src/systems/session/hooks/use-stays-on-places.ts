import { GLOBAL_SCOPE, type AgentPlaceOption } from "@/systems/agent";
import { useActiveWorkspace } from "@/systems/workspace";

import {
  prefillSessionProject,
  readLastSessionProject,
  sessionProjectOptions,
} from "../lib/session-create-target";

/**
 * Where a stays-on agent can live when the list spans several places: Global and
 * every project, preselected like New session (the remembered focus, then the
 * last project used from All projects, then the first project).
 */
export function useStaysOnPlaces(): {
  places: AgentPlaceOption[];
  preselected: string;
  /** The project catalog hasn't answered: the places and the preselection aren't final yet. */
  pending: boolean;
} {
  const active = useActiveWorkspace();
  const options = sessionProjectOptions(active.projectWorkspaces);
  const focusedBinding = active.focus.kind === "project" ? active.focus.workspaceId : null;
  return {
    places: options.map(option => ({
      id: option.value,
      name: option.value === GLOBAL_SCOPE ? "Global" : option.label,
    })),
    preselected: prefillSessionProject(options, {
      focusedBinding,
      lastUsed: readLastSessionProject(),
    }),
    pending: active.pending,
  };
}
