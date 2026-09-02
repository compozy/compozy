import { useEffect, useEffectEvent } from "react";
import { toast } from "sonner";

import { definitionToGraph } from "../lib/codec";
import { editorDefinitionFromLoop } from "../lib/loop-editor-definition";
import { layoutEditorGraph } from "../lib/loop-editor-layout";
import type { LoopDetail } from "../types";
import type { useLoopEditorState } from "./use-loop-editor-state";
import type { useLoop, useLoopAnnotations } from "./use-loops";

interface UseLoopEditorSynchronizationOptions {
  annotationsQuery: ReturnType<typeof useLoopAnnotations>;
  initializedSourceKey: string | null;
  loopQuery: ReturnType<typeof useLoop>;
  name: string;
  onPublished?: (loop: LoopDetail) => void;
  store: ReturnType<typeof useLoopEditorState>["store"];
  workspaceId: string;
}

export function useLoopEditorSynchronization({
  annotationsQuery,
  initializedSourceKey,
  loopQuery,
  name,
  onPublished,
  store,
  workspaceId,
}: UseLoopEditorSynchronizationOptions) {
  const handlePublished = useEffectEvent((loop: LoopDetail) => onPublished?.(loop));

  useEffect(() => {
    const loop = loopQuery.data;
    if (!loop || annotationsQuery.isLoading) return;
    const definition = editorDefinitionFromLoop(loop);
    const key = `${workspaceId}:${name}:${loop.source}`;
    if (initializedSourceKey === key) return;
    const graph = definitionToGraph(definition);
    const nodes = layoutEditorGraph(graph.nodes, graph.edges, annotationsQuery.data ?? []);
    store.trigger.draftInitialized({ definition, edges: graph.edges, nodes, sourceKey: key });
  }, [
    loopQuery.data,
    annotationsQuery.data,
    annotationsQuery.isLoading,
    workspaceId,
    name,
    initializedSourceKey,
    store,
  ]);

  useEffect(() => {
    store.trigger.annotationsStatusObserved({
      failed: annotationsQuery.isError,
      notify: () => toast.error("Could not load saved node positions — using auto-layout."),
    });
  }, [annotationsQuery.isError, store]);

  useEffect(() => {
    const published = store.on("publishCompleted", event => handlePublished(event.loop));
    return () => published.unsubscribe();
  }, [store]);

  useEffect(() => () => store.trigger.lifecycleDisposed(), [store]);
}
