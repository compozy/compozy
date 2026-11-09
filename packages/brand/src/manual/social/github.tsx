import { cn } from "@compozy/ui/utils";

import { card, h3, lab } from "../cls";
import { Frame } from "../kit-frame";
import { Section } from "../parts/section";
import { githubRules, readmeSnippet } from "./data";
import { SpecCard } from "../parts/spec-card";

const README_LINE = (
  <p className="m-0 mt-4 text-[14px]">
    <b>Compozy</b> is the autonomous control plane for your agents: one desktop to see, command and
    approve every session across Claude Code, Codex and Cursor.
  </p>
);

/** The README rendered in GitHub's light and dark themes: paper band and ink band stand in. */
function ReadmeThemes() {
  return (
    <div className="mt-4 grid gap-4 lg:grid-cols-2">
      <div className="band-paper rounded-card border border-(--b-line) p-5">
        <p className="m-0 mb-3 mono-label text-[11.5px] tracking-[0.08em] text-(--b-muted)">
          Light theme
        </p>
        <Frame id="readme-header" caption={false} />
        {README_LINE}
      </div>
      <div className="band-ink rounded-card border border-line-strong p-5">
        <p className="m-0 mb-3 mono-label text-[11.5px] tracking-[0.08em] text-(--b-muted)">
          Dark theme
        </p>
        <Frame id="readme-header" caption={false} />
        {README_LINE}
      </div>
    </div>
  );
}

export function Github() {
  return (
    <Section
      id="github"
      n="06"
      file="github.md"
      title={
        <>
          GitHub: the <em>front door</em>
        </>
      }
      intro={
        <p>
          Most developers meet Compozy on GitHub, often through a link unfurled in Slack or X. The
          social preview, the org avatar and the README header are the first brand they see, so they
          get the same care as the site.
        </p>
      }
    >
      <div className="grid items-start gap-4 lg:grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)]">
        <Frame id="github-social-preview" guides />
        <div className={card}>
          <p className={lab}>Unfurled in Slack</p>
          <div className="mt-4 border-l-4 border-line-strong pl-3">
            <p className="m-0 text-[13px] font-semibold">GitHub</p>
            <p className="m-0 text-[14px] font-semibold text-(--b-accent)">compozy/compozy</p>
            <p className="m-0 text-[13px] text-(--b-muted)">
              The autonomous control plane for your agents. Local-first, MIT.
            </p>
            <div className="mt-2 max-w-[360px]">
              <Frame id="github-social-preview" caption={false} />
            </div>
          </div>
        </div>
      </div>

      <h3 className={cn(h3, "mt-16")}>README header</h3>
      <ReadmeThemes />
      <div className="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)]">
        <div className="min-w-0 overflow-hidden rounded-card border border-(--b-line) bg-(--b-card)">
          <p className="m-0 border-b border-(--b-line) px-5 py-3 font-mono text-[12.5px] text-(--b-muted)">
            README.md
          </p>
          <pre className="m-0 overflow-x-auto p-5 font-mono text-[12.5px] leading-[1.7]">
            <code>{readmeSnippet}</code>
          </pre>
          <p className="m-0 px-5 pb-5 text-[14px] text-(--b-muted)">
            The header carries its own ink ground and rounded corners, so it reads the same in both
            themes. Below it: the one-liner, install, and a product shot, in that order.
          </p>
        </div>
        <SpecCard title="Repository" rows={githubRules} />
      </div>
      <div className="mt-4 grid items-start gap-4 lg:grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)]">
        <Frame id="release-card" guides />
        <div className="grid gap-4">
          <div className="max-w-[200px]">
            <Frame id="github-avatar" />
          </div>
          <p className="m-0 text-[14.5px] text-(--b-muted)">
            The release card heads each GitHub release and is reused unchanged on X, LinkedIn and
            Discord, so one image announces the version everywhere.
          </p>
        </div>
      </div>
    </Section>
  );
}
