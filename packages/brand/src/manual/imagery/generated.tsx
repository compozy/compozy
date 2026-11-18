import { cn } from "@compozy/ui/utils";

import { card, h3, lab, rules } from "../cls";
import { GenImage } from "../parts/gen-image";
import { Section } from "../parts/section";
import { genAllowed, genForbidden, photo, promptExample, promptSchema } from "./data";
import { RuleCard } from "../parts/rule-card";

const codeCard = "min-w-0 overflow-hidden rounded-card border border-(--b-line) bg-(--b-card)";
const codeHead =
  "m-0 border-b border-(--b-line) px-5 py-3 font-mono text-[12.5px] text-(--b-muted)";
const codeBody = "m-0 p-5 font-mono text-[12.5px] leading-[1.75] whitespace-pre-wrap";

function Prompts() {
  return (
    <>
      <div className="grid gap-4 lg:grid-cols-2">
        <div className={codeCard}>
          <p className={codeHead}>prompts/photo/&lt;id&gt;.md · schema</p>
          <pre className={codeBody}>
            <code>{promptSchema}</code>
          </pre>
        </div>
        <div className={codeCard}>
          <p className={codeHead}>prompts/photo/laptop-closed.md · filled</p>
          <pre className={cn(codeBody, "text-(--b-accent)")}>
            <code>{promptExample}</code>
          </pre>
        </div>
      </div>

      <div className="mt-4 grid gap-4 md:grid-cols-2">
        <RuleCard title="Allowed">
          {genAllowed.map(r => (
            <li key={r}>{r}</li>
          ))}
        </RuleCard>
        <div
          className={cn(
            card,
            "border-dashed border-[color-mix(in_oklab,var(--color-destructive)_55%,transparent)]"
          )}
        >
          <p className="m-0 mono-label text-destructive">Never</p>
          <ul className={cn(rules, "mt-4 text-[15px] marker:text-destructive")}>
            {genForbidden.map(r => (
              <li key={r}>{r}</li>
            ))}
          </ul>
        </div>
      </div>
    </>
  );
}

function Provenance() {
  const ex = photo("laptop-closed");
  return (
    <>
      <h3 className={`${h3} mt-16`}>Provenance caption</h3>
      <div className="mt-4 grid items-start gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
        <GenImage src={ex.src} alt={ex.alt} caption={ex.caption} ratio="3/2" />
        <div className="grid gap-4">
          <div className={card}>
            <p className={lab}>Format</p>
            <p className="m-0 mt-3 font-mono text-[14px]">
              &lt;caption&gt; · <span className="text-(--b-accent)">GENERATED REFERENCE</span>
            </p>
            <ul className={cn(rules, "mt-4 text-[15px]")}>
              <li>
                Geist Mono 11 px, uppercase, 0.08 em tracking, 70% opacity, right after the caption.
              </li>
              <li>
                On social posts, the last line of the post reads “Image generated, not a
                photograph.”
              </li>
              <li>
                The prompt file and model name stay in <code>prompts/</code> next to the image, so
                anyone can trace it.
              </li>
            </ul>
          </div>
          <RuleCard title="Review">
            <li>
              Check hands, keyboards and screens at 200%: extra fingers, melted keys and fake glyphs
              are the usual failures.
            </li>
            <li>Any letter-like shape gets painted out before export.</li>
            <li>
              Generate four, pick one on a contact sheet next to real photos; reject anything that
              looks rendered.
            </li>
          </RuleCard>
        </div>
      </div>
    </>
  );
}

export function Generated() {
  return (
    <Section
      id="generated"
      n="05"
      file="generated.md"
      title={
        <>
          Generated images, <em>labelled</em>
        </>
      }
      intro={
        <p>
          Image models are a sketching tool for Compozy: references, objects, backgrounds a real
          photo will replace. They never stand in for a real person and never carry text. Every
          generated image says so in its caption.
        </p>
      }
    >
      <Prompts />
      <Provenance />
    </Section>
  );
}
