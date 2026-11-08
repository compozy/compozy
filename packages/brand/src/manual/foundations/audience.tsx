import { card, lab } from "../cls";
import { Section } from "../parts/section";
import { personas } from "./data";

export function Audience() {
  return (
    <Section
      id="audience"
      n="02"
      file="audience.md"
      title={
        <>
          Who we <em>talk</em> to
        </>
      }
      bg="ink2"
      status="approved"
      intro={
        <p>
          Today Compozy is for people who already run an agent CLI and want it to keep working.
          Write for them in the present tense. Non-technical operators are the future; claim them
          when the product does.
        </p>
      }
    >
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        {personas.map((p, i) => (
          <article key={p.name} className={`${card} flex flex-col`}>
            <p className="m-0 font-mono text-[12px] text-(--b-accent)">P{i + 1}</p>
            <h3 className="m-0 mt-5 font-display text-[24px] font-medium tracking-[-0.02em]">
              {p.name}
            </h3>
            <p className="m-0 mt-1 text-[14px] text-(--b-muted)">{p.role}</p>
            <dl className="m-0 mt-6 grid gap-4 text-[15px]">
              <div>
                <dt className={lab}>Pain</dt>
                <dd className="m-0 mt-1">{p.pain}</dd>
              </div>
              <div>
                <dt className={lab}>Wants</dt>
                <dd className="m-0 mt-1">{p.wants}</dd>
              </div>
              <div>
                <dt className={lab}>Trigger</dt>
                <dd className="m-0 mt-1">{p.trigger}</dd>
              </div>
            </dl>
            <p className="m-0 mt-auto pt-6">
              <span className="inline-flex rounded-full border border-(--b-line) px-3 py-1 font-mono text-[12px] text-(--b-accent)">
                → {p.cta}
              </span>
            </p>
          </article>
        ))}
      </div>
      <p className="mt-8 max-w-[70ch] text-[15px] text-(--b-muted)">
        Channel priority: Hacker News and GitHub first, X and AI-engineering Discords next, YouTube
        and the blog after, LinkedIn last. Investor copy lives outside the brand and follows its own
        rules.
      </p>
    </Section>
  );
}
