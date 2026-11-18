import { card, lab } from "../cls";
import { Frame } from "../kit-frame";
import { Section } from "../parts/section";
import { blogRules } from "./data";
import { Rows } from "./rows";

const figures: [topic: string, visual: string][] = [
  ["Orchestration", "orchestrate"],
  ["Always-on agents", "agents"],
  ["Board and next steps", "board"],
  ["Approvals", "approval"],
  ["The whole desktop", "desktop"],
  ["Infrastructure essays", "terrain (Atmosphere)"],
  ["Founder essays", "swarm (Atmosphere)"],
];

const inlineCode = "rounded bg-elevated px-1.5 py-0.5 text-(--color-brand-cyan)";

function SamplePost() {
  return (
    <article className="min-w-0 overflow-hidden rounded-card border border-(--b-line) bg-rail">
      <Frame id="blog-hero" caption={false} />
      <div className="mx-auto max-w-[680px] px-6 py-10 sm:px-8">
        <p className="m-0 mono-label tracking-[0.12em] text-(--color-brand-cyan)">
          Engineering · loops
        </p>
        <h3 className="m-0 mt-3 font-display text-[clamp(32px,3.6vw,52px)] leading-[1.02] tracking-[-0.025em] text-fg">
          Why agent work needs a definition of done
        </h3>
        <p className="m-0 mt-4 mono-label text-muted-foreground">
          Pedro Nauck · Nov 12, 2026 · 6 min read
        </p>
        <div className="mt-8 flex flex-col gap-5 text-[18px] leading-[1.7] text-fg/90">
          <p className="m-0">
            I have taught more than 3,000 developers to work with agents, and the place they break
            is not the prompt. It is the moment nobody decided what finished looks like.
          </p>
          <p className="m-0">
            An agent with no definition of done keeps going until the context window or the budget
            ends it. Neither is an outcome you can read the next morning.
          </p>
          <blockquote className="m-0 border-l-2 border-(--color-brand-cyan) py-1 pl-5 font-display text-[26px] leading-[1.25] tracking-[-0.02em] text-fg">
            A run should end in a state you can name, not in a bill you discover.
          </blockquote>
          <h4 className="m-0 pt-3 font-display text-[28px] tracking-[-0.02em] text-fg">
            Four fields
          </h4>
          <p className="m-0">
            In Compozy a Goal node needs an <code className={inlineCode}>objective</code>, a{" "}
            <code className={inlineCode}>judge</code>, a{" "}
            <code className={inlineCode}>max_turns</code> and what to do when turns run out. That is
            the whole contract.
          </p>
        </div>
      </div>
    </article>
  );
}

export function Blog() {
  return (
    <Section
      id="blog"
      n="03"
      file="blog/[slug].mdx"
      title={
        <>
          Articles that <em>prove</em> a point
        </>
      }
      bg="ink2"
      status="proposal"
      intro={
        <p>
          The blog is where the founder thinks in public and the product shows its work. One column,
          a hero made of our UI (essays: one Atmosphere visual), the title in HTML, and a command or
          a file behind every claim.
        </p>
      }
    >
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.5fr)]">
        <div className={card}>
          <p className={lab}>Article rules</p>
          <Rows rows={blogRules} className="mt-3" />
        </div>
        <SamplePost />
      </div>
      <div className="mt-10 grid items-start gap-6 lg:grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)]">
        <Frame id="blog-hero" guides />
        <div className={card}>
          <p className={lab}>One visual per topic</p>
          <dl className="m-0 mt-3 grid gap-0 font-mono text-[13px]">
            {figures.map(([t, f]) => (
              <div
                key={t}
                className="flex justify-between gap-4 border-b border-(--b-line) py-2.5 last:border-0"
              >
                <dt className="text-(--b-muted)">{t}</dt>
                <dd className="m-0 text-(--b-accent)">{f}</dd>
              </div>
            ))}
          </dl>
          <p className="m-0 mt-4 text-[14px] text-(--b-muted)">
            The hero carries no title: search engines and screen readers get the H1, and the image
            stays reusable when the title changes.
          </p>
        </div>
      </div>
    </Section>
  );
}
