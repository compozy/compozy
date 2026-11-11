// Page behaviours ported from ManualLayout.astro and Base.astro inline scripts:
// reveal-on-scroll for [data-reveal], copy buttons ([data-copy], [data-copy-from]),
// and the in-page index scroll-spy.
import { useEffect, useState } from "react";

import type { Chapter } from "../chapters";

function useRevealOnScroll() {
  useEffect(() => {
    const io = new IntersectionObserver(
      entries => {
        for (const e of entries) {
          if (!e.isIntersecting) continue;
          e.target.classList.add("is-in");
          io.unobserve(e.target);
        }
      },
      { rootMargin: "0px 0px -8% 0px" }
    );
    for (const el of document.querySelectorAll("[data-reveal]")) io.observe(el);
    return () => io.disconnect();
  }, []);
}

/** Shows feedback in place of the button's content, then puts the original nodes back (swatches keep their markup). */
function flash(btn: HTMLElement, text: string) {
  if (btn.dataset.flashing) return;
  const original = Array.from(btn.childNodes);
  btn.dataset.flashing = "true";
  btn.replaceChildren(text);
  window.setTimeout(() => {
    btn.replaceChildren(...original);
    delete btn.dataset.flashing;
  }, 1200);
}

function useCopyButtons() {
  useEffect(() => {
    const onClick = (event: MouseEvent) => {
      const btn = (event.target as Element | null)?.closest<HTMLElement>(
        "button[data-copy], button[data-copy-from]"
      );
      if (!btn) return;
      const from = btn.dataset.copyFrom ? document.getElementById(btn.dataset.copyFrom) : null;
      const text = from ? from.innerText : (btn.dataset.copy ?? "");
      if (!navigator.clipboard) return flash(btn, "select and copy");
      navigator.clipboard.writeText(text).then(
        () => flash(btn, "copied ✓"),
        () => flash(btn, "select and copy")
      );
    };
    document.addEventListener("click", onClick);
    return () => document.removeEventListener("click", onClick);
  }, []);
}

/** Returns the id of the section the reader is in (scroll-spy for the in-page index). */
function useActiveSection(page: Chapter) {
  const [active, setActive] = useState<string | null>(null);
  useEffect(() => {
    const spy = () => {
      const y = window.scrollY + 200;
      let current: string | null = null;
      for (const [id] of page.sections) {
        const el = document.getElementById(id);
        if (el && el.offsetTop <= y) current = id;
      }
      setActive(current);
    };
    spy();
    window.addEventListener("scroll", spy, { passive: true });
    return () => window.removeEventListener("scroll", spy);
  }, [page]);
  return active;
}

export function useManualBehaviors(page: Chapter) {
  useRevealOnScroll();
  useCopyButtons();
  return useActiveSection(page);
}
