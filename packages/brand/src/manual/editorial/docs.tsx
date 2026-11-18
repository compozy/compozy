import { cn } from "@compozy/ui/utils";
import { Fragment } from "react";

import { Visual } from "../art/visual";
import { card, lab } from "../cls";
import { Section } from "../parts/section";
import { callouts, cyanInk, docsRules } from "./data";
import { Rows } from "./rows";

const nav: [group: string, items: string[]][] = [
  ["Get started", ["Install", "Start the daemon", "First session"]],
  ["Loops", ["Overview", "Definition of done", "Judges", "Budgets"]],
  ["Sessions", ["Durable sessions", "Attention", "Worktrees"]],
];
const steps = ["objective", "act", "judge", "outcome"];
const label = "m-0 mono-label text-[11px] tracking-[0.12em]";
const prompt = <span className="select-none text-muted-foreground">$ </span>;

function DocsNav() {
  return (
    <nav className="hidden border-r border-(--b-line) p-5 text-[13.5px] lg:block">
      {nav.map(([g, items]) => (
        <div key={g} className="mb-5">
          <p className={cn(label, "mb-1.5 text-(--b-muted)")}>{g}</p>
          {items.map(i => (
            <p
              key={i}
              className={cn(
                "m-0 rounded-md px-2 py-1",
                i === "Definition of done"
                  ? "bg-(--b-card-2) font-medium text-(--b-fg)"
                  : "text-(--b-muted)"
              )}
            >
              {i}
            </p>
          ))}
        </div>
      ))}
    </nav>
  );
}

function DocsArticle() {
  return (
    <article className="min-w-0 px-6 py-8 text-(--b-fg) sm:px-10">
      <p className="m-0 mono-label text-[11.5px] tracking-[0.12em] text-(--b-muted)">
        Loops / Definition of done{" "}
        <span
          className={cn(
            "ml-2 rounded border border-[color-mix(in_oklab,var(--color-brand-cyan)_40%,transparent)] px-1.5",
            cyanInk
          )}
        >
          beta
        </span>
      </p>
      <h3 className="m-0 mt-3 font-display text-[40px] leading-[1.05] tracking-[-0.025em]">
        Write a loop with a definition of done
      </h3>
      <p className="m-0 mt-4 max-w-[68ch] text-[17px] leading-[1.65] text-(--b-muted)">
        A Goal node runs an agent until a judge says the objective is met, or until its budget runs
        out. This page shows the four fields that decide when a loop stops.
      </p>
      <div className="relative mt-6 aspect-[16/5] overflow-hidden rounded-[12px] border border-(--b-line)">
        <Visual name="orchestrate" theme="light" x={0.5} y={0.5} />
      </div>
      <div className="mt-6 grid gap-3">
        {callouts.map(c => (
          <div
            key={c.kind}
            className={cn(
              "rounded-[10px] border-l-[3px] bg-(--b-surface) px-4 py-3 text-[15px] leading-[1.6]",
              c.border
            )}
          >
            <span className={cn("mr-2 mono-label text-[11.5px] font-medium", c.text)}>
              {c.kind}
            </span>
            <span className="text-(--b-fg)">{c.body}</span>
          </div>
        ))}
      </div>
      <h4 className="m-0 mt-8 font-display text-[26px] tracking-[-0.02em]">Run it</h4>
      <div className="mt-3 overflow-hidden rounded-[12px] bg-rail font-mono text-[13.5px] leading-[1.7] text-fg">
        <div className="flex items-center justify-between border-b border-line px-4 py-2 text-[12px]">
          <span className="flex gap-4">
            <span className="text-fg">CLI</span>
            <span className="text-muted-foreground">HTTP</span>
            <span className="text-muted-foreground">Native tool</span>
          </span>
          <span className="text-muted-foreground">Copy</span>
        </div>
        <pre className="m-0 overflow-x-auto p-4">
          {prompt}compozy loop validate fix-flaky-tests{"\n"}
          {prompt}compozy loop run fix-flaky-tests --dry-run{"\n"}
          {prompt}compozy loop run fix-flaky-tests
        </pre>
      </div>
      <h4 className="m-0 mt-8 font-display text-[26px] tracking-[-0.02em]">How a run ends</h4>
      <div className="mt-4 flex flex-wrap items-center gap-2 font-mono text-[12.5px]">
        {steps.map((s, i) => (
          <Fragment key={s}>
            <span
              className={cn(
                "rounded-[10px] border px-4 py-2.5",
                i === 2
                  ? "border-(--b-accent) text-(--b-accent)"
                  : "border-[color-mix(in_oklab,var(--b-muted)_50%,transparent)] text-(--b-fg)"
              )}
            >
              {s}
            </span>
            {i < steps.length - 1 && <span className="text-(--b-muted)">→</span>}
          </Fragment>
        ))}
      </div>
      <p className="m-0 mt-3 font-mono text-[12px] text-(--b-muted)">
        judge fails → act again, until max_turns · then on_exhausted: halt | escalate
      </p>
    </article>
  );
}

function DocsSpecimen() {
  return (
    <div className="overflow-hidden rounded-card border border-(--b-line) bg-(--b-card) shadow-[0_1px_0_var(--b-line)]">
      <div className="flex items-center justify-between border-b border-(--b-line) px-6 py-3">
        <div className="flex items-center gap-3">
          <img src="/brand/svg/compozy-lockup-light.svg" alt="Compozy" className="h-5 w-auto" />
          <span
            className={cn(
              "rounded-md border border-(--b-line) px-1.5 py-0.5 mono-label text-[10.5px] tracking-[0.12em]",
              cyanInk
            )}
          >
            Docs
          </span>
        </div>
        <div className="flex items-center gap-4 text-[13px] text-(--b-muted)">
          <span className="hidden rounded-[8px] border border-(--b-line) px-3 py-1.5 font-mono text-[12px] sm:inline">
            Search ⌘K
          </span>
          <span>GitHub</span>
          <span>Changelog</span>
        </div>
      </div>
      <div className="grid lg:grid-cols-[220px_minmax(0,1fr)_200px]">
        <DocsNav />
        <DocsArticle />
        <aside className="hidden border-l border-(--b-line) p-5 text-[13px] text-(--b-muted) lg:block">
          <p className={cn(label, "mb-2")}>On this page</p>
          <p className="m-0 py-1 text-(--b-fg)">Run it</p>
          <p className="m-0 py-1">How a run ends</p>
          <p className="m-0 py-1">Judges</p>
          <p className="m-0 py-1">Budgets</p>
          <p className={cn(label, "mt-6")}>Edit on GitHub ↗</p>
        </aside>
      </div>
    </div>
  );
}

export function Docs() {
  return (
    <Section
      id="docs"
      n="02"
      file="docs.compozy.com"
      title={
        <>
          Docs on <em>paper</em>
        </>
      }
      bg="paper"
      status="proposal"
      intro={
        <p>
          The docs are where the product is proved. They sit on paper, read like reference material
          and carry the same type, symbol and labels as everything else, at a working volume.
        </p>
      }
    >
      <DocsSpecimen />
      <p className="mt-2.5 text-sm text-(--b-muted)">
        Live HTML specimen. Callouts: Note in blue, Beta in cyan ink, Warning in amber, all 3 px
        left rules on paper; never a filled coloured box.
      </p>
      <div className={cn(card, "mt-10")}>
        <p className={lab}>Page anatomy</p>
        <Rows rows={docsRules} className="mt-3" />
      </div>
    </Section>
  );
}
