/**
 * The canvas's preview queue (`preview-queue.ts`) as a small store. Prototype nodes register
 * their preview source and read whether their preview is mounted; external nodes never register,
 * so they never become iframes. The plan changes only once a pan or zoom has settled for
 * `settleDelayMs`, never mid-gesture or in a brief pause between drags, and a preview that
 * reports ready or fails lets the next queued one start.
 */
import { useState, type ReactNode } from "react";
import { useOnViewportChange, useStoreApi } from "@xyflow/react";

import { createPreviewQueue, PreviewQueueContext } from "#gallery/shell/flows/preview-store";
import { previewView } from "#gallery/shell/flows/preview-queue";

/** Plans the previews of the canvas inside it; `ready` once the layout has placed the nodes. */
export function PreviewQueue({ ready, children }: { ready: boolean; children: ReactNode }) {
  const flowStore = useStoreApi();
  const [queue] = useState(createPreviewQueue);

  const settle = () => {
    if (!ready) return;
    const { transform, width, height, nodeLookup } = flowStore.getState();
    const nodes = [...nodeLookup.values()].map(node => ({
      id: node.id,
      x: node.internals.positionAbsolute.x,
      y: node.internals.positionAbsolute.y,
      width: node.measured.width ?? 0,
      height: node.measured.height ?? 0,
    }));
    const [x, y, zoom] = transform;
    queue.settle(previewView(nodes, { x, y, zoom, width, height }));
  };
  useOnViewportChange({ onStart: () => queue.moveStart(), onEnd: settle });

  return <PreviewQueueContext value={queue}>{children}</PreviewQueueContext>;
}
