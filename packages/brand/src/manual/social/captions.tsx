import { cn } from "@compozy/ui/utils";

import { card, h3 } from "../cls";
import { Section } from "../parts/section";
import { anatomy, forbidden, formulas, lengths, posts } from "./data";
import { SpecCard } from "../parts/spec-card";

function Anatomy() {
  return (
    <div className="grid gap-px overflow-hidden rounded-card border border-(--b-line) bg-(--b-line) md:grid-cols-4">
      {anatomy.map((a, i) => (
        <div key={a.part} className="flex flex-col bg-(--b-card) p-6">
          <p className="m-0 flex items-center justify-between mono-label">
            <span className="text-(--b-accent)">
              0{i + 1} {a.part}
            </span>
            <span className="text-(--b-muted)">{a.words}</span>
          </p>
          <p
            className={cn(
              "m-0 mt-4 flex-1 text-[16px] leading-[1.45]",
              (a.part === "Proof" || a.part === "Link") &&
                "font-mono text-[14px] text-(--color-brand-cyan)"
            )}
          >
            {a.text}
          </p>
          <p className="m-0 mt-4 border-t border-(--b-line) pt-3 text-[13.5px] text-(--b-muted)">
            {a.note}
          </p>
        </div>
      ))}
    </div>
  );
}

export function Captions() {
  return (
    <Section
      id="captions"
      n="07"
      file="captions.md"
      title={
        <>
          Captions that <em>prove</em> something
        </>
      }
      bg="ink2"
      intro={
        <p>
          Every post has the same four parts: a hook that states the outcome, the mechanism, one
          piece of proof and one link. No hashtags, no emoji, no exclamation marks, on any network.
        </p>
      }
    >
      <Anatomy />

      <div className="mt-4 grid gap-4 lg:grid-cols-2">
        <SpecCard title="Formulas" rows={formulas} valueClassName="font-mono text-[13px]" />
        <SpecCard title="Length" rows={lengths} />
      </div>

      <h3 className={cn(h3, "mt-16")}>Six posts in the voice</h3>
      <div className="mt-4 grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {posts.map(p => (
          <article key={p.net} className={cn(card, "flex flex-col")}>
            <p className="m-0 flex items-center justify-between gap-2 mono-label text-[11.5px] tracking-[0.08em]">
              <span className="text-(--b-accent)">{p.net}</span>
              <span className="text-(--b-muted)">{p.kind}</span>
            </p>
            <p className="m-0 mt-4 flex-1 whitespace-pre-line text-[15px] leading-[1.5]">
              {p.text}
            </p>
            <p className="m-0 mt-4 border-t border-(--b-line) pt-3 font-mono text-[11.5px] text-(--b-muted)">
              {p.text.length} characters
            </p>
          </article>
        ))}
      </div>

      <h3 className={cn(h3, "mt-16")}>Never in a post</h3>
      <div className="mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
        {forbidden.map(([k, v]) => (
          <div
            key={k}
            className="rounded-card border border-dashed border-[color-mix(in_oklab,var(--color-destructive)_55%,transparent)] bg-(--b-card) p-5"
          >
            <p className="m-0 mono-label tracking-[0.08em] text-destructive">✕ {k}</p>
            <p className="m-0 mt-2.5 text-[14px] text-(--b-muted)">{v}</p>
          </div>
        ))}
      </div>
    </Section>
  );
}
