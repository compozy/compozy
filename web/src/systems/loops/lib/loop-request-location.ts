export interface LoopRequestLocationTarget {
  workspaceId: string;
  runId: string;
  nodeId: string;
  itemIndex: number;
}

export function loopRequestLocation(target: LoopRequestLocationTarget) {
  return {
    pathname: `/loop-runs/${encodeURIComponent(target.runId)}`,
    search: {
      workspace: target.workspaceId,
      request_node: target.nodeId,
      request_item: target.itemIndex,
    },
  };
}

export function loopRequestLocationPath(target: LoopRequestLocationTarget): string {
  const location = loopRequestLocation(target);
  const search = new URLSearchParams({
    workspace: location.search.workspace,
    request_node: location.search.request_node,
    request_item: String(location.search.request_item),
  });
  return `${location.pathname}?${search}`;
}
