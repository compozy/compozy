// The motion demos of chapter 09. Every stage renders its final frame, so prerendered HTML and
// reduced motion read correctly; a demo rewinds its stage to the start and plays it with the Web
// Animations API. `useMotionDemo` owns when a demo plays (scroll, replay, reduced motion).
import { bez, motion } from "./data";

export type DemoId =
  | "converge"
  | "sweep"
  | "lane-grow"
  | "state-change"
  | "log-line"
  | "handoff"
  | "face-state"
  | "card-move"
  | "dock-answer"
  | "counter"
  | "words"
  | "scramble"
  | "lines"
  | "durations"
  | "stagger";

/** False once a newer run (replay) or unmount superseded this run: pending timers stop. */
export type Alive = () => boolean;

const D = motion.durations;
const S = motion.stagger;
const E = {
  out: bez(motion.eases.out),
  inOut: bez(motion.eases.inOut),
};

const q = (r: ParentNode, s: string) => r.querySelector<HTMLElement>(s);
const qa = (r: ParentNode, s: string) => [...r.querySelectorAll<HTMLElement>(s)];

interface TweenOptions {
  delay?: number;
  easing?: string;
  fill?: FillMode;
}
const tw = (el: Element | null, kf: Keyframe[], d: number, o: TweenOptions = {}) =>
  el?.animate(kf, {
    duration: d,
    delay: o.delay ?? 0,
    easing: o.easing ?? E.out,
    fill: o.fill ?? "backwards",
  });

/** opacity 1 inside the window [a,b] in ms, with enter-length crossfades at each edge */
const fadeSeq = (el: Element | null, [a, b]: [number, number], total: number) => {
  const o = (t: number) => Math.min(1, t / total);
  const kf: Keyframe[] =
    a === 0
      ? [{ opacity: 1, offset: 0 }]
      : [
          { opacity: 0, offset: 0 },
          { opacity: 0, offset: o(a) },
          { opacity: 1, offset: o(a + D.enter) },
        ];
  if (b < total) kf.push({ opacity: 1, offset: o(b) }, { opacity: 0, offset: o(b + D.enter) });
  if (b < total) kf.push({ opacity: 0, offset: 1 });
  else if (kf.at(-1)?.offset !== 1) kf.push({ opacity: 1, offset: 1 });
  return el?.animate(kf, { duration: total, fill: "backwards" });
};

/** The brand cyan, resolved from the @compozy/ui token for keyframes that cannot read var(). */
const cyan = (r: Element) =>
  getComputedStyle(r).getPropertyValue("--color-brand-cyan").trim() || "currentColor";

const demos: Record<DemoId, (r: HTMLElement, alive: Alive) => void> = {
  converge: r => {
    // particles start scattered (data-dx / data-dy in px) and end on the stream line
    qa(r, "[data-pt]").forEach((p, i) => {
      const dx = Number(p.dataset.dx);
      const dy = Number(p.dataset.dy);
      tw(
        p,
        [
          { transform: `translate(${dx}px,${dy}px)`, opacity: 0.35 },
          { transform: "translate(0,0)", opacity: 1 },
        ],
        D.handoff,
        { easing: E.inOut, delay: i * 12 }
      );
    });
    tw(
      q(r, "[data-stream]"),
      [{ opacity: 0 }, { opacity: 0, offset: 0.5 }, { opacity: 1 }],
      D.handoff,
      {
        easing: E.inOut,
      }
    );
  },
  sweep: r => {
    tw(
      q(r, "[data-lit]"),
      [
        { maskPosition: "0% 0", opacity: 0 },
        { opacity: 1, offset: 0.3 },
        { opacity: 1, offset: 0.7 },
        { maskPosition: "100% 0", opacity: 0 },
      ],
      D.sweep,
      { easing: E.inOut, fill: "both" }
    );
  },
  "lane-grow": (r, alive) => {
    const label = q(r, "[data-label]");
    tw(
      q(r, "[data-bar]"),
      [
        { transform: "scaleX(0.55)", opacity: 0.6 },
        { transform: "scaleX(1)", opacity: 1 },
      ],
      D.handoff
    );
    if (!label) return;
    label.textContent = "running";
    window.setTimeout(() => {
      if (alive()) label.textContent = "running · 4 m";
    }, D.handoff);
  },
  "state-change": r => {
    const T = 2200;
    for (const attr of ["s", "p"]) {
      fadeSeq(q(r, `[data-${attr}="idle"]`), [0, 600], T);
      fadeSeq(q(r, `[data-${attr}="running"]`), [600, 1800], T);
      fadeSeq(q(r, `[data-${attr}="done"]`), [1800, T], T);
    }
  },
  "log-line": r => {
    qa(r, "[data-ll]").forEach((l, i) =>
      tw(
        l,
        [
          { opacity: 0, transform: "translateY(6px)" },
          { opacity: 1, transform: "none" },
        ],
        D.enter,
        { delay: 300 + i * 700 }
      )
    );
  },
  handoff: r => {
    tw(
      q(r, "[data-pk]"),
      [
        { left: "0%", opacity: 1 },
        { left: "100%", opacity: 1, offset: 0.9 },
        { left: "100%", opacity: 0 },
      ],
      D.handoff,
      { easing: E.inOut }
    );
    tw(q(r, "[data-a]"), [{ opacity: 1 }, { opacity: 0.35 }], D.handoff, { easing: E.inOut });
    tw(
      q(r, "[data-b]"),
      [{ opacity: 0.35 }, { opacity: 0.35, offset: 0.6 }, { opacity: 1 }],
      D.handoff,
      {
        easing: E.inOut,
      }
    );
  },
  "face-state": r => {
    const T = 1400;
    for (const attr of ["fs", "fl"]) {
      fadeSeq(q(r, `[data-${attr}="working"]`), [0, 600], T);
      fadeSeq(q(r, `[data-${attr}="needs"]`), [600, T], T);
    }
  },
  "card-move": r => {
    tw(
      q(r, "[data-card-move]"),
      [
        { transform: "translateX(-107%)" },
        { transform: "translateX(-107%)", offset: 0.2 },
        { transform: "none" },
      ],
      D.handoff * 1.25,
      { easing: E.inOut }
    );
  },
  "dock-answer": r => {
    const T = 1300;
    fadeSeq(q(r, '[data-dk="ask"]'), [0, 960], T);
    fadeSeq(q(r, '[data-dk="done"]'), [960, T], T);
    const c = cyan(r);
    tw(
      q(r, "[data-allow]"),
      [
        { background: "transparent" },
        { background: "transparent", offset: 0.5 },
        { background: `color-mix(in oklab, ${c} 25%, transparent)`, borderColor: c },
      ],
      960
    );
  },
  counter: r => {
    qa(r, "[data-col]").forEach((c, i, all) => {
      const d = Number(c.dataset.col);
      tw(
        c,
        [{ transform: "translateY(0)" }, { transform: `translateY(-${d * 10}%)` }],
        D.sweep + (all.length - i) * S.list * 2
      );
    });
  },
  // kinetic type
  words: r => {
    const words = qa(r, "[data-w]");
    words.forEach((w, i) =>
      tw(
        w,
        [
          { opacity: 0, transform: "translateY(.3em)" },
          { opacity: 1, transform: "none" },
        ],
        D.enter,
        { delay: i * S.reveal }
      )
    );
    tw(q(r, "[data-em]"), [{ opacity: 0.35 }, { opacity: 1 }], D.press, {
      delay: (words.length - 1) * S.reveal + D.enter + D.press,
    });
  },
  scramble: (r, alive) => {
    const set = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ/·_";
    qa(r, "[data-scr]").forEach((el, li) => {
      const final = el.dataset.scr ?? "";
      const t0 = performance.now() + li * 160;
      const tick = (now: number) => {
        if (!alive()) return;
        const t = now - t0;
        const done = Math.max(0, Math.floor(t / S.list));
        el.textContent = final
          .split("")
          .map((ch, i) =>
            i < done || ch === " "
              ? ch
              : t < 0 && i > 0
                ? ""
                : set[Math.floor(Math.random() * set.length)]
          )
          .join("");
        if (done < final.length) requestAnimationFrame(tick);
        else el.textContent = final;
      };
      requestAnimationFrame(tick);
    });
  },
  lines: r => {
    qa(r, "[data-line]").forEach((l, i) =>
      tw(
        l,
        [
          { clipPath: "inset(0 0 100% 0)", transform: "translateY(40%)" },
          { clipPath: "inset(0 0 0 0)", transform: "none" },
        ],
        D.sweep,
        { delay: i * S.reveal * 2 }
      )
    );
  },
  // tokens
  durations: r => {
    qa(r, "[data-dur]").forEach(el =>
      tw(el, [{ left: "0%" }, { left: "calc(100% - 10px)" }], Number(el.dataset.dur))
    );
  },
  stagger: r => {
    const step = Number(r.dataset.step ?? S.list);
    qa(r, "[data-st]").forEach((el, i) =>
      tw(
        el,
        [
          { opacity: 0, transform: "translateY(10px)" },
          { opacity: 1, transform: "none" },
        ],
        D.enter,
        { delay: i * step }
      )
    );
  },
};

// a run id per stage, so timers and frames of a previous run stop themselves on replay or unmount
const runs = new WeakMap<Element, number>();
const nextRun = (root: Element): Alive => {
  const n = (runs.get(root) ?? 0) + 1;
  runs.set(root, n);
  return () => runs.get(root) === n;
};

/** Stops the current run and cancels every animation, so the stage shows its final frame. */
export function resetDemo(root: HTMLElement) {
  nextRun(root);
  for (const a of root.getAnimations({ subtree: true })) a.cancel();
}

export function playDemo(id: DemoId, root: HTMLElement) {
  resetDemo(root);
  demos[id](root, nextRun(root));
}
