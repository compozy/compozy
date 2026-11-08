// Preview of one kit template, sized to the template's aspect ratio with optional guides
// (dashed content margin + hatched bands the platform covers). The template artwork and its
// /kit/$id page arrive with slice S5; until then the frame renders a marked slot.
import { cn } from "@compozy/ui/utils";

import { kitSpecById, type Insets } from "./kit-specs";

interface FrameProps {
  id: string;
  guides?: boolean;
  caption?: boolean;
  className?: string;
}

const HATCH =
  "bg-[repeating-linear-gradient(45deg,color-mix(in_oklab,var(--color-destructive)_35%,transparent)_0_6px,transparent_6px_12px)]";

const pct = (n: number) => `${n}%`;

function Guides({ margin, unsafe }: { margin: Insets; unsafe?: Insets }) {
  return (
    <div className="pointer-events-none absolute inset-0">
      <div
        className="absolute border border-dashed border-(--color-brand-cyan)"
        style={{
          top: pct(margin.t),
          right: pct(margin.r),
          bottom: pct(margin.b),
          left: pct(margin.l),
        }}
      />
      {unsafe && unsafe.t > 0 && (
        <div className={cn("absolute inset-x-0 top-0", HATCH)} style={{ height: pct(unsafe.t) }} />
      )}
      {unsafe && unsafe.b > 0 && (
        <div
          className={cn("absolute inset-x-0 bottom-0", HATCH)}
          style={{ height: pct(unsafe.b) }}
        />
      )}
      {unsafe && unsafe.r > 0 && (
        <div className={cn("absolute inset-y-0 right-0", HATCH)} style={{ width: pct(unsafe.r) }} />
      )}
      {unsafe && unsafe.l > 0 && (
        <div className={cn("absolute inset-y-0 left-0", HATCH)} style={{ width: pct(unsafe.l) }} />
      )}
    </div>
  );
}

export function Frame({ id, guides = false, caption = true, className }: FrameProps) {
  const t = kitSpecById(id);
  return (
    <figure className={cn("m-0 min-w-0", className)} data-kit-frame={id}>
      <div
        data-kit-pending="S5"
        className="relative grid place-items-center overflow-hidden rounded-[14px] bg-(--b-card) shadow-[0_0_0_1px_var(--b-line)]"
        style={{ aspectRatio: `${t.w}/${t.h}` }}
      >
        <p className="m-0 max-w-[90%] truncate rounded-md border border-dashed border-(--b-line) bg-(--b-surface) px-2.5 py-1 mono-label text-[10px] text-(--b-muted)">
          Template · {t.name}
        </p>
        {guides && <Guides margin={t.margin} unsafe={t.unsafe} />}
      </div>
      {caption && (
        <figcaption className="mt-2.5 flex flex-wrap items-center justify-between gap-2 mono-label text-[11.5px] tracking-[0.08em] text-(--b-muted)">
          <span>
            {t.name} · {t.w}×{t.h} · level {t.level}
          </span>
          <span className="text-(--b-accent)">kit/{t.id}</span>
        </figcaption>
      )}
    </figure>
  );
}
