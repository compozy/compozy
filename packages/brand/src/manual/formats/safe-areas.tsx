// Formats, safe areas: platform chrome and crops drawn to scale over a real visual.
import { cn } from "@compozy/ui/utils";
import type { CSSProperties } from "react";

import { Visual } from "../art/visual";
import { cap, lab } from "../cls";
import { Section } from "../parts/section";
import { safeCases, type SafeCase, type Zone } from "./data";
import { RuleCard } from "../parts/rule-card";

const hatch =
  "bg-[repeating-linear-gradient(45deg,color-mix(in_oklab,var(--color-destructive)_38%,transparent)_0_6px,transparent_6px_13px)] border border-destructive/60";
const safe = "border-2 border-dashed border-brand-cyan bg-brand-cyan/8";
const neutral = "border border-dashed border-muted-foreground/70";
const wide = safeCases.filter(s => s.w / s.h > 1.5);
const tall = safeCases.filter(s => s.w / s.h <= 1.5);
const lbl = (kind?: string) =>
  kind === "safe" || kind === "cover" ? "top-1 left-1" : "right-1 bottom-1";

/** Zones never overlap exactly, so a zone's box names it within its case. */
const zoneKey = (z: Zone) => `${z.x},${z.y},${z.w},${z.h}`;

function ZoneBox({ z, className }: { z: Zone; className: string }) {
  return (
    <div
      className={cn("absolute", className)}
      style={{ left: `${z.x}%`, top: `${z.y}%`, width: `${z.w}%`, height: `${z.h}%` }}
    >
      {z.label && (
        <span
          className={cn(
            "absolute rounded bg-rail/90 px-1 font-mono text-[10px] leading-[1.5] whitespace-nowrap text-fg",
            lbl(z.kind)
          )}
        >
          {z.label}
        </span>
      )}
    </div>
  );
}

function Caption({ s, style }: { s: SafeCase; style?: CSSProperties }) {
  return (
    <figcaption className="mx-auto mt-3" style={style}>
      <span className={lab}>{s.name}</span>
      <span className={cn(cap, "mt-1! block")}>{s.note}</span>
    </figcaption>
  );
}

function WideCase({ s }: { s: SafeCase }) {
  return (
    <figure className="m-0 min-w-0">
      <div
        className="relative w-full overflow-hidden rounded-[12px] shadow-[0_0_0_1px_var(--b-line)]"
        style={{ aspectRatio: `${s.w}/${s.h}` }}
        role="img"
        aria-label={`${s.name} with covered and safe zones`}
      >
        <Visual name="swarm" x={0.82} y={0.6} className="opacity-70" />
        {s.zones.map(z => (
          <ZoneBox
            key={zoneKey(z)}
            z={z}
            className={cn(
              z.kind === "cover" ? hatch : z.kind === "safe" ? safe : neutral,
              (z.label === "avatar" || z.label === "profile photo") && "rounded-full"
            )}
          />
        ))}
      </div>
      <Caption s={s} />
    </figure>
  );
}

function TallCase({ s }: { s: SafeCase }) {
  return (
    <figure className="m-0 min-w-0">
      <div
        className="relative mx-auto w-full overflow-hidden rounded-[12px] shadow-[0_0_0_1px_var(--b-line)]"
        style={{ aspectRatio: `${s.w}/${s.h}`, maxWidth: Math.round((380 * s.w) / s.h) }}
        role="img"
        aria-label={`${s.name} with covered and safe zones`}
      >
        {s.circle ? (
          <div className="absolute inset-0 bg-[radial-gradient(circle_at_50%_55%,color-mix(in_oklab,var(--color-brand-volt)_30%,transparent),var(--color-rail)_62%)]" />
        ) : (
          <Visual name="converge" x={0.7} y={0.5} className="opacity-70" />
        )}
        {s.circle && (
          <>
            <img
              src="/brand/png/compozy-symbol-618.png"
              alt=""
              className="absolute top-[18%] left-[18%] w-[64%]"
            />
            <div className="absolute inset-0 rounded-[12px] [background:radial-gradient(circle_at_center,transparent_49.6%,color-mix(in_oklab,var(--color-destructive)_22%,transparent)_50%)]" />
            <div className="absolute inset-0 rounded-full border-2 border-dashed border-brand-cyan" />
          </>
        )}
        {s.zones.map(z => (
          <ZoneBox
            key={zoneKey(z)}
            z={z}
            className={
              z.kind === "cover"
                ? hatch
                : z.kind === "safe"
                  ? s.circle
                    ? "border border-dashed border-brand-cyan/70"
                    : safe
                  : neutral
            }
          />
        ))}
      </div>
      <Caption s={s} style={{ maxWidth: Math.max(300, Math.round((380 * s.w) / s.h)) }} />
    </figure>
  );
}

export function SafeAreas() {
  return (
    <Section
      id="safe-areas"
      n="03"
      file="safe-areas.svg"
      title={
        <>
          What the platform <em>covers</em>
        </>
      }
      status="proposal"
      intro={
        <p>
          Every network draws its own interface on top of the canvas or crops it. Red hatching is
          covered or cropped; cyan dashes are the area that survives everywhere. Text, logo and the
          visual's focal point stay out of red.
        </p>
      }
    >
      <div className="grid gap-x-6 gap-y-10 md:grid-cols-2">
        {wide.map(s => (
          <WideCase key={s.id} s={s} />
        ))}
      </div>
      <div className="mt-10 grid items-start gap-6 sm:grid-cols-2 lg:grid-cols-[minmax(0,0.75fr)_minmax(0,1fr)_minmax(0,1.25fr)]">
        {tall.map(s => (
          <TallCase key={s.id} s={s} />
        ))}
        <RuleCard title="Before you export">
          <li>
            Turn on the template guides (<code>guides</code>) and check nothing readable touches
            red.
          </li>
          <li>The visual may run under platform chrome; its focal point and the text may not.</li>
          <li>Preview the link image at 1:1 and 1.91:1: X, Slack and iMessage crop differently.</li>
          <li>
            Check the avatar at 32 px. If the gap closes, use the symbol alone on ink, without the
            visual.
          </li>
        </RuleCard>
      </div>
    </Section>
  );
}
