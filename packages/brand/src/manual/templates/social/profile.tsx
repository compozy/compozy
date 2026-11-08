// Social profile assets: avatars and the network banners. Each banner keeps its text inside the
// strip every device shows; the Atmosphere visual carries the rest.
import { Visual } from "../../art/visual";
import { display, ink, Mark, monoUp, Scrim, scrim, SymbolArt } from "../parts";

/** Square avatar master. The symbol sits inside the inscribed circle, so circle and rounded
 *  crops keep it whole: 56cqi wide, inside the 70.7cqi safe square. */
export function Avatar({ tone = "ink" }: { tone?: string }) {
  return (
    <div className={tone === "black" ? "absolute inset-0 bg-black" : "absolute inset-0 bg-rail"}>
      <div className="absolute top-1/2 left-1/2 -translate-1/2">
        <SymbolArt width={56} />
      </div>
    </div>
  );
}

/** X header, 1500×500. The avatar covers the lower-left; mobile crops top and bottom. */
export function XHeader({
  title = "The autonomous control plane for your agents.",
  meta = "compozy.com · github.com/compozy/compozy",
}: {
  title?: string;
  meta?: string;
}) {
  return (
    <div className={ink}>
      <Visual name="terrain" x={0.1} y={0.55} />
      <Scrim
        style={scrim("90deg", [
          [0, 0],
          [36, 0.6],
          [58, 1],
        ])}
      />
      <div className="absolute flex flex-col justify-center" style={{ inset: "20% 6% 20% 40%" }}>
        <p data-field="title" className={display} style={{ fontSize: "4.4cqi", lineHeight: 1.04 }}>
          {title}
        </p>
        <p
          data-field="meta"
          className="m-0 font-mono text-muted-foreground"
          style={{ fontSize: "1.25cqi", letterSpacing: ".06em", marginTop: "2.2cqi" }}
        >
          {meta}
        </p>
      </div>
    </div>
  );
}

/** LinkedIn personal banner, 1584×396. The photo covers the lower-left quarter. */
export function LinkedinBanner({
  title = "Building the control plane for your agents.",
  meta = "Founder · compozy.com",
}: {
  title?: string;
  meta?: string;
}) {
  return (
    <div className={ink}>
      <Visual name="horizon" x={0.8} y={0.5} />
      <Scrim
        style={scrim("90deg", [
          [0, 1],
          [74, 1],
          [96, 0],
        ])}
      />
      <div className="absolute flex flex-col justify-center" style={{ inset: "16% 22% 16% 32%" }}>
        <Mark size={2.2} className="self-start" />
        <p
          data-field="title"
          className={display}
          style={{ fontSize: "3.6cqi", lineHeight: 1.05, marginTop: "1.8cqi", maxWidth: "15em" }}
        >
          {title}
        </p>
        <p
          data-field="meta"
          className={`${monoUp} text-muted-foreground`}
          style={{ fontSize: "1cqi", letterSpacing: ".08em", marginTop: "1.4cqi" }}
        >
          {meta}
        </p>
      </div>
    </div>
  );
}

/** LinkedIn company cover, 1128×191. One line in the middle band; art at the right edge. */
export function LinkedinCover({
  title = "The autonomous control plane for your agents.",
}: {
  title?: string;
}) {
  return (
    <div className={ink}>
      <Visual name="terrain" x={0.85} y={0.6} />
      <Scrim
        style={scrim("90deg", [
          [0, 1],
          [66, 1],
          [94, 0],
        ])}
      />
      <div className="absolute flex items-center" style={{ inset: "18% 30% 18% 26%" }}>
        <p
          data-field="title"
          className={`${display} tracking-[-0.025em]`}
          style={{ fontSize: "3.1cqi", lineHeight: 1.05 }}
        >
          {title}
        </p>
      </div>
    </div>
  );
}

/** YouTube channel art, 2560×1440. Only the centred 1546×423 strip shows on every device. */
export function YoutubeBanner({
  title = "Agents that keep running.",
  meta = "New videos every week · compozy.com",
}: {
  title?: string;
  meta?: string;
}) {
  return (
    <div className={ink}>
      <Visual name="converge" x={0.9} y={0.5} sizes="2560px" />
      <Scrim
        style={scrim("90deg", [
          [0, 1],
          [40, 1],
          [60, 0.85],
          [80, 0],
        ])}
      />
      <div
        className="absolute flex flex-col justify-center"
        style={{ inset: "35.3% 19.8% 35.3% 19.8%" }}
      >
        <Mark size={1.7} className="self-start" />
        <p
          data-field="title"
          className={display}
          style={{ fontSize: "3.3cqi", lineHeight: 1.02, marginTop: "1.3cqi" }}
        >
          {title}
        </p>
        <p
          data-field="meta"
          className={`${monoUp} text-muted-foreground`}
          style={{ fontSize: ".78cqi", letterSpacing: ".1em", marginTop: "1.1cqi" }}
        >
          {meta}
        </p>
      </div>
    </div>
  );
}

/** Discord server banner, 960×540, shown at ≈240×135 above the channel list. */
export function DiscordBanner({ title = "Community" }: { title?: string }) {
  return (
    <div className={ink}>
      <Visual name="cone" x={0.7} y={0.25} />
      <Scrim
        style={scrim("0deg", [
          [0, 1],
          [38, 0.7],
          [70, 0],
        ])}
      />
      <div
        className="absolute flex items-end justify-between"
        style={{ left: "6%", right: "6%", bottom: "10%" }}
      >
        <Mark size={6} />
        <p
          data-field="title"
          className={`${monoUp} text-brand-cyan`}
          style={{ fontSize: "3.6cqi", letterSpacing: ".1em" }}
        >
          {title}
        </p>
      </div>
    </div>
  );
}
