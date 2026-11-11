import { card, lab, voltFill } from "../cls";
import { Frame } from "../kit-frame";
import { Section } from "../parts/section";
import { cyanInk, newsletterRules } from "./data";
import { Rows } from "./rows";

const label = `m-0 mt-3 mono-label text-[11.5px] tracking-[0.12em] ${cyanInk}`;

/** Sample issue at 600 px on white: a paper band so every colour resolves to the light values. */
function SampleIssue() {
  return (
    <div className="band-paper mx-auto w-full max-w-[600px] overflow-hidden rounded-[14px] shadow-[0_0_0_1px_var(--b-line)]">
      <div className="bg-(--b-card)">
        <div className="border-b border-(--b-line) px-6 py-3 font-mono text-[11.5px] text-(--b-muted)">
          <p className="m-0">
            <b className="font-medium text-(--b-fg)">Subject</b> · Agents now wait for you in one
            list
          </p>
          <p className="m-0 mt-0.5">
            Preview · v0.6.0 shipped: orchestrator sessions and a Kanban of who holds the next step.
          </p>
        </div>
        <Frame id="newsletter-header" caption={false} />
        <div className="flex flex-col gap-3 px-7 py-7 text-[16px] leading-[1.65] text-(--b-fg)">
          <p className="m-0">Hi, Pedro here.</p>
          <p className={label}>What shipped</p>
          <p className="m-0">
            v0.6.0 lets one orchestrator session run child sessions in their own worktrees, and the
            board now sorts every session by who holds the next step.{" "}
            <a href="#newsletter" className="text-(--b-accent) underline">
              Read the release notes
            </a>
            .
          </p>
          <p className={label}>What I learned</p>
          <p className="m-0">
            The question I heard most in the community this month was the same one: where do I see
            what needs me? That became the attention filter.
          </p>
          <p className={label}>What is next</p>
          <p className="m-0">
            Store-and-forward for the gateway, so offline peers stop missing events.
          </p>
          <p className="m-0 pt-2">
            <span
              className={`inline-block rounded-[10px] px-4 py-2.5 text-[15px] font-medium ${voltFill}`}
            >
              Install the beta
            </span>
          </p>
        </div>
        <div className="border-t border-(--b-line) px-7 py-5 text-[12.5px] leading-[1.6] text-(--b-muted)">
          <img src="/brand/svg/compozy-lockup-mono-ink.svg" alt="Compozy" className="h-4 w-auto" />
          <p className="m-0 mt-3">
            You get this because you starred compozy/compozy or installed the beta. One email a
            month at most.
          </p>
          <p className="m-0 mt-1">
            <span className="underline">Unsubscribe</span> · Compozy · [postal address]
          </p>
        </div>
      </div>
    </div>
  );
}

export function Newsletter() {
  return (
    <Section
      id="newsletter"
      n="05"
      file="newsletter.html"
      title={
        <>
          Email that reads with <em>images</em> off
        </>
      }
      bg="ink2"
      status="proposal"
      intro={
        <p>
          The newsletter is a letter from the person building the product. One header image, then
          text: what shipped, one thing learned, one thing next. It has to work in every client,
          including the ones that block images.
        </p>
      }
    >
      <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
        <SampleIssue />
        <div className="grid gap-4">
          <p className="m-0 text-[14px] text-(--b-muted)">
            Sample issue at 600 px, illustrative content. Section labels in Geist Mono cyan ink, the
            only colour in the body besides links.
          </p>
          <div className={card}>
            <p className={lab}>Rules</p>
            <Rows rows={newsletterRules} className="mt-3" />
          </div>
          <Frame id="newsletter-header" guides />
        </div>
      </div>
    </Section>
  );
}
