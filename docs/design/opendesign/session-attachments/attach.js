/* Session attachments — one-row rail + edge fades.
   Hydrates .att-strip / .msg-user__atts (workspaces overview pattern).
   No pointer-drag: the composer still accepts file drops. */
(() => {
  const RAIL = ".att-strip, .msg-user__atts";

  function reducedMotion() {
    return matchMedia("(prefers-reduced-motion: reduce)").matches;
  }
  function trackOf(rail) {
    return rail.querySelector(":scope > .att-rail__track");
  }
  function fadePad(rail) {
    const n = parseFloat(getComputedStyle(rail).getPropertyValue("--att-fade"));
    return Number.isFinite(n) ? n : 56;
  }

  function hydrate(rail) {
    if (trackOf(rail)) return;
    const track = document.createElement("div");
    track.className = "att-rail__track";
    while (rail.firstChild) track.appendChild(rail.firstChild);
    const start = document.createElement("span");
    start.className = "att-rail__edge att-rail__edge--start";
    start.setAttribute("aria-hidden", "true");
    const end = document.createElement("span");
    end.className = "att-rail__edge att-rail__edge--end";
    end.setAttribute("aria-hidden", "true");
    rail.append(start, track, end);
  }

  function paint(rail) {
    const track = trackOf(rail);
    if (!track) return;
    const max = track.scrollWidth - track.clientWidth;
    if (max <= 2) {
      rail.dataset.overflow = "none";
      return;
    }
    const bits = [];
    if (track.scrollLeft > 2) bits.push("start");
    if (track.scrollLeft < max - 2) bits.push("end");
    rail.dataset.overflow = bits.length ? bits.join(" ") : "none";
  }

  function applyScrollHint(rail) {
    const hint = rail.getAttribute("data-scroll");
    const track = trackOf(rail);
    if (!hint || !track) return;
    const max = Math.max(0, track.scrollWidth - track.clientWidth);
    if (hint === "mid") track.scrollLeft = max / 2;
    else if (hint === "end") track.scrollLeft = max;
    else track.scrollLeft = 0;
  }

  function keepInView(rail, el) {
    const track = trackOf(rail);
    if (!track) return;
    const pad = fadePad(rail);
    const r = el.getBoundingClientRect();
    const t = track.getBoundingClientRect();
    let delta = 0;
    if (r.left < t.left + pad) delta = r.left - (t.left + pad);
    else if (r.right > t.right - pad) delta = r.right - (t.right - pad);
    if (Math.abs(delta) < 1) {
      paint(rail);
      return;
    }
    track.scrollTo({
      left: track.scrollLeft + delta,
      behavior: reducedMotion() ? "auto" : "smooth",
    });
  }

  function whenImagesReady(rail, fn) {
    const imgs = Array.from(rail.querySelectorAll("img"));
    let pending = imgs.filter((img) => !img.complete).length;
    if (pending === 0) {
      requestAnimationFrame(fn);
      return;
    }
    const tick = () => {
      pending -= 1;
      if (pending <= 0) fn();
    };
    imgs.forEach((img) => {
      if (img.complete) return;
      img.addEventListener("load", tick, { once: true });
      img.addEventListener("error", tick, { once: true });
    });
  }

  function bind(rail) {
    hydrate(rail);
    const track = trackOf(rail);
    if (!track) return;
    paint(rail);
    whenImagesReady(rail, () => {
      applyScrollHint(rail);
      paint(rail);
    });
    track.addEventListener("scroll", () => paint(rail), { passive: true });
    track.addEventListener(
      "wheel",
      (e) => {
        if (track.scrollWidth <= track.clientWidth + 2) return;
        if (Math.abs(e.deltaY) <= Math.abs(e.deltaX)) return;
        e.preventDefault();
        track.scrollLeft += e.deltaY;
      },
      { passive: false },
    );
    rail.addEventListener("focusin", (e) => {
      const item = e.target.closest(".att-tile, .att-frame, .att-file");
      if (item && rail.contains(item)) keepInView(rail, item);
    });
    if (typeof ResizeObserver !== "undefined") {
      const ro = new ResizeObserver(() => paint(rail));
      ro.observe(track);
    }
  }

  function mount() {
    document.querySelectorAll(RAIL).forEach(bind);
    window.addEventListener("resize", () => {
      document.querySelectorAll(RAIL).forEach(paint);
    });
  }

  document.addEventListener("click", (e) => {
    const btn = e.target.closest(".att-x");
    if (!btn) return;
    const tile = btn.closest(".att-tile");
    const rail = tile && tile.closest(".att-strip");
    if (!tile || !rail) return;
    tile.remove();
    if (!rail.querySelector(".att-tile")) {
      rail.remove();
      return;
    }
    paint(rail);
  });

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", mount);
  } else {
    mount();
  }
})();
