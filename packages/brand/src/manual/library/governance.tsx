import { cn } from "@compozy/ui/utils";

import { pages, VERSION } from "../chapters";
import { card, lab, rules } from "../cls";
import { Section } from "../parts/section";
import { StatusTag } from "../parts/status-tag";

const log: [version: string, date: string, items: string[]][] = [
  [
    "v1.1.0",
    "Nov 2026",
    [
      "The Echo art system is replaced by the visual system: Atmosphere (generated rasters) and Live (drawn from product state), with named visuals.",
      "Instrument Sans replaces Funnel Display for headlines, big numbers and the wordmark; Figtree and Geist Mono stay. Headlines run at weight 400.",
      "Blacks are neutral (ink #0a0a0a); the wordmark is redrawn in Instrument Sans.",
    ],
  ],
  [
    "v1.0.0",
    "Nov 2026",
    [
      "New identity replaces the orange knot: loop-with-echo symbol, Funnel Display / Figtree / Geist Mono, ink and the blue-to-cyan gradient.",
      "Twelve chapters, Echo art system, template kit with native export, digital asset set (favicons, OG, social banners and avatars).",
      "Migration table for the compozy repo in Product → Migration.",
    ],
  ],
  [
    "Next",
    "planned",
    [
      "Own vector drawing of the symbol before trademark filing.",
      "Approval pass on every chapter marked proposal.",
      "Rendered motion pieces (sting, end card) once the signatures are approved.",
    ],
  ],
];

export function Governance() {
  return (
    <Section
      id="governance"
      n="04"
      file="CHANGELOG.md"
      title={
        <>
          How the manual <em>changes</em>
        </>
      }
      bg="ink2"
      status="approved"
      intro={
        <p>
          Every page carries a status. Approved rules bind every piece. Proposals are direction: use
          them, and flag anything that doesn’t hold up in real work.
        </p>
      }
    >
      <div className="grid gap-4 lg:grid-cols-3">
        <div className={card}>
          <p className={lab}>Status</p>
          <div className="mt-4 grid gap-3 text-[15px]">
            <p className="m-0">
              <StatusTag status="approved" /> decided by the founder; binding.
            </p>
            <p className="m-0">
              <StatusTag status="proposal" /> written by the brand team; valid until replaced or
              approved.
            </p>
          </div>
          <ul className="m-0 mt-6 grid list-none gap-1.5 p-0 text-[14px]">
            {pages.map(p => (
              <li
                key={p.n}
                className="flex items-center justify-between border-b border-(--b-line) py-1.5"
              >
                <span>
                  {p.n} {p.title}
                </span>
                <StatusTag status={p.status} />
              </li>
            ))}
          </ul>
        </div>
        <div className={card}>
          <p className={lab}>Changing a rule</p>
          <ul className={cn(rules, "mt-4 text-[15px]")}>
            <li>Open a pull request against this manual, never a one-off file.</li>
            <li>
              Change the token, component or copy at its source (<code>brand/</code>,{" "}
              <code>data.ts</code>), not in a page.
            </li>
            <li>
              Bump the edition: patch for fixes, minor for new sections, major for a new symbol,
              palette or type.
            </li>
            <li>
              <code>bun run test</code> must pass: tokens in sync and every recommended pair at AA.
            </li>
            <li>A proposal becomes approved when the founder says so in the PR.</li>
          </ul>
        </div>
        <div className={card}>
          <p className={lab}>Changelog · current {VERSION}</p>
          <div className="mt-4 grid gap-6">
            {log.map(([v, d, items]) => (
              <div key={v}>
                <p className="m-0 font-mono text-[13px]">
                  <span className="text-(--b-accent)">{v}</span>{" "}
                  <span className="text-(--b-muted)">· {d}</span>
                </p>
                <ul className={cn(rules, "mt-2 text-[14px] text-(--b-muted)")}>
                  {items.map(i => (
                    <li key={i}>{i}</li>
                  ))}
                </ul>
              </div>
            ))}
          </div>
        </div>
      </div>
    </Section>
  );
}
