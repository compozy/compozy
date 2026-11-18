// Editorial media: blog hero, newsletter header, UI card, call background and wallpaper.
import { Visual } from "../../art/visual";
import { visualByName, visuals, type ProductShotName } from "../../art/visuals";
import { Mark, Scrim, Shot, ShotPanel } from "../parts";
import { cq, display, ink, monoUp, scrim } from "../styles";

/** Blog hero, 1600×840: art only, the title stays in HTML. Product posts show a capture of our
 *  UI (default: orchestrate); essays pass an Atmosphere visual (`art=terrain`). */
export function BlogHero({
  kicker = "Product · Orchestration",
  art = "orchestrate",
}: {
  kicker?: string;
  art?: string;
}) {
  const v = visuals.find(x => x.name === art) ?? visualByName("orchestrate");
  return (
    <div className={ink}>
      {v.family === "product" ? (
        <div className="absolute" style={{ left: cq(14), top: cq(5) }}>
          <ShotPanel
            name={v.name as ProductShotName}
            w={72}
            h={44}
            pad={0}
            radius={1.2}
            fit="cover"
            x={0.5}
            y={0}
          />
        </div>
      ) : (
        <Visual name={v.name} x={0.66} y={0.5} />
      )}
      <Scrim
        className="top-auto h-[34%]"
        style={scrim("180deg", [
          [0, 0],
          [100, 0.9],
        ])}
      />
      <p
        data-field="kicker"
        className={`${monoUp} absolute text-brand-cyan`}
        style={{ left: cq(5), bottom: cq(5), fontSize: "1.1cqi", letterSpacing: ".12em" }}
      >
        {kicker}
      </p>
    </div>
  );
}

/** Newsletter header, 1200×400: lockup, issue and date, the always-on faces cut by the right edge. */
export function NewsletterHeader({
  issue = "Issue 07",
  date = "November 2026",
  title = "What shipped, what broke, what is next.",
}: {
  issue?: string;
  date?: string;
  title?: string;
}) {
  return (
    <div className={ink}>
      <Shot name="agents" w={52} style={{ left: cq(50), top: cq(4) }} />
      <Scrim
        style={scrim("90deg", [
          [0, 1],
          [46, 0.92],
          [60, 0],
        ])}
      />
      <div
        className="absolute flex flex-col"
        style={{ top: cq(5), bottom: cq(5), left: cq(5), width: cq(44) }}
      >
        <div className="flex items-center" style={{ gap: "2cqi" }}>
          <Mark size={2.6} />
          <span
            className={`${monoUp} text-muted-foreground`}
            style={{ fontSize: "1.2cqi", letterSpacing: ".12em" }}
          >
            <span data-field="issue" className="text-brand-cyan">
              {issue}
            </span>{" "}
            · <span data-field="date">{date}</span>
          </span>
        </div>
        <p
          data-field="title"
          className={`${display} tracking-[-0.025em]`}
          style={{ fontSize: "3.8cqi", lineHeight: 1.02, marginTop: "auto" }}
        >
          {title}
        </p>
      </div>
    </div>
  );
}

/** UI card, 1600×900: one capture of our UI in a window over the Atmosphere cone, one caption
 *  and the lockup. Code as image is a secondary docs piece, never the product image. */
export function CodeCard({
  caption = "Approvals answer right in the session, wherever the agent runs.",
}: {
  caption?: string;
}) {
  return (
    <div className={ink}>
      <Visual name="cone" x={0.2} y={0.2} />
      <div
        className="absolute shadow-[0_2cqi_6cqi_color-mix(in_oklab,var(--color-black)_50%,transparent)]"
        style={{ top: cq(7), left: cq(8), borderRadius: "1.4cqi" }}
      >
        <ShotPanel name="approval" w={84} h={34.25} pad={4} radius={1.4} />
      </div>
      <div
        className="absolute flex items-center justify-between"
        style={{ left: cq(8), right: cq(8), bottom: cq(5) }}
      >
        <p data-field="caption" className="m-0 text-fg" style={{ fontSize: "1.5cqi" }}>
          {caption}
        </p>
        <Mark size={2.2} />
      </div>
    </div>
  );
}

/** Video call background, 1920×1080: the centre stays empty for the face; terrain at the edges. */
export function CallBackground() {
  return (
    <div className={ink}>
      <Visual name="terrain" x={0.5} y={0.5} />
      <div
        aria-hidden
        className="absolute inset-0"
        style={{
          backgroundImage:
            "radial-gradient(42% 60% at 50% 55%, var(--color-rail) 0%, var(--color-rail) 55%, transparent 100%)",
        }}
      />
      <div className="absolute" style={{ top: cq(4), left: cq(4) }}>
        <Mark size={2} />
      </div>
    </div>
  );
}

/** Desktop wallpaper, 2560×1440: Atmosphere field, a small one-colour mark bottom right. */
export function Wallpaper() {
  return (
    <div className={ink}>
      <Visual name="field" x={0.6} y={0.5} />
      <div className="absolute opacity-70" style={{ right: cq(3), bottom: cq(6) }}>
        <Mark variant="mono-white" size={1} />
      </div>
    </div>
  );
}
