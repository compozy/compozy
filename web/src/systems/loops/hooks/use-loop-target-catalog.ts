import { useLoops } from "./use-loops";
import {
  projectLoopTargetCatalog,
  type LoopAutomationStartKind,
  type LoopTargetCatalog,
} from "../lib/loop-target-availability";

export function useLoopTargetCatalog(
  workspaceId: string,
  selectedName: string,
  requiredStartKind: LoopAutomationStartKind
): LoopTargetCatalog {
  const catalogQuery = useLoops(workspaceId, { limit: 50, sort: "name" }, workspaceId !== "");
  const loadedSelection = catalogQuery.loops.find(loop => loop.name === selectedName) ?? null;
  const shouldLoadSelection = workspaceId !== "" && selectedName !== "" && loadedSelection === null;
  const selectionQuery = useLoops(
    workspaceId,
    { limit: 50, q: selectedName, sort: "name" },
    shouldLoadSelection
  );
  const selected =
    loadedSelection ?? selectionQuery.loops.find(loop => loop.name === selectedName) ?? null;

  return projectLoopTargetCatalog({
    error: catalogQuery.error ?? selectionQuery.error ?? null,
    fetchNextPage: () => {
      void catalogQuery.fetchNextPage();
    },
    hasNextPage: Boolean(catalogQuery.hasNextPage),
    isFetchingNextPage: catalogQuery.isFetchingNextPage,
    isLoading: catalogQuery.isLoading || (shouldLoadSelection && selectionQuery.isLoading),
    loops: catalogQuery.loops,
    requiredStartKind,
    selected,
    selectedName,
  });
}
