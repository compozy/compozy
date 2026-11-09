// Social posts: X, LinkedIn, release, GitHub, README, YouTube thumbnail and the vertical cover.
// Product pieces show a real capture of the web/ UI (ADR-005) beside the copy.
import { Visual } from "../../art/visual";
import { Mark, Scrim, Shot } from "../parts";
import { cq, display, gradientFill, ink, monoUp, scrim } from "../styles";

const leftFade = (end: number) =>
  scrim("90deg", [
    [0, 1],
    [end, 1],
    [100, 0],
  ]);

/** X post, 1600×900. Level 2: one headline, the always-on agents on the right, mono metadata. */
export function XPost({
  kicker = "Release 0.6",
  title = "Agents that check in on their own.",
  meta = "compozy.com/changelog",
}: {
  kicker?: string;
  title?: string;
  meta?: string;
}) {
  return (
    <div className={ink}>
      <Shot name="agents" w={44} style={{ right: cq(4), top: cq(20) }} />
      <Scrim className="right-auto" style={{ ...leftFade(80), width: cq(54) }} />
      <div className="absolute flex flex-col justify-between" style={{ inset: cq(6.25) }}>
        <Mark size={2.6} className="self-start" />
        <div>
          <p
            data-field="kicker"
            className={`${monoUp} text-brand-cyan`}
            style={{ fontSize: "1.25cqi", letterSpacing: ".12em" }}
          >
            {kicker}
          </p>
          <p
            data-field="title"
            className={display}
            style={{ fontSize: "4.8cqi", lineHeight: 1.02, maxWidth: "8.6em", marginTop: "1.6cqi" }}
          >
            {title}
          </p>
        </div>
        <p
          data-field="meta"
          className="m-0 font-mono text-muted-foreground"
          style={{ fontSize: "1.1cqi", letterSpacing: ".06em" }}
        >
          {meta}
        </p>
      </div>
    </div>
  );
}

/** Quote card, 1600×900. Only real, attributed quotes from a named person who agreed to it. */
export function XQuote({
  quote = "Companies run agents with no repeatable process. Every person ends up inventing their own stack.",
  name = "Pedro Nauck",
  role = "Founder, Compozy",
}: {
  quote?: string;
  name?: string;
  role?: string;
}) {
  return (
    <div className={ink}>
      <Visual name="cone" x={0.95} y={0.1} />
      <Scrim
        style={scrim("90deg", [
          [0, 1],
          [55, 0.9],
          [100, 0.2],
        ])}
      />
      <div
        className="absolute flex flex-col justify-between"
        style={{ inset: "11.1% 6.25% 11.1% 6.25%" }}
      >
        <span
          className="block font-display text-brand-cyan"
          style={{ fontSize: "9cqi", lineHeight: 0.7 }}
        >
          “
        </span>
        <p
          data-field="quote"
          className={`${display} tracking-[-0.025em]`}
          style={{ fontSize: "4.3cqi", lineHeight: 1.12, maxWidth: "18em" }}
        >
          {quote}
        </p>
        <div className="flex items-end justify-between">
          <div className="flex items-center" style={{ gap: "1.6cqi" }}>
            <span className="block" style={{ width: "4cqi", height: ".25cqi", ...gradientFill }} />
            <span className="font-sans" style={{ fontSize: "1.5cqi" }}>
              <span data-field="name" className="font-semibold text-fg">
                {name}
              </span>{" "}
              <span data-field="role" className="text-muted-foreground">
                · {role}
              </span>
            </span>
          </div>
          <Mark size={2.4} />
        </div>
      </div>
    </div>
  );
}

/** LinkedIn single image, 1200×1200. The pain and the claim on top, the sessions board low. */
export function LinkedinPost({
  kicker = "Field note",
  title = "The bottleneck stopped being the model.",
  body = "It became managing the sessions. Compozy is one place to see every agent, who has the next step, and what needs you.",
  meta = "compozy.com",
}: {
  kicker?: string;
  title?: string;
  body?: string;
  meta?: string;
}) {
  return (
    <div className={ink}>
      <Shot name="board" w={92} style={{ left: cq(4), bottom: cq(-3) }} />
      <div
        className="absolute flex flex-col"
        style={{ top: cq(7.5), left: cq(7.5), right: cq(7.5) }}
      >
        <div className="flex items-center justify-between">
          <Mark size={3.2} />
          <span
            data-field="meta"
            className="font-mono text-muted-foreground"
            style={{ fontSize: "1.6cqi", letterSpacing: ".06em" }}
          >
            {meta}
          </span>
        </div>
        <p
          data-field="kicker"
          className={`${monoUp} text-brand-cyan`}
          style={{ fontSize: "1.7cqi", letterSpacing: ".12em", marginTop: "6cqi" }}
        >
          {kicker}
        </p>
        <p
          data-field="title"
          className={display}
          style={{ fontSize: "7cqi", lineHeight: 1.02, marginTop: "2cqi", maxWidth: "10em" }}
        >
          {title}
        </p>
        <p
          data-field="body"
          className="m-0 text-muted-foreground"
          style={{ fontSize: "2.4cqi", lineHeight: 1.4, marginTop: "2.6cqi", maxWidth: "24em" }}
        >
          {body}
        </p>
      </div>
    </div>
  );
}

/** Release card, 1600×900: version, summary, three changes and the notes link beside the desktop. */
export function ReleaseCard({
  version = "v0.6.0",
  title = "Always-on agents, handoff and approvals.",
  changes = "Always-on agents check in on their own and call you only when needed;Continue a session with another agent, files and worktree included;Approve once, always or deny from the permission dock",
  link = "github.com/compozy/compozy/releases",
}: {
  version?: string;
  title?: string;
  changes?: string;
  link?: string;
}) {
  return (
    <div className={ink}>
      <Shot name="desktop" w={74} style={{ right: cq(-8), top: cq(14) }} />
      <Scrim className="right-auto" style={{ ...leftFade(80), width: cq(52) }} />
      <div
        className="absolute flex flex-col justify-between"
        style={{ top: "11.1%", bottom: "11.1%", left: "6.25%", width: cq(40) }}
      >
        <Mark size={2.6} className="self-start" />
        <div>
          <p
            data-field="version"
            className="m-0 font-mono text-brand-cyan"
            style={{ fontSize: "2cqi", letterSpacing: ".06em" }}
          >
            {version}
          </p>
          <p
            data-field="title"
            className={display}
            style={{ fontSize: "4.2cqi", lineHeight: 1.04, marginTop: "1.4cqi" }}
          >
            {title}
          </p>
          <div
            data-field="changes"
            className="flex flex-col"
            style={{ gap: "1cqi", marginTop: "3cqi" }}
          >
            {changes.split(";").map((c, i) => (
              <div key={c} className="flex" style={{ gap: "1.4cqi" }}>
                <span
                  className="font-mono text-brand-cyan"
                  style={{ fontSize: "1.1cqi", paddingTop: ".3cqi" }}
                >
                  0{i + 1}
                </span>
                <span
                  className="text-muted-foreground"
                  style={{ fontSize: "1.45cqi", lineHeight: 1.35 }}
                >
                  {c}
                </span>
              </div>
            ))}
          </div>
        </div>
        <p
          data-field="link"
          className="m-0 font-mono text-muted-foreground"
          style={{ fontSize: "1.15cqi", letterSpacing: ".05em" }}
        >
          {link}
        </p>
      </div>
    </div>
  );
}

/** GitHub social preview, 1280×640. Unfurls shrink it to ~500 px, so text stays above 2.6cqi. */
export function GithubPreview({
  title = "The autonomous control plane for your agents.",
  repo = "compozy/compozy",
  meta = "Go · MIT · beta",
}: {
  title?: string;
  repo?: string;
  meta?: string;
}) {
  return (
    <div className={ink}>
      <Shot name="desktop" w={58} style={{ right: cq(-10), top: cq(12) }} />
      <Scrim className="right-auto" style={{ ...leftFade(85), width: cq(52) }} />
      <div
        className="absolute flex flex-col justify-between"
        style={{ top: "12.5%", bottom: "12.5%", left: "6.25%", width: cq(42) }}
      >
        <Mark size={3.6} className="self-start" />
        <p data-field="title" className={display} style={{ fontSize: "4.8cqi", lineHeight: 1.02 }}>
          {title}
        </p>
        <p className="m-0 flex items-center font-mono" style={{ fontSize: "1.8cqi", gap: "2cqi" }}>
          <span data-field="repo" className="text-fg">
            {repo}
          </span>
          <span data-field="meta" className="text-muted-foreground">
            {meta}
          </span>
        </p>
      </div>
    </div>
  );
}

/** README header, 1280×320, shown ~830 px wide on light and dark GitHub; carries its own ink. */
export function ReadmeHeader({
  title = "The autonomous control plane for your agents.",
}: {
  title?: string;
}) {
  return (
    <div className={ink} style={{ borderRadius: "1.2cqi" }}>
      <Shot name="desktop" w={46} style={{ right: cq(-3), top: cq(4) }} />
      <Scrim className="right-auto" style={{ ...leftFade(85), width: cq(54) }} />
      <div
        className="absolute flex flex-col justify-center"
        style={{ top: "18%", bottom: "18%", left: "5%", width: cq(44) }}
      >
        <Mark size={3.4} className="self-start" />
        <p
          data-field="title"
          className={`${display} tracking-[-0.025em]`}
          style={{ fontSize: "3.4cqi", lineHeight: 1.1, marginTop: "2cqi" }}
        >
          {title}
        </p>
      </div>
    </div>
  );
}

/** YouTube thumbnail, 1280×720, read at 168 px: four huge words and the product; the
 *  bottom-right corner stays free for the timestamp. */
export function YoutubeThumb({
  title = "Five terminals, one desktop",
  kicker = "Compozy · 12 min",
}: {
  title?: string;
  kicker?: string;
}) {
  return (
    <div className={ink}>
      <Shot name="desktop" w={76} style={{ right: cq(-14), top: cq(12) }} />
      <Scrim className="right-auto" style={{ ...leftFade(85), width: cq(52) }} />
      <div className="absolute" style={{ left: cq(5), top: cq(7.5), width: cq(44) }}>
        <p
          data-field="kicker"
          className={`${monoUp} text-brand-cyan`}
          style={{ fontSize: "1.9cqi", letterSpacing: ".1em" }}
        >
          {kicker}
        </p>
        <p
          data-field="title"
          className={display}
          style={{ fontSize: "8cqi", lineHeight: 0.98, marginTop: "2cqi" }}
        >
          {title}
        </p>
      </div>
    </div>
  );
}

/** Vertical cover, 1080×1920. The app covers the top 13% and the bottom 18%. */
export function Story({
  kicker = "Live now",
  title = "Watch twelve agents ship one release.",
  meta = "youtube.com/@compozy",
}: {
  kicker?: string;
  title?: string;
  meta?: string;
}) {
  return (
    <div className={ink}>
      <Visual name="swarm" x={0.6} y={0.3} />
      <Scrim
        style={scrim("0deg", [
          [0, 1],
          [30, 1],
          [52, 0.7],
          [74, 0],
        ])}
      />
      <div className="absolute flex flex-col justify-end" style={{ inset: "13% 8.3% 18% 8.3%" }}>
        <p
          data-field="kicker"
          className={`${monoUp} flex items-center text-brand-cyan`}
          style={{ fontSize: "2.6cqi", letterSpacing: ".12em", gap: "1.6cqi" }}
        >
          <span
            className="block rounded-full bg-brand-cyan"
            style={{ width: "1.8cqi", height: "1.8cqi" }}
          />
          {kicker}
        </p>
        <p
          data-field="title"
          className={display}
          style={{ fontSize: "10cqi", lineHeight: 1, marginTop: "3.4cqi" }}
        >
          {title}
        </p>
        <div className="flex items-center justify-between" style={{ marginTop: "7cqi" }}>
          <Mark size={4.4} />
          <span
            data-field="meta"
            className="font-mono text-muted-foreground"
            style={{ fontSize: "2.4cqi" }}
          >
            {meta}
          </span>
        </div>
      </div>
    </div>
  );
}
