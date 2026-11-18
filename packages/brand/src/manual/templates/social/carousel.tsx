// Carousel slides, 1080×1350 (LinkedIn document post and X image set): cover, one body slide per
// idea (claim + one product capture), and the closing call to action.
import { visuals, type ProductShotName } from "../../art/visuals";
import { Visual } from "../../art/visual";
import { Mark, Scrim, Shot, ShotPanel } from "../parts";
import { cq, display, gradientText, ink, monoUp, scrim } from "../styles";

const asShot = (v: string): ProductShotName =>
  (visuals.find(x => x.family === "product" && x.name === v)?.name as ProductShotName) ?? "desktop";

function Counter({ count }: { count: string }) {
  return (
    <span
      data-field="count"
      className="font-mono text-muted-foreground"
      style={{ fontSize: "2cqi", letterSpacing: ".08em" }}
    >
      {count}
    </span>
  );
}

/** First slide: the pain of the whole carousel in one line, the desktop as the only image. */
export function CarouselCover({
  kicker = "Why a control plane",
  title = "Five terminals and the prompt in the wrong window.",
  count = "01 / 06",
}: {
  kicker?: string;
  title?: string;
  count?: string;
}) {
  return (
    <div className={ink}>
      <Shot name="desktop" w={104} style={{ left: cq(8.3), bottom: cq(-6) }} />
      <div className="absolute flex flex-col justify-between" style={{ inset: "6.7% 8.3%" }}>
        <div className="flex items-center justify-between">
          <Mark size={3.6} />
          <Counter count={count} />
        </div>
        <div style={{ marginBottom: "auto", marginTop: "10cqi" }}>
          <p
            data-field="kicker"
            className={`${monoUp} text-brand-cyan`}
            style={{ fontSize: "2.2cqi", letterSpacing: ".12em" }}
          >
            {kicker}
          </p>
          <p
            data-field="title"
            className={display}
            style={{ fontSize: "8.6cqi", lineHeight: 1, marginTop: "3cqi" }}
          >
            {title}
          </p>
        </div>
      </div>
    </div>
  );
}

/** Body slide: step number, a short claim, two lines of explanation and one product capture. */
export function CarouselBody({
  step = "02",
  title = "One desktop for every agent.",
  body = "Claude Code, Codex, Gemini CLI, OpenCode and Cursor in one control plane. You see all of them, and command all of them, from the same place.",
  visual = "desktop",
  count = "02 / 06",
}: {
  step?: string;
  title?: string;
  body?: string;
  visual?: string;
  count?: string;
}) {
  return (
    <div className={ink}>
      <div className="absolute flex flex-col" style={{ inset: "6.7% 8.3%" }}>
        <div className="flex items-center justify-between">
          <Mark size={2.8} variant="symbol" />
          <Counter count={count} />
        </div>
        <p
          data-field="step"
          className="m-0 w-max font-mono"
          style={{
            fontSize: "10cqi",
            lineHeight: 1,
            letterSpacing: "-.03em",
            marginTop: "6cqi",
            ...gradientText,
          }}
        >
          {step}
        </p>
        <p
          data-field="title"
          className={display}
          style={{ fontSize: "6.8cqi", lineHeight: 1.04, marginTop: "3cqi" }}
        >
          {title}
        </p>
        <p
          data-field="body"
          className="m-0 text-muted-foreground"
          style={{ fontSize: "3cqi", lineHeight: 1.4, marginTop: "2.6cqi" }}
        >
          {body}
        </p>
        <div style={{ marginTop: "auto" }}>
          <ShotPanel name={asShot(visual)} w={83.4} h={41} pad={1} />
        </div>
      </div>
    </div>
  );
}

/** Last slide: one action, where to read more; the install line is a small detail. */
export function CarouselCta({
  title = "Run every agent from one desktop.",
  command = "brew install compozy/compozy/compozy",
  link = "compozy.com",
  count = "06 / 06",
}: {
  title?: string;
  command?: string;
  link?: string;
  count?: string;
}) {
  return (
    <div className={ink}>
      <Visual name="field" x={0.5} y={0.3} />
      <Scrim
        style={scrim("0deg", [
          [0, 1],
          [42, 1],
          [70, 0],
        ])}
      />
      <div className="absolute flex flex-col" style={{ inset: "6.7% 8.3%" }}>
        <div className="flex items-center justify-between">
          <Mark size={2.8} variant="symbol" />
          <Counter count={count} />
        </div>
        <div style={{ marginTop: "auto" }}>
          <p data-field="title" className={display} style={{ fontSize: "9cqi", lineHeight: 1 }}>
            {title}
          </p>
          <div className="flex items-center justify-between" style={{ marginTop: "6cqi" }}>
            <Mark size={4} />
            <span
              data-field="link"
              className="font-mono text-brand-cyan"
              style={{ fontSize: "2.6cqi" }}
            >
              {link}
            </span>
          </div>
          <p className="m-0 font-mono text-faint" style={{ marginTop: "4cqi", fontSize: "2cqi" }}>
            <span>$ </span>
            <span data-field="command">{command}</span>
          </p>
        </div>
      </div>
    </div>
  );
}
