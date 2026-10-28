import { use } from "react";
import { useSelector } from "@xstate/store-react";
import { SubagentDrawerContext, subagentDrawerLogic } from "../contexts/subagent-drawer-context";

const emptyDrawerStore = subagentDrawerLogic.createStore();

export function useSubagentDrawerSelected(id: string): boolean {
  const drawer = use(SubagentDrawerContext);
  return useSelector(
    drawer?.store ?? emptyDrawerStore,
    snapshot => snapshot?.context.selected?.subagent.id === id
  );
}
