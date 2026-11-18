import { Visual } from "../art/visual";
import { lab, voltFill } from "../cls";
import { InContext } from "../parts/in-context";
import { Section } from "../parts/section";

const B = "/brand";
const fluidUi = "text-[clamp(11px,1vw,14px)]";

function SiteHero() {
  return (
    <>
      <p className={lab}>Site · hero</p>
      <div className="relative isolate mt-3 aspect-[16/9] overflow-hidden rounded-card border border-(--b-line) bg-rail">
        <Visual name="desktop" x={0.98} y={0.6} className="-z-10" />
        <div className="absolute inset-0 -z-10 bg-[linear-gradient(90deg,var(--color-rail)_0%,var(--color-rail)_34%,color-mix(in_oklab,var(--color-rail)_75%,transparent)_48%,transparent_66%)]" />
        <div className="flex items-center justify-between px-[4%] pt-[3%]">
          <img
            src={`${B}/svg/compozy-lockup-dark.svg`}
            alt="Compozy"
            className="h-[clamp(16px,2.2vw,28px)] w-auto"
          />
          <div className={`hidden gap-6 text-muted-foreground sm:flex ${fluidUi}`}>
            <span>Product</span>
            <span>Docs</span>
            <span>Changelog</span>
            <span>GitHub</span>
          </div>
          <span className={`rounded-[10px] px-3 py-1.5 font-medium ${fluidUi} ${voltFill}`}>
            Install the beta
          </span>
        </div>
        <div className="absolute bottom-[12%] left-[4%] max-w-[44%]">
          <p className="m-0 mono-label text-[clamp(9px,.9vw,12px)] tracking-[0.12em] text-(--color-brand-cyan)">
            Beta · MIT · self-hosted
          </p>
          <p className="m-0 mt-[2%] font-display text-[clamp(22px,4vw,56px)] leading-[1.02] tracking-[-0.03em]">
            The autonomous control plane for your <em>agents</em>.
          </p>
          <p className="m-0 mt-[3%] max-w-[44ch] text-[clamp(11px,1.25vw,18px)] text-muted-foreground">
            Run Claude Code, Codex, Gemini and the rest from one desktop. Always-on agents work on
            their own and call you when they need you.
          </p>
          <p className="m-0 mt-[4%] flex flex-wrap items-center gap-3">
            <span className={`rounded-[10px] px-3.5 py-2 font-medium ${fluidUi} ${voltFill}`}>
              Download for macOS
            </span>
            <span className="font-mono text-[clamp(9px,.85vw,12px)] text-muted-foreground">
              or brew install compozy/tap/compozy
            </span>
          </p>
        </div>
      </div>
    </>
  );
}

function DocsPaper() {
  return (
    <div>
      <p className={lab}>Docs · paper</p>
      <div className="band-paper mt-3 overflow-hidden rounded-card border border-(--b-line)">
        <div className="flex items-center justify-between border-b border-(--b-line) px-5 py-3">
          <img src={`${B}/svg/compozy-lockup-light.svg`} alt="" className="h-4 w-auto" />
          <span className="font-mono text-[11px] text-(--b-muted)">docs / loops</span>
        </div>
        <div className="p-6">
          <p className="m-0 mono-label text-[11px] tracking-[0.12em] text-(--b-accent)">Loops</p>
          <p className="m-0 mt-2 font-display text-[28px] leading-tight tracking-[-0.02em] text-(--b-fg)">
            Goal, act, verify, stop
          </p>
          <p className="m-0 mt-3 text-[14.5px] text-(--b-muted)">
            A loop is repeatable work with a definition of done. It ends in a named state: done,
            blocked, failed, exhausted or stalled.
          </p>
          <pre className="m-0 mt-4 overflow-x-auto rounded-[12px] bg-rail p-4 font-mono text-[12.5px] leading-[1.7] text-fg">
            <span className="text-muted-foreground">$</span> compozy loop start review-pr \{"\n"}
            {"  "}--budget 5 --gate tests{"\n"}
            <span className="text-(--color-brand-cyan)">●</span> loop 1/5 · working
          </pre>
        </div>
      </div>
    </div>
  );
}

export function Applications() {
  return (
    <Section
      id="applications"
      n="11"
      file="applications.astro"
      title={
        <>
          The system, <em>applied</em>
        </>
      }
      status="proposal"
      intro={
        <p>
          Three surfaces show the identity at work: the site, the product and the docs. Each one
          uses the same ink, type and visual system, at a different volume.
        </p>
      }
    >
      <SiteHero />
      <div className="mt-10 grid gap-4 lg:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)]">
        <div>
          <p className={lab}>Product · desktop</p>
          <div className="mt-3 overflow-hidden rounded-card border border-(--b-line)">
            <Visual name="desktop" inline ratio="16/10" />
          </div>
        </div>
        <DocsPaper />
      </div>
      <InContext ids={["laptop-site", "monitor-app"]} title="In context · site and product" />
    </Section>
  );
}
