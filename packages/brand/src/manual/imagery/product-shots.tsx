import { cn } from "@compozy/ui/utils";

import { Visual } from "../art/visual";
import { card, h3, lab, specs } from "../cls";
import { DoDont } from "../parts/do-dont";
import { Section } from "../parts/section";
import { productRules } from "./data";
import { Window } from "./window";

const CAPTURE: [label: string, value: string][] = [
  ["Scale", "2x device pixels, exported as PNG; WebP at 85 for the web."],
  ["Window", "1440×900 app window, zoom 100%, dark theme, no extensions."],
  ["Padding", "Ground around the window = 6% of the shot’s width, at least 24 px."],
  ["Radius", "12 px at 1x; 24 px at 2x. The ground card takes 20 px."],
  ["Cursor", "Hidden, unless the shot is about a click. Then the system arrow, never a hand icon."],
  ["Refresh", "Recapture every minor release; captions carry the version."],
];

const ground = "rounded-card bg-rail p-[clamp(20px,4vw,48px)] shadow-[0_0_0_1px_var(--b-line)]";

function Shots() {
  return (
    <>
      <div className="grid items-start gap-4 lg:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)]">
        <figure className="m-0 min-w-0">
          <div className={ground}>
            <Window title="Compozy · desktop">
              <Visual name="desktop" inline ratio="16/10" />
            </Window>
          </div>
          <figcaption className="mt-2.5 text-sm text-(--b-muted)">
            Wide shot: the whole desktop. Use on the download page and in hero pieces; every other
            shot crops to one task.
          </figcaption>
        </figure>
        <figure className="m-0 min-w-0">
          <div className={ground}>
            <Window title="migrate-db · codex">
              <Visual name="approval" inline ratio="4/3" />
            </Window>
            <div className="mt-3 flex items-center gap-2 pl-5">
              <span className="h-px w-8 bg-brand-cyan" />
              <span className="font-mono text-[11.5px] text-brand-cyan">
                children never widen parent permissions
              </span>
            </div>
          </div>
          <figcaption className="mt-2.5 text-sm text-(--b-muted)">
            Cropped to one task: the approval. This is the default product shot.
          </figcaption>
        </figure>
      </div>
      <p className="mt-3 mono-label text-[11px] tracking-[0.08em] text-(--b-muted)">
        Captures of the real desktop components in the current theme.
      </p>
    </>
  );
}

function Rules() {
  return (
    <div className="mt-16 grid gap-4 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)]">
      <div className={card}>
        <p className={lab}>Rules</p>
        <dl className={cn(specs, "mt-4")}>
          {productRules.map(([k, v]) => (
            <div key={k}>
              <dt>{k}</dt>
              <dd>{v}</dd>
            </div>
          ))}
        </dl>
      </div>
      <div className={card}>
        <p className={lab}>Capture</p>
        <dl className={cn(specs, "mt-4")}>
          {CAPTURE.map(([k, v]) => (
            <div key={k}>
              <dt>{k}</dt>
              <dd>{v}</dd>
            </div>
          ))}
        </dl>
      </div>
    </div>
  );
}

function Straight() {
  return (
    <>
      <h3 className={`${h3} mt-16`}>Straight, never staged</h3>
      <DoDont
        className="mt-4"
        doContent={
          <>
            <div className="rounded-[14px] bg-rail p-6">
              <Window title="Board · who has the next step">
                <Visual name="board" inline ratio="16/9" />
              </Window>
            </div>
            <p className="m-0 mt-4 text-[15px] text-(--b-muted)">
              Flat, 1:1 scale, one thin frame, the crop shows one screen: the board.
            </p>
          </>
        }
        dontContent={
          <>
            <div className="overflow-hidden rounded-[14px] bg-[radial-gradient(circle_at_30%_20%,var(--color-brand-volt),var(--color-rail)_70%)] p-6 [perspective:700px]">
              <div className="[transform:rotateX(18deg)_rotateY(-24deg)_rotateZ(4deg)] rounded-[22px] border-[6px] border-line-strong shadow-[0_30px_60px_rgba(0,0,0,.6),0_0_40px_var(--color-brand-cyan)]">
                <Window title="Board">
                  <Visual name="board" inline ratio="16/9" />
                </Window>
              </div>
            </div>
            <p className="m-0 mt-4 text-[15px] text-(--b-muted)">
              Tilted device, glow, bezel, gradient field. It looks like a concept, so readers assume
              the product is not real.
            </p>
          </>
        }
      />
    </>
  );
}

export function ProductShots() {
  return (
    <Section
      id="product-shots"
      n="03"
      file="product-shots.md"
      title={
        <>
          Our UI is the <em>hero</em>
        </>
      }
      intro={
        <p>
          The desktop app is the strongest image Compozy has: faces, board, handoff, approvals. Show
          it as it ships, a real screenshot on ink inside one thin window, cropped to the single
          task the sentence beside it describes. The terminal is the before picture, never the
          product shot.
        </p>
      }
    >
      <Shots />
      <Rules />
      <Straight />
    </Section>
  );
}
