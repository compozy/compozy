import { AGENT_PLACE_LATER } from "./agent-create-draft";
import { GLOBAL_SCOPE } from "./agent-instance";

/** What Where it works calls a place: Global, Decide later, or the project's name. */
export function agentPlaceLabel(
  place: string,
  workspaces: ReadonlyArray<{ id: string; name: string }>
) {
  if (place === GLOBAL_SCOPE) return "Global";
  if (place === AGENT_PLACE_LATER) return "Decide later";
  return workspaces.find(workspace => workspace.id === place)?.name ?? place;
}
