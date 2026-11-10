import { card, h3, lab } from "../cls";
import { Frame } from "../kit-frame";
import { Section } from "../parts/section";
import { ogImpl, ogRules, ogSnippet } from "./data";
import { Rows } from "./rows";

export function Og() {
  return (
    <Section
      id="og"
      n="01"
      file="lib/og"
      title={
        <>
          Every link gets a <em>real</em> image
        </>
      }
      status="proposal"
      intro={
        <p>
          A Compozy link shared in X, Slack or a Discord channel is often the first thing anyone
          sees of the brand. Four templates cover every page; the site renders them at build time,
          one PNG per URL.
        </p>
      }
    >
      <div className="grid gap-6 md:grid-cols-2">
        <Frame id="og-default" guides />
        <Frame id="og-blog" guides />
        <Frame id="og-docs" guides />
        <Frame id="og-changelog" guides />
      </div>
      <div className="mt-10 grid gap-4 lg:grid-cols-2">
        <div className={card}>
          <p className={lab}>Link image rules</p>
          <Rows rows={ogRules} className="mt-3" />
        </div>
        <div className={card}>
          <p className={lab}>In a feed</p>
          <div className="mt-4 max-w-[520px] rounded-[16px] border border-(--b-line) bg-rail p-4">
            <div className="flex items-center gap-2.5">
              <span className="h-9 w-9 rounded-full bg-elevated" />
              <div>
                <p className="m-0 text-[14px] font-semibold text-fg">Pedro Nauck</p>
                <p className="m-0 font-mono text-[11.5px] text-muted-foreground">
                  @pedronauck · 2h
                </p>
              </div>
            </div>
            <p className="m-0 mt-3 text-[14.5px] text-fg">
              Wrote down why every loop needs a definition of done, and what happens when the budget
              runs out.
            </p>
            <div className="mt-3 overflow-hidden rounded-[14px] border border-line">
              <Frame id="og-blog" caption={false} />
            </div>
            <p className="m-0 mt-2 font-mono text-[11.5px] text-muted-foreground">
              From compozy.com
            </p>
          </div>
          <p className="m-0 mt-4 text-[14px] text-(--b-muted)">
            At 500 px wide the title still reads at 22 px. That is the size to judge by, not the
            1200 px export.
          </p>
        </div>
      </div>
      <h3 className={`${h3} mt-16`}>How the site consumes them</h3>
      <p className="m-0 mb-5 max-w-[66ch] text-[15px] text-(--b-muted)">
        The Next.js site already renders OG images with next/og in packages/site/lib/og. The rebrand
        changes its tokens, fonts and backgrounds, not its routes.
      </p>
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.15fr)]">
        <div className={card}>
          <Rows rows={ogImpl} />
        </div>
        <pre className="m-0 self-start overflow-x-auto rounded-card border border-(--b-line) bg-canvas p-6 font-mono text-[12.5px] leading-[1.7] text-fg">
          {ogSnippet}
        </pre>
      </div>
    </Section>
  );
}
