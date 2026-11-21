import { afterEach, describe, expect, it } from "vitest";

import { isRenderedAtScroll, trackRenderedOnScroll } from "../scroll-owner";

// Invariant (#1546): a scroll listener on a tracked element reads whether the element was rendered
// for the event being dispatched, whatever order it registered in relative to the tracking; once the
// last reader releases, nothing keeps a stale answer. Owning layer: the shared walk in
// `scroll-owner.ts`; the transcript's readers are covered in `session-thread.test.tsx`.
describe("trackRenderedOnScroll", () => {
  const pane = document.createElement("div");
  const scroller = document.createElement("div");
  pane.append(scroller);

  afterEach(() => {
    pane.hidden = false;
    pane.remove();
  });

  function readOnScroll(): boolean[] {
    const reads: boolean[] = [];
    const listener = () => reads.push(isRenderedAtScroll(scroller));
    scroller.addEventListener("scroll", listener);
    return reads;
  }

  it("Should answer for the scroll being dispatched to a listener registered before the tracking", () => {
    document.body.append(pane);
    const reads = readOnScroll();
    const release = trackRenderedOnScroll(scroller);

    scroller.dispatchEvent(new Event("scroll"));
    pane.hidden = true;
    scroller.dispatchEvent(new Event("scroll"));
    pane.hidden = false;
    scroller.dispatchEvent(new Event("scroll"));
    release();

    expect(reads).toEqual([true, false, true]);
  });

  it("Should keep the shared walk until the last reader releases", () => {
    document.body.append(pane);
    const reads = readOnScroll();
    const releaseFirst = trackRenderedOnScroll(scroller);
    const releaseSecond = trackRenderedOnScroll(scroller);

    releaseFirst();
    releaseFirst();
    pane.hidden = true;
    scroller.dispatchEvent(new Event("scroll"));
    pane.hidden = false;
    // The remaining reader still holds the walk: the answer is the one its scroll event got.
    expect(isRenderedAtScroll(scroller)).toBe(false);
    releaseSecond();

    expect(reads).toEqual([false]);
    expect(isRenderedAtScroll(scroller)).toBe(true);
  });

  it("Should not walk for scrolls of a scroller nested in the tracked element", () => {
    document.body.append(pane);
    const nested = document.createElement("div");
    scroller.append(nested);
    const release = trackRenderedOnScroll(scroller);

    pane.hidden = true;
    nested.dispatchEvent(new Event("scroll"));
    pane.hidden = false;
    const answer = isRenderedAtScroll(scroller);
    release();
    nested.remove();

    expect(answer).toBe(true);
  });
});
