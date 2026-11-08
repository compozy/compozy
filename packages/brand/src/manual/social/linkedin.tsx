import { cn } from "@compozy/ui/utils";

import { card, lab, specs } from "../cls";
import { Frame } from "../kit-frame";
import { Section } from "../parts/section";
import { linkedinRules } from "./data";

const FLAT = "[&_figure>div]:rounded-none [&_figure>div]:shadow-none";

/** LinkedIn feed mock: a light card on the paper band, drawn with band variables. */
function FeedMock() {
  return (
    <div className="mx-auto w-full max-w-[560px] overflow-hidden rounded-[10px] border border-(--b-line) bg-(--b-card) text-(--b-fg) shadow-[0_1px_2px_color-mix(in_oklab,var(--color-black)_6%,transparent)]">
      <div className="flex gap-3 p-4">
        <span className="grid size-12 shrink-0 place-items-center rounded-full bg-canvas font-display text-[18px] text-fg">
          PN
        </span>
        <div className="min-w-0">
          <p className="m-0 text-[14px] font-semibold">Pedro Nauck</p>
          <p className="m-0 truncate text-[12px] text-(--b-muted)">Founder at Compozy · 2h</p>
        </div>
      </div>
      <p className="m-0 px-4 pb-3 text-[14px] leading-[1.45]">
        I have taught 3,000 developers to work with agents. The bottleneck stopped being the model.
        It became managing the sessions: five terminals, and the prompt in the wrong window.
        <span className="text-(--b-muted)"> …see more</span>
      </p>
      <div className={FLAT}>
        <Frame id="linkedin-post" caption={false} />
      </div>
      <div className="flex justify-between border-t border-(--b-line) px-4 py-2.5 text-[12.5px] text-(--b-muted)">
        <span>Like</span>
        <span>Comment</span>
        <span>Repost</span>
        <span>Send</span>
      </div>
    </div>
  );
}

export function Linkedin() {
  return (
    <Section
      id="linkedin"
      n="03"
      file="linkedin.md"
      title={
        <>
          LinkedIn: the <em>process</em> argument
        </>
      }
      bg="paper"
      intro={
        <p>
          LinkedIn readers are engineering managers deciding how a team should work with agents. The
          post leads with the bottleneck, managing the sessions, and keeps the mechanism one step
          deeper.
        </p>
      }
    >
      <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
        <FeedMock />
        <div className="grid gap-4">
          <Frame id="linkedin-post" guides />
        </div>
      </div>
      <p className="mt-3 mono-label text-[11px] tracking-[0.08em] text-(--b-muted)">
        Feed mock: the first 210 characters are all most readers see before “see more”.
      </p>
      <div className="mt-12 grid gap-4 lg:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)]">
        <div className={card}>
          <p className={lab}>Rules</p>
          <dl className={cn(specs, "mt-4")}>
            {linkedinRules.map(([k, v]) => (
              <div key={k}>
                <dt>{k}</dt>
                <dd>{v}</dd>
              </div>
            ))}
          </dl>
        </div>
        <div className={card}>
          <p className={lab}>Company page</p>
          <p className="m-0 mt-3 text-[15px]">
            Tagline: “The autonomous control plane for your agents.” About: three sentences from the
            messaging bank, the repo link and the docs link. Industry: Software Development.
          </p>
          <p className="m-0 mt-3 text-[15px] text-(--b-muted)">
            The page reshares founder posts with one line of context; it posts on its own only for
            releases and events.
          </p>
          <div className="mt-6 grid grid-cols-[120px_minmax(0,1fr)] items-center gap-4">
            <div className={cn("overflow-hidden rounded-[18%]", FLAT)}>
              <Frame id="linkedin-avatar" caption={false} />
            </div>
            <p className="m-0 text-[14px] text-(--b-muted)">
              Logo: the avatar master at 400×400. LinkedIn rounds the corners; the symbol stays
              clear of them.
            </p>
          </div>
        </div>
      </div>
    </Section>
  );
}
