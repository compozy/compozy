import { use, useEffect, useSyncExternalStore } from "react";

import { PreviewQueueContext } from "#gallery/shell/flows/preview-store";

/** Whether this node's preview is mounted, and how it reports that it finished loading. */
export function usePreview(id: string, src: string) {
  const queue = use(PreviewQueueContext);
  if (!queue) throw new Error("usePreview needs a PreviewQueue");
  useEffect(() => queue.register(id, src), [queue, id, src]);
  const mounted = useSyncExternalStore(queue.subscribe, () => queue.mounted(id, src));
  return { mounted, onSettle: () => queue.settled(id, src) };
}
