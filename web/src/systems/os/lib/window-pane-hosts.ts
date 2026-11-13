/**
 * One DOM host per window, owned by a desktop's tab layer. A window's pane renders into its host
 * through a portal whose container never changes, and the split slot the window occupies adopts
 * the host node. Moving a window between tabs or positions in a split then moves a DOM node; the
 * pane, its app state and its data stay mounted instead of remounting under a new React parent.
 */
export class WindowPaneHosts {
  private readonly hosts = new Map<string, HTMLElement>();

  /** The host for a window, created on first use; a `display: contents` box adds no layout. */
  hostOf(windowId: string): HTMLElement {
    let host = this.hosts.get(windowId);
    if (host === undefined) {
      host = document.createElement("div");
      host.style.display = "contents";
      host.dataset.slot = "os-window-host";
      host.dataset.windowId = windowId;
      this.hosts.set(windowId, host);
    }
    return host;
  }

  /** Drops the hosts of windows the layer no longer renders. */
  retain(windowIds: ReadonlySet<string>): void {
    for (const windowId of this.hosts.keys()) {
      if (!windowIds.has(windowId)) this.hosts.delete(windowId);
    }
  }
}
