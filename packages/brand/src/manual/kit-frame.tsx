// Live preview of one kit template: the real template component rendered at the frame's width
// (templates are sized in cqi, so they scale with the box), with optional guides (dashed content
// margin + hatched bands the platform covers) and a caption linking to its /kit/$id page.
import { cn } from "@compozy/ui/utils";

import type { Insets } from "./kit-specs";
import { fieldsOf, findTemplate } from "./templates/registry";
import type { KitFields } from "./templates/types";

interface FrameProps {
  id: string;
  /** field overrides on top of the template defaults (e.g. one carousel body slide) */
  fields?: KitFields;
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

export function Frame({ id, fields, guides = false, caption = true, className }: FrameProps) {
  const found = findTemplate(id);
  if (!found) throw new Error(`unknown kit template ${id}`);
  const { spec: t, template } = found;
  const Template = template.component;
  return (
    <figure className={cn("m-0 min-w-0", className)} data-kit-frame={id}>
      <div
        className="relative overflow-hidden rounded-[14px] shadow-[0_0_0_1px_var(--b-line)] [container-type:inline-size]"
        style={{ aspectRatio: `${t.w}/${t.h}` }}
      >
        <Template {...fieldsOf(template, fields)} />
        {guides && <Guides margin={t.margin} unsafe={t.unsafe} />}
      </div>
      {caption && (
        <figcaption className="mt-2.5 flex flex-wrap items-center justify-between gap-2 mono-label text-[11.5px] tracking-[0.08em] text-(--b-muted)">
          <span>
            {t.name} · {t.w}×{t.h} · level {t.level}
          </span>
          <a href={`/kit/${t.id}`} className="text-(--b-accent) no-underline hover:underline">
            kit/{t.id} ↗
          </a>
        </figcaption>
      )}
    </figure>
  );
}
