// 06 CLI and API: secondary surfaces. Compact terminal palette, help output, machine-output rules.
// The palette has one source: public/brand/product/terminal-palette.json, served for download
// and imported here for the swatches and contrast checks.
import { cn } from "@compozy/ui/utils";
import type { ReactNode } from "react";

import palette from "../../../public/brand/product/terminal-palette.json";
import { cap, h3, lab, rules } from "../cls";
import { grade, ratio } from "../contrast";
import { DoDont } from "../parts/do-dont";
import { Section } from "../parts/section";
import { numberLines } from "../text-runs";
import { cliHelp } from "./data";

type Ansi = Record<string, { hex: string; name: string; role: string }>;
const dark = palette.dark.ansi as Ansi;
const light = palette.light.ansi as Ansi;
const keys = Object.keys(dark);
const sets: { name: string; set: Ansi; bg: string; paper: boolean }[] = [
  { name: "Dark", set: dark, bg: palette.dark.background, paper: false },
  { name: "Light", set: light, bg: palette.light.background, paper: true },
];

// help output, coloured by role: commands cyan (ANSI 6), flags violet (ANSI 5), paths and the
// placeholder bright blue (ANSI 12), meta in ANSI 8, headings bright white
const ink = {
  head: "text-(--color-fg-strong) font-semibold",
  meta: "text-(--color-faint)",
  body: "text-(--color-fg-3)",
};
const HELP_LINES = numberLines(cliHelp);

const ansi = (k: string) => ({ color: dark[k]?.hex });

function colourHelp(l: string, n: number): ReactNode {
  if (n === 1)
    return (
      <>
        <span className={ink.head}>compozy</span> <span className={ink.meta}>0.6.0 ·</span>{" "}
        <span className={ink.body}>the autonomous control plane for your agents</span>
      </>
    );
  if (/^[A-Z]+$/.test(l)) return <span className={ink.head}>{l}</span>;
  if (l.startsWith("  compozy"))
    return (
      <>
        {"  "}
        <span style={ansi("6")}>compozy</span> <span style={ansi("12")}>&lt;command&gt;</span>{" "}
        <span className={ink.meta}>[flags]</span>
      </>
    );
  const m = l.match(/^(\s+)(-\S.*?|[a-z]+)(\s{2,})(.*)$/);
  if (m) {
    const [, lead = "", name = "", gap = "", desc = ""] = m;
    const d = desc.match(/^(.*)\(default (\w+)\)$/);
    return (
      <>
        {lead}
        <span style={ansi(name.startsWith("-") ? "5" : "6")}>{name}</span>
        {gap}
        <span className={ink.body}>
          {d ? d[1] : desc}
          {d && <span className={ink.meta}>(default {d[2]})</span>}
        </span>
      </>
    );
  }
  if (l.startsWith("Docs:"))
    return (
      <>
        <span className={ink.meta}>Docs:</span>{" "}
        <span className="underline" style={ansi("12")}>
          https://compozy.com/docs
        </span>
      </>
    );
  return l;
}

const json = `[
  {
    "id": "sess_01JCZ7F2",
    "name": "migrate-db",
    "agent": "codex",
    "state": "needs_you",
    "attention": "tool_approval",
    "updated_at": "2026-11-07T14:02:11Z"
  }
]`;

const pre =
  "m-0 overflow-x-auto rounded-[12px] bg-(--color-rail) p-4 font-mono text-[12.5px] leading-[1.55]";

export function Cli() {
  return (
    <Section
      id="cli"
      n="06"
      file="terminal-palette.json"
      title={
        <>
          CLI and API: <em>secondary</em> surfaces
        </>
      }
      bg="black"
      status="proposal"
      intro={
        <p>
          The desktop is the product; <code>compozy</code> and the API are for scripts, CI and
          people who live in a shell. They share the system (same state words, same glyphs, one
          palette) but they are never the image of the brand.
        </p>
      }
    >
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)]">
        <div className="grid gap-4">
          {sets.map(({ name, set, bg, paper }) => (
            <div
              key={name}
              className={cn(
                "min-w-0 rounded-card border border-(--b-line) p-5",
                paper && "band-paper"
              )}
              style={{ background: bg }}
            >
              <p className="m-0 mono-label text-[11px] text-(--b-muted)">
                {name} · 16 ANSI colours · bg {bg}
              </p>
              <div className="mt-4 grid grid-cols-8 gap-2">
                {keys.map(k => {
                  const c = set[k];
                  if (!c) return null;
                  const r = ratio(c.hex, bg);
                  return (
                    <div key={k} className="min-w-0">
                      <span
                        className="block aspect-square rounded-[8px] shadow-[inset_0_0_0_1px_var(--b-line)]"
                        style={{ background: c.hex }}
                        title={`${c.name} · ${c.role}`}
                      />
                      <span className="mt-1 block font-mono text-[10px] text-(--b-muted)">
                        {k} · {r.toFixed(1)}
                        {grade(r) === "fail" ? " —" : ""}
                      </span>
                    </div>
                  );
                })}
              </div>
            </div>
          ))}
          <p className={cn(cap, "mt-0!")}>
            Source:{" "}
            <a className="text-(--b-accent)" href="/brand/product/terminal-palette.json">
              brand/product/terminal-palette.json
            </a>
            . Numbers are contrast ratios against the background. Roles: cyan for commands and live,
            violet for flags, ANSI 8 for meta.
          </p>
        </div>
        <figure className="m-0 min-w-0">
          <p className={cn(lab, "mb-3")}>compozy --help</p>
          <pre className="m-0 overflow-x-auto rounded-card border border-(--b-line) bg-(--color-rail) p-5 font-mono text-[12.5px] leading-[1.6] text-(--color-fg)">
            {HELP_LINES.map(line => (
              <span key={line.n}>
                {colourHelp(line.text, line.n)}
                {"\n"}
              </span>
            ))}
          </pre>
          <figcaption className={cap}>
            Command names in cyan, flags in violet, defaults in ANSI 8. One plain sentence per
            description, sentence case, no full stop.
          </figcaption>
        </figure>
      </div>

      <h3 className={cn(h3, "mt-12")}>Machine output stays plain</h3>
      <DoDont
        className="mt-4"
        doLabel="compozy session list -o json"
        dontLabel="Colour or glyphs in JSON"
        doContent={<pre className={cn(pre, "text-(--color-fg)")}>{json}</pre>}
        dontContent={
          <pre className={pre}>
            <span className={ink.meta}>\x1b[33m</span>
            <span className="text-warning">◐ Needs you</span>
            {"\n"}
            <span className="text-(--color-fg)">{'{ "state": "'}</span>
            <span className="text-warning">\x1b[33mneeds you\x1b[0m</span>
            <span className="text-(--color-fg)">{'" }'}</span>
            {"\n"}
            <span className="text-(--color-brand-cyan)">⠋ loading…</span>
          </pre>
        }
      />
      <ul className={cn(rules, "mt-8 max-w-[76ch] text-[15.5px]")}>
        <li>
          <strong>-o json</strong> and <strong>-o yaml</strong> print data only: no ANSI codes,
          glyphs, spinners or banner, on stdout, one document.
        </li>
        <li>
          <strong>State fields</strong> carry the API’s wire values, never the display words or
          glyphs: words are for people, values are for scripts.
        </li>
        <li>
          <strong>NO_COLOR</strong> and a non-TTY stdout turn colour off;{" "}
          <code>--color=always</code> turns it back on for pagers.
        </li>
        <li>
          <strong>Not an image.</strong> Brand pieces show the desktop (Graphic system → Product
          UI). A terminal appears only as the “before” of a story.
        </li>
      </ul>
    </Section>
  );
}
