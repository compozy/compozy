import { cn } from "@compozy/ui/utils";

import { card, h3, lab } from "../cls";
import { Frame } from "../kit-frame";
import { DoDont } from "../parts/do-dont";
import { InContext } from "../parts/in-context";
import { Section } from "../parts/section";
import { xRules } from "./data";
import { Phone } from "./phone";
import { SpecCard } from "../parts/spec-card";

const thread: [n: string, text: string][] = [
  ["1/", "The outcome, alone. Works if nobody opens the thread."],
  ["2/", "The problem it removes, in the reader’s words."],
  ["3/", "The mechanism: always-on, handoff, approval."],
  ["4/", "A crop of the desktop, or a command if the post is about the CLI."],
  ["5/", "What does not work yet. Honest limits build trust."],
  ["6/", "One link and how to try it."],
];

export function X() {
  return (
    <Section
      id="x"
      n="02"
      file="x.md"
      title={
        <>
          X: short, <em>specific</em>, linked
        </>
      }
      bg="ink2"
      intro={
        <p>
          On X, Compozy talks like an engineer who shipped something: one idea, one fact, one link.
          Images exist to carry a fact the text cannot, such as a version, a quote or a product
          crop.
        </p>
      }
    >
      <div className="grid items-center gap-8 lg:grid-cols-[minmax(0,1fr)_380px]">
        <div className="grid gap-4">
          <Frame id="x-post" guides />
          <div className="grid gap-4 md:grid-cols-2">
            <Frame id="release-card" />
            <Frame id="x-quote" />
          </div>
        </div>
        <div>
          <Phone />
          <p className="m-0 mt-3 text-center mono-label text-[11.5px] tracking-[0.08em] text-(--b-muted)">
            Release card in the feed, 1:1 at phone width
          </p>
        </div>
      </div>

      <div className="mt-16 grid gap-4 lg:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)]">
        <SpecCard title="Rules" rows={xRules} />
        <div className={card}>
          <p className={lab}>A thread, six posts</p>
          <ol className="m-0 mt-4 grid list-none gap-0 p-0 text-[14.5px]">
            {thread.map(([n, t]) => (
              <li key={n} className="flex gap-3 border-b border-(--b-line) py-2.5 last:border-0">
                <span className="w-7 shrink-0 font-mono text-(--b-accent)">{n}</span>
                <span>{t}</span>
              </li>
            ))}
          </ol>
        </div>
      </div>

      <h3 className={cn(h3, "mt-16")}>The quote card</h3>
      <DoDont
        className="mt-4"
        doContent={
          <p className="m-0 text-[15px]">
            A sentence a named person said or wrote, with their consent, attributed with name and
            role. The founder’s own lines are the default source.
          </p>
        }
        dontContent={
          <p className="m-0 text-[15px] text-(--b-muted)">
            An anonymous “developer at a big tech company”, a paraphrase in quotation marks, or a
            quote from a design partner presented as a customer review.
          </p>
        }
      />
      <InContext
        ids={["phone-feed"]}
        title="In context · the release post in a feed"
        className="max-w-[520px]"
      />
    </Section>
  );
}
