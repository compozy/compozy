// 09 · 07 Delivery: codecs, frame rate, sizes, naming, where files live, how renders are made.
import { cn } from "@compozy/ui/utils";

import { card, h3, lab, rules } from "../cls";
import { Section } from "../parts/section";
import { delivery } from "./data";
import { SpecCard } from "../parts/spec-card";
import { TableHead } from "../parts/table-head";

const tree = `brand/motion.json            tokens (source of truth)
public/assets/motion/        published renders, by type
  sting/  end-card/  demo/  loops/
motion/src/                  HyperFrames compositions (mexy)
motion/cues/                 start.wav  done.wav  needs-you.wav`;

const checks = [
  "30 fps, constant frame rate",
  "Ink reads as ink, not pure black, on a calibrated display",
  "Every ease is out, inOut or loop from motion.json",
  "No bounce, no overshoot, no spinner",
  "Captions burned in for social cuts, .srt for YouTube",
  "Commands on screen match the current CLI exactly",
  "Master at -14 LUFS, true peak -1 dBTP",
  "End card holds 0.6 s after the last element settles",
];

export function Delivery() {
  return (
    <Section
      id="delivery"
      n="07"
      file="motion/delivery.md"
      title={
        <>
          One frame rate, named <em>files</em>
        </>
      }
      bg="black"
      status="proposal"
      intro={
        <p>
          Renders come out of code, not a timeline: compositions are written as HTML with
          HyperFrames and the mexy library, reading the same tokens as this page, then rendered to
          the files below. The current launch video was cut in Remotion and stays as is until it is
          replaced.
        </p>
      }
    >
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
        <div className="overflow-x-auto rounded-card border border-(--b-line) bg-(--b-card)">
          <table className="w-full min-w-[620px] border-collapse text-[14px]">
            <TableHead columns={["Deliverable", "Codec / container", "For"]} />
            <tbody>
              {delivery.codecs.map(([a, b, c]) => (
                <tr key={a} className="border-b border-(--b-line) last:border-0">
                  <td className="px-5 py-3 font-medium">{a}</td>
                  <td className="px-5 py-3 font-mono text-[12.5px]">{b}</td>
                  <td className="px-5 py-3 text-(--b-muted)">{c}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <SpecCard title="frame" rows={delivery.frame} listClassName="mt-3" />
      </div>

      <div className="mt-4 grid gap-4 lg:grid-cols-2">
        <div className={card}>
          <p className={lab}>file naming</p>
          <p className="m-0 mt-4 font-mono text-[15px] break-all text-(--b-accent)">
            {delivery.naming}
          </p>
          <ul className="m-0 mt-4 grid list-none gap-1.5 p-0 font-mono text-[12.5px] text-(--b-muted)">
            {delivery.examples.map(e => (
              <li key={e} className="break-all">
                {e}
              </li>
            ))}
          </ul>
          <ul className={cn(rules, "mt-5 text-[14.5px]")}>
            <li>
              Lower case, underscores between fields, no spaces: files travel through CLIs and URLs.
            </li>
            <li>
              Ratio as <code>16x9</code>, <code>4x5</code>, <code>9x16</code>; duration in seconds
              with one decimal for stings.
            </li>
            <li>
              Versions only go up. A published file is never overwritten; publish <code>v3</code>{" "}
              and retire <code>v2</code>.
            </li>
          </ul>
        </div>
        <div className={card}>
          <p className={lab}>where the files go</p>
          <pre className="m-0 mt-4 overflow-x-auto rounded-[12px] bg-rail p-4 font-mono text-[12.5px] leading-[1.75] text-muted-foreground">
            {tree}
          </pre>
          <p className="m-0 mt-4 text-[14px] text-(--b-muted)">
            Proposed layout. None of these renders is published yet; the library lists files only
            once they exist. Until then, the live demos on this page are the reference.
          </p>
        </div>
      </div>

      <div className={cn("mt-4", card)}>
        <h3 className={h3}>Before a render ships</h3>
        <ul className="m-0 mt-3 grid list-none gap-2 p-0 text-[15px] md:grid-cols-2">
          {checks.map(t => (
            <li key={t} className="flex gap-2.5">
              <span className="text-(--b-accent)">✓</span>
              <span>{t}</span>
            </li>
          ))}
        </ul>
      </div>
    </Section>
  );
}
