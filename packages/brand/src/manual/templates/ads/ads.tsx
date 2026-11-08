// Ads: one hook, one proof, one volt button. Brand pieces carry an Atmosphere visual; proof
// pieces show the mechanism in a real capture of the web/ UI (ADR-005).
import { Visual } from "../../art/visual";
import { cq, Cta, display, ink, Mark, monoUp, Scrim, scrim, Shot, ShotPanel } from "../parts";

const leftFade = (end: number) =>
  scrim("90deg", [
    [0, 1],
    [end, 1],
    [100, 0],
  ]);

/** Square brand ad, 1080×1080: one hook, the Field visual low right, a quiet CTA. */
export function AdSquareBrand({
  hook = "One desktop for every agent.",
  body = "The autonomous control plane for the agents your team already uses.",
  cta = "Install the beta",
  meta = "Beta · MIT · self-hosted",
}: {
  hook?: string;
  body?: string;
  cta?: string;
  meta?: string;
}) {
  return (
    <div className={ink}>
      <Visual name="field" x={0.6} y={0.85} />
      <Scrim
        style={scrim("180deg", [
          [0, 1],
          [38, 0.9],
          [70, 0],
        ])}
      />
      <div className="absolute flex flex-col" style={{ inset: cq(7.4) }}>
        <Mark size={4} className="self-start" />
        <p
          data-field="meta"
          className={`${monoUp} text-brand-cyan`}
          style={{ fontSize: "1.9cqi", letterSpacing: ".12em", marginTop: "9cqi" }}
        >
          {meta}
        </p>
        <p
          data-field="hook"
          className={display}
          style={{ fontSize: "9.6cqi", lineHeight: 0.98, maxWidth: "7.6em", marginTop: "2.4cqi" }}
        >
          {hook}
        </p>
        <p
          data-field="body"
          className="m-0 text-muted-foreground"
          style={{ fontSize: "3cqi", lineHeight: 1.4, maxWidth: "21em", marginTop: "3.4cqi" }}
        >
          {body}
        </p>
        <div className="mt-auto">
          <Cta label={cta} size={3} />
        </div>
      </div>
    </div>
  );
}

/** Square proof ad, 1080×1080: the approval dock, with the outcome in words. */
export function AdSquareProof({
  hook = "The agent asks. You decide.",
  proof = "Allow once, always allow or deny, right in the session, across every harness.",
  cta = "Install the beta",
}: {
  hook?: string;
  proof?: string;
  cta?: string;
}) {
  return (
    <div className={ink}>
      <div className="absolute flex flex-col" style={{ inset: cq(7.4) }}>
        <Mark size={4} className="self-start" />
        <p
          data-field="hook"
          className={display}
          style={{ fontSize: "8cqi", lineHeight: 1, maxWidth: "9em", marginTop: "6cqi" }}
        >
          {hook}
        </p>
        <div style={{ marginTop: "5cqi" }}>
          <ShotPanel name="approval" w={85.2} h={36} pad={2} />
        </div>
        <div className="mt-auto flex items-end justify-between" style={{ gap: "4cqi" }}>
          <p
            data-field="proof"
            className="m-0 text-muted-foreground"
            style={{ fontSize: "2.6cqi", lineHeight: 1.4, maxWidth: "20em" }}
          >
            {proof}
          </p>
          <Cta label={cta} size={2.6} />
        </div>
      </div>
    </div>
  );
}

/** Portrait feed ad, 1080×1350: hook on top, the orchestrator handing work to other harnesses,
 *  mechanism and CTA below. */
export function AdPortrait({
  hook = "Continue with another agent.",
  mechanism = "Hand a session from Claude Code to Codex with its context, files and worktree.",
  cta = "Install the beta",
  meta = "compozy.com",
}: {
  hook?: string;
  mechanism?: string;
  cta?: string;
  meta?: string;
}) {
  return (
    <div className={ink}>
      <div className="absolute flex flex-col" style={{ inset: cq(7.4) }}>
        <div className="flex items-center justify-between">
          <Mark size={4} />
          <span
            data-field="meta"
            className="font-mono text-muted-foreground"
            style={{ fontSize: "2cqi", letterSpacing: ".06em" }}
          >
            {meta}
          </span>
        </div>
        <p
          data-field="hook"
          className={display}
          style={{ fontSize: "9cqi", lineHeight: 0.98, maxWidth: "8em", marginTop: "6cqi" }}
        >
          {hook}
        </p>
        <div style={{ marginTop: "5cqi" }}>
          <ShotPanel name="orchestrate" w={85.2} h={42} pad={2} />
        </div>
        <div className="mt-auto">
          <p
            data-field="mechanism"
            className="m-0"
            style={{ fontSize: "3.1cqi", lineHeight: 1.4, maxWidth: "21em" }}
          >
            {mechanism}
          </p>
          <div style={{ marginTop: "4cqi" }}>
            <Cta label={cta} size={3} />
          </div>
        </div>
      </div>
    </div>
  );
}

/** Story ad, 1080×1920: content lives between the platform bars (top 14%, bottom 20%). */
export function AdStory({
  hook = "Agents that check in.",
  body = "Always-on agents work between triggers and call you only when they need a decision.",
  cta = "Install the beta",
}: {
  hook?: string;
  body?: string;
  cta?: string;
}) {
  return (
    <div className={ink}>
      <div
        className="absolute flex flex-col"
        style={{ top: cq(26), bottom: cq(37), left: cq(7.4), right: cq(7.4) }}
      >
        <Mark size={4.4} className="self-start" />
        <p
          data-field="hook"
          className={display}
          style={{ fontSize: "10cqi", lineHeight: 0.98, marginTop: "6cqi" }}
        >
          {hook}
        </p>
        <div style={{ marginTop: "6cqi" }}>
          <ShotPanel name="agents" w={85.2} h={56} pad={2} />
        </div>
        <p
          data-field="body"
          className="m-0 text-muted-foreground"
          style={{ fontSize: "3.4cqi", lineHeight: 1.4, marginTop: "6cqi" }}
        >
          {body}
        </p>
        <div style={{ marginTop: "4cqi" }}>
          <Cta label={cta} size={3.6} />
        </div>
      </div>
    </div>
  );
}

/** Landscape ad, 1200×628 (Google display, Reddit, Meta link): text left, the desktop right. */
export function AdLandscape({
  hook = "One desktop for every agent.",
  body = "See and command Claude Code, Codex and Cursor from one control plane. Always-on agents call you only when needed.",
  cta = "Install the beta",
}: {
  hook?: string;
  body?: string;
  cta?: string;
}) {
  return (
    <div className={ink}>
      <Shot name="desktop" w={64} style={{ right: cq(-6), top: cq(9) }} />
      <Scrim className="right-auto" style={{ ...leftFade(85), width: cq(52) }} />
      <div
        className="absolute flex flex-col"
        style={{ top: cq(6), bottom: cq(6), left: cq(6), width: cq(41) }}
      >
        <Mark size={2.8} className="self-start" />
        <p
          data-field="hook"
          className={display}
          style={{ fontSize: "5cqi", lineHeight: 1, marginTop: "auto" }}
        >
          {hook}
        </p>
        <p
          data-field="body"
          className="m-0 text-muted-foreground"
          style={{ fontSize: "1.8cqi", lineHeight: 1.4, marginTop: "2cqi" }}
        >
          {body}
        </p>
        <div style={{ marginTop: "3.4cqi" }}>
          <Cta label={cta} size={1.9} />
        </div>
      </div>
    </div>
  );
}

/** X website card, 800×418: X prints the domain over the bottom left, so the copy sits higher. */
export function AdXCard({
  hook = "Stop managing sessions.",
  meta = "Open source · beta",
}: {
  hook?: string;
  meta?: string;
}) {
  return (
    <div className={ink}>
      <Visual name="horizon" x={0.85} y={0.6} />
      <Scrim
        style={scrim("90deg", [
          [0, 1],
          [44, 0.86],
          [70, 0],
        ])}
      />
      <div
        className="absolute flex flex-col"
        style={{ top: cq(7), left: cq(6.5), width: cq(56), bottom: cq(14) }}
      >
        <Mark size={3.6} className="self-start" />
        <p
          data-field="meta"
          className={`${monoUp} text-brand-cyan`}
          style={{ fontSize: "1.7cqi", letterSpacing: ".12em", marginTop: "auto" }}
        >
          {meta}
        </p>
        <p
          data-field="hook"
          className={display}
          style={{ fontSize: "7cqi", lineHeight: 1, marginTop: "1.6cqi", maxWidth: "6.4em" }}
        >
          {hook}
        </p>
      </div>
    </div>
  );
}

/** LinkedIn single image, 1200×627: the sessions board, read by an engineering manager. */
export function AdLinkedin({
  hook = "Every agent session, one board.",
  body = "See who holds the next step across Claude Code, Codex and Cursor. Approve, hand off or step in from one place.",
  cta = "See what ships today",
}: {
  hook?: string;
  body?: string;
  cta?: string;
}) {
  return (
    <div className={ink}>
      <div className="absolute -translate-y-1/2" style={{ left: cq(43), top: "50%" }}>
        <ShotPanel name="board" w={51} h={30} radius={1.4} fit="cover" x={0} y={0.4} />
      </div>
      <div
        className="absolute flex flex-col"
        style={{ top: cq(6), bottom: cq(6), left: cq(6), width: cq(36) }}
      >
        <Mark size={2.8} className="self-start" />
        <p
          data-field="hook"
          className={display}
          style={{ fontSize: "4.4cqi", lineHeight: 1.02, marginTop: "auto" }}
        >
          {hook}
        </p>
        <p
          data-field="body"
          className="m-0 text-muted-foreground"
          style={{ fontSize: "1.7cqi", lineHeight: 1.45, marginTop: "1.8cqi" }}
        >
          {body}
        </p>
        <div style={{ marginTop: "3cqi" }}>
          <Cta label={cta} size={1.8} />
        </div>
      </div>
    </div>
  );
}

/** Sponsor and dev-newsletter banner, 600×200: symbol, one line, a mono CTA, the agents cropped. */
export function AdDevBanner({
  hook = "One desktop for every agent.",
  cta = "Beta · MIT · compozy.com →",
}: {
  hook?: string;
  cta?: string;
}) {
  return (
    <div className={ink}>
      <Shot name="agents" w={36} style={{ right: cq(-3), top: cq(12) }} />
      <Scrim className="right-auto" style={{ ...leftFade(88), width: cq(72) }} />
      <div
        className="absolute flex items-center"
        style={{ top: 0, bottom: 0, left: cq(6), width: cq(60), gap: cq(4) }}
      >
        <Mark variant="symbol" size={11} className="flex-none" />
        <div className="min-w-0">
          <p
            data-field="hook"
            className={`${display} tracking-[-0.025em]`}
            style={{ fontSize: "4.6cqi", lineHeight: 1.05 }}
          >
            {hook}
          </p>
          <p
            data-field="cta"
            className="m-0 font-mono text-brand-cyan"
            style={{ fontSize: "2.1cqi", marginTop: "2.4cqi", letterSpacing: ".04em" }}
          >
            {cta}
          </p>
        </div>
      </div>
    </div>
  );
}
