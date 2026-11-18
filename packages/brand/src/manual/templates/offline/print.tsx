// Large prints: sticker sheet, A3 poster, roll-up banner and the tee centre-chest print.
import { Logo, StateGlyph } from "@compozy/ui";
import type { CSSProperties } from "react";

import { AgentFace } from "../../art/pieces";
import { Visual } from "../../art/visual";
import { Mark, Scaled, Scrim, Shot, SymbolArt } from "../parts";
import { cq, display, gradientFill, ink, monoUp, scrim } from "../styles";

/** The die-cut path: a dashed cyan outline 1.1cqi outside each shape. */
const CUT: CSSProperties = {
  outline: ".28cqi dashed var(--color-brand-cyan)",
  outlineOffset: "1.1cqi",
};
const box = (left: number, top: number, width: number, height: number): CSSProperties => ({
  left: cq(left),
  top: cq(top),
  width: cq(width),
  height: cq(height),
  ...CUT,
});

/** Sticker sheet, A5: die-cut shapes (symbol, lines, chips, agent faces); the dashed cyan line is
 *  the cut path. Faces are the product's own <AgentAvatar>. */
export function StickerSheet({ title = "Sticker set · 01" }: { title?: string }) {
  const label = `${monoUp} absolute text-(--b-muted)`;
  return (
    <div className="band-paper absolute inset-0 overflow-hidden text-(--b-fg)">
      <p
        data-field="title"
        className={label}
        style={{ left: cq(6), top: cq(5), fontSize: "1.6cqi", letterSpacing: ".14em" }}
      >
        {title}
      </p>
      <p
        className={label}
        style={{ right: cq(6), top: cq(5), fontSize: "1.6cqi", letterSpacing: ".14em" }}
      >
        kiss-cut · matte vinyl
      </p>

      {/* symbol die-cut, 60 mm */}
      <div
        className="absolute grid place-items-center rounded-full bg-rail"
        style={box(8, 14, 38, 38)}
      >
        <SymbolArt width={25} />
      </div>
      {/* wordmark, 70×22 mm */}
      <div
        className="absolute grid place-items-center bg-rail"
        style={{ ...box(52, 16, 40, 14), borderRadius: cq(3) }}
      >
        <Logo variant="lettering" decorative className="h-auto w-[72%] text-fg-strong" />
      </div>
      {/* OS chip, 30 mm */}
      <div
        className="absolute grid place-items-center border-line-strong bg-rail font-mono text-brand-cyan"
        style={{
          ...box(52, 36, 16, 16),
          borderRadius: cq(2.4),
          borderWidth: cq(0.5),
          borderStyle: "solid",
          fontSize: cq(6),
        }}
      >
        OS
      </div>
      {/* running pill */}
      <div
        className="absolute flex items-center justify-center rounded-full bg-rail font-mono text-brand-cyan"
        style={{ ...box(72, 38, 20, 9), fontSize: cq(2.8), gap: cq(1.2) }}
      >
        <Scaled px={14} w={2.6}>
          <StateGlyph state="running" still />
        </Scaled>
        running
      </div>
      {/* line sticker, 90×20 mm */}
      <div
        className="absolute flex items-center justify-center rounded-full bg-(--b-surface) font-display tracking-[-0.025em]"
        style={{
          ...box(8, 60, 58, 12),
          fontSize: cq(4.4),
          boxShadow: "inset 0 0 0 .35cqi var(--color-rail)",
        }}
      >
        Keep every agent running
      </div>
      {/* Always-on */}
      <div
        className="absolute flex items-center justify-center rounded-full bg-rail text-fg"
        style={{
          ...box(70, 60, 22, 12),
          fontSize: cq(2.9),
          gap: cq(1.2),
          boxShadow: "inset 0 0 0 .5cqi var(--color-brand-sky)",
        }}
      >
        <span
          className="inline-block rounded-full"
          style={{ width: cq(2), height: cq(2), ...gradientFill }}
        />
        Always-on
      </div>
      {/* agent face + name pill, 80×35 mm */}
      <div
        className="absolute flex items-center bg-rail font-mono text-fg"
        style={{ ...box(8, 80, 46, 20), borderRadius: cq(10), paddingLeft: cq(3), gap: cq(2.6) }}
      >
        <Scaled px={96} w={12}>
          <AgentFace name="checkout-lead" state="working" size={96} />
        </Scaled>
        <span className="grid" style={{ gap: cq(0.6) }}>
          <span style={{ fontSize: cq(2.7) }}>checkout-lead</span>
          <span className="text-brand-cyan" style={{ fontSize: cq(2) }}>
            working
          </span>
        </span>
      </div>
      {/* Field crop, contour cut */}
      <div
        className="absolute overflow-hidden bg-rail"
        style={{ ...box(60, 80, 32, 52), borderRadius: cq(16) }}
      >
        <Visual name="field" x={0.55} y={0.6} />
      </div>
      {/* agent-face stickers, 25 mm round: the always-on crew */}
      {(
        [
          ["qa-runner", "idle", 8],
          ["release-manager", "needs-you", 26],
        ] as const
      ).map(([agent, state, left]) => (
        <div
          key={agent}
          className="absolute grid place-items-center rounded-full bg-rail"
          style={box(left, 108, 15, 15)}
        >
          <Scaled px={96} w={10}>
            <AgentFace name={agent} state={state} size={96} chip={false} />
          </Scaled>
        </div>
      ))}
      <p
        className="absolute m-0 font-mono text-(--b-muted)"
        style={{ left: cq(6), bottom: cq(4), fontSize: cq(1.4), letterSpacing: ".06em" }}
      >
        Cut line 0.3 pt cyan, spot colour CutContour. Keep 2 mm white border inside every cut.
      </p>
    </div>
  );
}

/** Poster, A3: one line, a Swarm at architectural scale, the event strip. */
export function Poster({
  kicker = "Meetup · running agents in production",
  title = "Many agents. One control plane.",
  event = "Compozy meetup · São Paulo",
  date = "28 Nov 2026 · 19:00",
  url = "compozy.com/events",
}: {
  kicker?: string;
  title?: string;
  event?: string;
  date?: string;
  url?: string;
}) {
  return (
    <div className={ink}>
      <Visual name="swarm" x={0.7} y={0.35} />
      <Scrim
        style={scrim("180deg", [
          [40, 0],
          [68, 0.92],
          [100, 1],
        ])}
      />
      <div className="absolute flex flex-col" style={{ inset: cq(7) }}>
        <Mark size={4.4} className="self-start" />
        <div className="mt-auto">
          <p
            data-field="kicker"
            className={`${monoUp} text-brand-cyan`}
            style={{ fontSize: "1.8cqi", letterSpacing: ".14em" }}
          >
            {kicker}
          </p>
          <p
            data-field="title"
            className={display}
            style={{ marginTop: "2.4cqi", fontSize: "9.6cqi", lineHeight: 0.98, maxWidth: "9.5em" }}
          >
            {title}
          </p>
          <div
            className="flex items-end justify-between border-t border-line-strong font-mono"
            style={{
              marginTop: "6cqi",
              paddingTop: "3cqi",
              fontSize: "1.9cqi",
              letterSpacing: ".04em",
            }}
          >
            <div>
              <p data-field="event" className="m-0">
                {event}
              </p>
              <p data-field="date" className="m-0 text-muted-foreground">
                {date}
              </p>
            </div>
            <p data-field="url" className="m-0 text-brand-cyan">
              {url}
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}

/** Roll-up banner, 850×2000 mm at 2 px/mm. Message above 1,600 mm; the bottom 150 mm disappears
 *  into the cassette. Events are where people meet the product: the desktop is shown large. */
export function Rollup({
  title = "The autonomous control plane for your agents.",
  line = "Every harness in one place. Always-on agents check in on their own and call you only when needed.",
  url = "compozy.com",
}: {
  title?: string;
  line?: string;
  url?: string;
}) {
  return (
    <div className={ink}>
      <Shot name="desktop" w={88} style={{ left: cq(6), top: cq(102) }} />
      <div className="absolute" style={{ left: cq(9), right: cq(9), top: cq(9) }}>
        <Mark size={8} />
        <p
          data-field="title"
          className={display}
          style={{ marginTop: "14cqi", fontSize: "11cqi", lineHeight: 1 }}
        >
          {title}
        </p>
        <p
          data-field="line"
          className="m-0 text-muted-foreground"
          style={{ marginTop: "5cqi", fontSize: "4cqi", lineHeight: 1.4, maxWidth: "21em" }}
        >
          {line}
        </p>
      </div>
      <div
        className="absolute flex items-end justify-between"
        style={{ left: cq(9), right: cq(9), bottom: cq(16) }}
      >
        <p data-field="url" className="m-0 font-mono text-brand-cyan" style={{ fontSize: "4cqi" }}>
          {url}
        </p>
        <span
          className={`${monoUp} grid place-items-center border border-line-strong text-muted-foreground`}
          style={{
            width: cq(14),
            height: cq(14),
            fontSize: "1.8cqi",
            letterSpacing: ".1em",
            borderRadius: "1.2cqi",
          }}
        >
          QR
        </span>
      </div>
    </div>
  );
}

/** Tee centre-chest print, 250×120 mm on black cotton: symbol (simulated process) and one line. */
export function TeeChest({ line = "Keep every agent running" }: { line?: string }) {
  return (
    <div className="absolute inset-0 flex flex-col items-center justify-center overflow-hidden bg-black">
      <SymbolArt width={20} />
      <p
        data-field="line"
        className={`${monoUp} text-fg-strong`}
        style={{ marginTop: "4cqi", fontSize: "2.6cqi", letterSpacing: ".32em" }}
      >
        {line}
      </p>
    </div>
  );
}
