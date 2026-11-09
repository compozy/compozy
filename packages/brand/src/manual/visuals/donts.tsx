import { cn } from "@compozy/ui/utils";

import { Visual } from "../art/visual";
import type { VisualName } from "../art/visuals";
import { Section } from "../parts/section";
import { donts, type DontKind } from "./data";

/** The broken example drawn for each don't. */
function Example({ visual, kind }: { visual: VisualName; kind: DontKind }) {
  if (kind === "paper") {
    return (
      <div className="absolute inset-[14%] overflow-hidden rounded-[8px]">
        <Visual name={visual} />
      </div>
    );
  }
  if (kind === "two") {
    return (
      <>
        <div className="absolute inset-y-0 left-0 w-1/2">
          <Visual name="terrain" />
        </div>
        <div className="absolute inset-y-0 right-0 w-1/2">
          <Visual name="cone" />
        </div>
      </>
    );
  }
  return (
    <>
      <Visual
        name={visual}
        x={kind === "text-on-light" ? 0.3 : undefined}
        className={kind === "hue" ? "[&>img]:hue-rotate-[150deg]" : undefined}
      />
      {kind === "text-on-light" && (
        <p className="absolute top-[38%] left-[10%] m-0 font-display text-[clamp(18px,2vw,26px)] leading-[1.05]">
          Agents that keep running.
        </p>
      )}
      {kind === "ring" && (
        <svg className="absolute inset-0 h-full w-full" viewBox="0 0 400 300" aria-hidden="true">
          <ellipse
            cx="270"
            cy="150"
            rx="90"
            ry="60"
            transform="rotate(-50 270 150)"
            fill="none"
            strokeWidth="14"
            style={{ stroke: "var(--color-brand-cyan)" }}
          />
          <ellipse
            cx="240"
            cy="125"
            rx="90"
            ry="60"
            transform="rotate(-50 240 125)"
            fill="none"
            strokeWidth="14"
            opacity=".5"
            style={{ stroke: "var(--color-brand-volt)" }}
          />
        </svg>
      )}
      {kind === "fake" && (
        <span className="absolute right-3 bottom-3 rounded bg-destructive px-2 py-0.5 font-mono text-[11px] text-destructive-ink">
          99.99% faster
        </span>
      )}
    </>
  );
}

export function Donts() {
  return (
    <Section
      id="donts"
      n="10"
      file="donts.md"
      title={
        <>
          What breaks the <em>system</em>
        </>
      }
      bg="black"
      status="proposal"
      intro={
        <p>
          Each of these has happened in drafts. Each one makes the visuals look like stock art
          instead of Compozy.
        </p>
      }
    >
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {donts.map(([t, d, v, k]) => (
          <figure key={t} className="m-0 min-w-0">
            <div
              className={cn(
                "relative isolate aspect-[4/3] overflow-hidden rounded-card border border-dashed border-[color-mix(in_oklab,var(--color-destructive)_55%,transparent)]",
                k === "paper" && "band-paper"
              )}
            >
              <Example visual={v} kind={k} />
              <span className="absolute top-3 left-3 grid h-6 w-6 place-items-center rounded-full bg-destructive font-mono text-[13px] text-(--color-fg-strong)">
                ✕
              </span>
            </div>
            <figcaption className="mt-3">
              <span className="block text-[16px] font-medium">{t}</span>
              <span className="mt-1 block text-[14px] text-(--b-muted)">{d}</span>
            </figcaption>
          </figure>
        ))}
      </div>
    </Section>
  );
}
