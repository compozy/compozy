import type { ShownWindow } from "../lib/window-manager-navigation";
import type { LayoutNodeId, WindowId } from "../lib/window-manager-types";

interface PendingSwap {
  first: WindowId;
  second: WindowId;
  /** The leaf `first` stood in when the swap was sent. */
  firstNode: LayoutNodeId;
}

/**
 * Swaps this client sent that the shown layout does not reflect yet, oldest first, so directional
 * input pressed before an answer reads where the windows are going.
 *
 * A swap exchanges the windows of two leaves and keeps the leaves, so whether one already shows is
 * read by leaf identity: while `first` still stands in the leaf it left, the swap is pending. That
 * holds through work-area resizes, and a swap the answer or a stream update already shows adds
 * nothing even before its completion settles.
 */
export class PendingSwaps {
  private entries: PendingSwap[] = [];

  /**
   * Records a sent swap against `shown`, the layout with the earlier pending swaps applied, and
   * returns the release to call when it settles. A swap whose first window is not shown in the
   * active tab is not tracked.
   */
  record(first: WindowId, second: WindowId, shown: readonly ShownWindow[]): (() => void) | null {
    const firstNode = shown.find(window => window.id === first)?.nodeId;
    if (firstNode === undefined) return null;
    const entry: PendingSwap = { first, second, firstNode };
    this.entries.push(entry);
    return () => {
      this.entries = this.entries.filter(candidate => candidate !== entry);
    };
  }

  /** Forgets every entry: a later command changes the layout in ways this client cannot predict. */
  clear(): void {
    this.entries = [];
  }

  /** `shown` with every pending swap applied in the order it was sent. */
  apply(shown: readonly ShownWindow[]): ShownWindow[] {
    let windows = [...shown];
    for (const swap of this.entries) {
      const first = windows.find(window => window.id === swap.first);
      const second = windows.find(window => window.id === swap.second);
      if (first === undefined || second === undefined || first.nodeId !== swap.firstNode) continue;
      windows = windows.map(window => {
        if (window.id === first.id) return { ...window, rect: second.rect, nodeId: second.nodeId };
        if (window.id === second.id) return { ...window, rect: first.rect, nodeId: first.nodeId };
        return window;
      });
    }
    return windows;
  }
}
