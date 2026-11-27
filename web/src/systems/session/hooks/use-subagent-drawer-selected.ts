import { use } from "react";
import { useSelector } from "@xstate/store-react";
import { useCurrentComputerId } from "@/hooks/use-current-computer-id";
import { normalizeComputerId, type ComputerId } from "@/lib/computer-scope";
import { SubagentDrawerContext } from "../contexts/subagent-drawer-context";
import { SessionSubagentsContext } from "../contexts/session-subagents-context-value";
import { sessionWindowDrawerLogic } from "../contexts/session-window-drawer-store";

const emptyDrawerStore = sessionWindowDrawerLogic.createStore();

export function useSubagentDrawerSelected(
  id: string,
  workspaceId?: string,
  owner?: ComputerId
): boolean {
  const drawer = use(SubagentDrawerContext);
  const computerId = useCurrentComputerId(owner);
  const transcript = use(SessionSubagentsContext);
  const workspace = workspaceId ?? transcript.workspaceId;
  return useSelector(drawer?.store ?? emptyDrawerStore, snapshot => {
    const selected = snapshot.context.selected;
    return (
      selected?.subagent.id === id &&
      normalizeComputerId(selected.computerId) === computerId &&
      (workspace === "" || selected.workspaceId === workspace)
    );
  });
}
