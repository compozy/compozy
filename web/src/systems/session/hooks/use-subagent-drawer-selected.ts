import { use } from "react";
import { useSelector } from "@xstate/store-react";
import { SubagentDrawerContext } from "../contexts/subagent-drawer-context";
import { sessionWindowDrawerLogic } from "../contexts/session-window-drawer-store";

const emptyDrawerStore = sessionWindowDrawerLogic.createStore();

export function useSubagentDrawerSelected(id: string): boolean {
  const drawer = use(SubagentDrawerContext);
  return useSelector(
    drawer?.store ?? emptyDrawerStore,
    snapshot => snapshot?.context.selected?.subagent.id === id
  );
}
