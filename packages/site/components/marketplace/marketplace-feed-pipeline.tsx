import { Eyebrow } from "@compozy/ui";
import { Globe, GitBranch, Terminal } from "lucide-react";
import { skillEntries } from "@/lib/marketplace-catalog";

/**
 * The page's one claim, drawn instead of asserted: `catalog/*.json` is a single source that this
 * site and the reader's own daemon both render. The JSON shown is the first real entry in
 * `catalog/skills.json`, so the figure cannot describe a feed shape the repository does not have.
 */

const FEED_TABS = ["skills.json", "extensions.json", "mcp.json"] as const;

function feedPreview(): string {
  const entry = skillEntries[0];
  if (!entry) {
    throw new Error("catalog/skills.json must contain at least one entry");
  }
  return JSON.stringify(
    {
      entry_id: entry.entry_id,
      name: entry.name,
      version: entry.version,
      install_slug: entry.install_slug,
    },
    null,
    2
  );
}

function Output({
  icon: Icon,
  label,
  note,
  here = false,
}: {
  icon: typeof Globe;
  label: string;
  note?: string;
  here?: boolean;
}) {
  return (
    <div className="relative flex min-h-6.5 items-center gap-2 ps-6 before:absolute before:start-0 before:top-1/2 before:h-px before:w-4 before:bg-line-strong">
      <Icon aria-hidden className="size-3.5 shrink-0 text-muted" />
      <span className="font-mono text-small-body whitespace-nowrap text-fg">{label}</span>
      {here ? (
        <span className="inline-flex h-4.5 items-center rounded-full bg-accent-tint px-1.5">
          <Eyebrow className="text-accent">You are here</Eyebrow>
        </span>
      ) : null}
      {note ? <span className="text-small-body text-subtle">{note}</span> : null}
    </div>
  );
}

export function MarketplaceFeedPipeline() {
  return (
    <figure
      className="m-0"
      aria-label="One catalog feed renders both this page and your daemon's marketplace"
    >
      <div className="overflow-hidden rounded-xl border border-line-strong bg-canvas shadow-overlay">
        <div className="flex items-center gap-1.5 border-b border-line px-3.5 py-2">
          <GitBranch aria-hidden className="size-2.5 text-subtle opacity-70" />
          <Eyebrow className="text-subtle">compozy/compozy · catalog/</Eyebrow>
        </div>
        <div className="flex gap-0.5 border-b border-line bg-canvas-soft px-2">
          {FEED_TABS.map((tab, index) => (
            <span
              key={tab}
              className={
                index === 0
                  ? "relative px-2.5 pt-2 pb-1.5 font-mono text-badge text-fg after:absolute after:inset-x-2 after:-bottom-px after:h-0.5 after:rounded-full after:bg-fg"
                  : "px-2.5 pt-2 pb-1.5 font-mono text-badge text-subtle"
              }
            >
              {tab}
            </span>
          ))}
        </div>
        <pre className="m-0 overflow-x-auto px-4 py-3.5 font-mono text-badge leading-relaxed text-subtle">
          {feedPreview()}
        </pre>
      </div>

      <div className="relative ms-7.5 flex flex-col gap-2.5 pt-3.5 before:absolute before:start-0 before:top-0 before:bottom-3 before:w-px before:bg-line-strong">
        <Output icon={Globe} label="compozy.com/marketplace" here />
        <Output icon={Terminal} label="compozy marketplace" note="your daemon" />
      </div>

      <figcaption className="mt-4 ms-0.5 text-small-body leading-relaxed text-subtle">
        One feed, two surfaces — this page and your daemon render the same JSON.
      </figcaption>
    </figure>
  );
}
