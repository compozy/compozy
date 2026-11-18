// A live signature card: stage (children, already in its final state), name, duration, ease,
// use, a short timeline and a replay button.
import { cn } from "@compozy/ui/utils";
import type { ReactNode } from "react";

import { btn, btnSecondary, btnSm, card, h3, lab } from "../cls";
import type { Sig } from "./data";
import type { DemoId } from "./demos";
import { useMotionDemo } from "./use-motion-demo";

export function ReducedNote({ shown, className }: { shown: boolean; className?: string }) {
  return (
    <p
      hidden={!shown}
      className={cn("m-0 mono-label text-[11.5px] tracking-[0.08em] text-warning", className)}
    >
      Reduced motion: final frame shown
    </p>
  );
}

export function ReplayButton({ onClick, label }: { onClick: () => void; label?: string }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(btn, btnSm, btnSecondary)}
      aria-label={label}
    >
      ↻ Replay
    </button>
  );
}

export function SigCard({ sig, children }: { sig: Sig; children: ReactNode }) {
  const { stageRef, play, reducedShown } = useMotionDemo(sig.id as DemoId);
  return (
    <article className={cn(card, "grid content-start gap-4")}>
      <div
        ref={stageRef}
        data-demo={sig.id}
        className="relative aspect-[2/1] overflow-hidden rounded-[14px] bg-rail"
      >
        {children}
      </div>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h3 className={cn(h3, "mb-0")}>{sig.name}</h3>
        <ReplayButton onClick={play} label={`Replay ${sig.name}`} />
      </div>
      <ReducedNote shown={reducedShown} />
      <dl className="m-0 grid grid-cols-2 gap-3 font-mono text-[12px]">
        <div>
          <dt className="text-(--b-muted)">duration</dt>
          <dd className="m-0 mt-1">{sig.dur}</dd>
        </div>
        <div>
          <dt className="text-(--b-muted)">ease</dt>
          <dd className="m-0 mt-1 text-(--b-accent)">{sig.ease}</dd>
        </div>
      </dl>
      <p className="m-0 text-[14.5px] text-(--b-muted)">
        <span className="text-(--b-fg)">Use:</span> {sig.use}
      </p>
      <div>
        <p className={cn(lab, "mb-1")}>timeline</p>
        <dl className="m-0 border-t border-(--b-line) text-[13px]">
          {sig.timeline.map(([t, w]) => (
            <div
              key={t}
              className="grid grid-cols-[96px_minmax(0,1fr)] gap-3 border-b border-(--b-line) py-1.5"
            >
              <dt className="font-mono text-(--b-accent)">{t}</dt>
              <dd className="m-0 text-(--b-muted)">{w}</dd>
            </div>
          ))}
        </dl>
      </div>
    </article>
  );
}
