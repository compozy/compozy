/** How long a restored offset waits for content that can hold it; past this it is let go. */
export const SETTLE_MS = 1_000;

interface Target {
  top: number;
  left: number;
}

interface Settling {
  target: Target;
  /** Whose restore this is: a gesture in that pane, or the pane hiding, lets it go. */
  owner: object;
  timer: ReturnType<typeof setTimeout>;
  /** The content elements observed for growth, released with the entry. */
  content: Element[];
}

/** Whether `current` is an offset nobody chose: dropped to 0, or clamped short of the target. */
function dropped(current: number, target: number, max: number): boolean {
  if (target === 0 || Math.abs(current - target) < 1) return false;
  return current === 0 || (current < target && current >= max - 1);
}

/**
 * Restored scroll offsets settling into content that is still being laid out. A container whose
 * content was not rendered while its pane was hidden (a virtualized list that unmounted its rows,
 * a code view that measured its lines at 0) cannot hold its offset at the moment the pane shows
 * again: the write clamps, and the content grows a frame or more later. The settler keeps such
 * an offset and writes it again when the container's content changes size (a resize observer on
 * its children, so the write lands before that frame paints) or when the container's own logic
 * moves it back to 0, which a library re-reading its hidden state does. It never polls, and
 * each entry ends at the first of: the operator's gesture in the pane, the pane hiding, the
 * container scrolled to a position that is not a dropped one (the app's own move), or
 * {@link SETTLE_MS}.
 */
export class PaneScrollSettler {
  private readonly settling = new Map<Element, Settling>();
  private readonly containerOf = new WeakMap<Element, Element>();
  private observer: ResizeObserver | null = null;

  /**
   * Writes a restored offset the browser dropped (the container reads 0 on that axis) and keeps
   * it settling for a while, so content that grows or resets after the write still gets it.
   */
  restore(container: Element, target: Target, owner: object): void {
    this.release(container);
    if (target.top === 0 && target.left === 0) return;
    if (container.scrollTop === 0 && target.top !== 0) container.scrollTop = target.top;
    if (container.scrollLeft === 0 && target.left !== 0) container.scrollLeft = target.left;
    this.track(container, target, owner);
  }

  /**
   * A scroll on a settling container: a dropped offset is written again and the event is the
   * settler's; any other position is the app's or the operator's, and ends the settling.
   * Returns whether the scroll was a dropped one (not to be recorded as the container's offset).
   */
  scrolled(container: Element): boolean {
    const entry = this.settling.get(container);
    if (entry === undefined) return false;
    if (this.droppedOffset(container, entry.target)) {
      this.write(container, entry.target);
      return true;
    }
    this.release(container);
    return false;
  }

  /** Lets go of every offset `owner` restored. */
  releaseOwner(owner: object): void {
    for (const [container, entry] of this.settling) {
      if (entry.owner === owner) this.release(container);
    }
  }

  dispose(): void {
    for (const container of this.settling.keys()) this.release(container);
    this.observer?.disconnect();
    this.observer = null;
  }

  private track(container: Element, target: Target, owner: object): void {
    const timer = setTimeout(() => this.release(container), SETTLE_MS);
    const content = [...container.children];
    this.settling.set(container, { target, owner, timer, content });
    const observer = this.ensureObserver();
    for (const child of content) {
      this.containerOf.set(child, container);
      observer?.observe(child);
    }
  }

  private ensureObserver(): ResizeObserver | null {
    if (this.observer !== null || typeof ResizeObserver === "undefined") return this.observer;
    this.observer = new ResizeObserver(entries => {
      const grown = new Set<Element>();
      for (const entry of entries) {
        const container = this.containerOf.get(entry.target);
        if (container !== undefined) grown.add(container);
      }
      for (const container of grown) {
        const settling = this.settling.get(container);
        if (settling === undefined) continue;
        if (!container.isConnected || container.closest("[hidden]") !== null) continue;
        if (this.droppedOffset(container, settling.target)) this.write(container, settling.target);
      }
    });
    return this.observer;
  }

  private droppedOffset(container: Element, target: Target): boolean {
    const maxTop = container.scrollHeight - container.clientHeight;
    const maxLeft = container.scrollWidth - container.clientWidth;
    return (
      dropped(container.scrollTop, target.top, maxTop) ||
      dropped(container.scrollLeft, target.left, maxLeft)
    );
  }

  private write(container: Element, target: Target): void {
    if (Math.abs(container.scrollTop - target.top) >= 1) container.scrollTop = target.top;
    if (Math.abs(container.scrollLeft - target.left) >= 1) container.scrollLeft = target.left;
  }

  private release(container: Element): void {
    const entry = this.settling.get(container);
    if (entry === undefined) return;
    clearTimeout(entry.timer);
    for (const child of entry.content) {
      this.observer?.unobserve(child);
      this.containerOf.delete(child);
    }
    this.settling.delete(container);
  }
}
