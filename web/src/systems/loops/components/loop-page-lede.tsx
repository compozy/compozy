import type { ReactNode } from "react";

import { Pill } from "@compozy/ui";

interface LoopPageLedeProps {
  name: string;
  /** Rendered muted before the name (`Run` on the run form). */
  prefix?: string;
  tags: readonly string[];
  /** Machine identity, demoted to micro mono — present, never leading. */
  meta: readonly string[];
  lede?: ReactNode;
  testId?: string;
}

/**
 * Page lede for a single Loop: what it is called, what it is, and its machine
 * identity at micro scale.
 *
 * The name repeats the breadcrumb on purpose — the crumb is navigation, this is the
 * subject of the page.
 */
export function LoopPageLede({ name, prefix, tags, meta, lede, testId }: LoopPageLedeProps) {
  return (
    <div className="flex flex-col gap-2 pt-4" data-testid={testId}>
      <div className="flex flex-wrap items-center gap-2">
        <h1 className="text-detail-h1 font-medium tracking-detail-h1 text-fg-strong">
          {prefix ? <span className="text-muted">{prefix} </span> : null}
          {name}
        </h1>
        {tags.map(tag => (
          <Pill key={tag} size="xs" tone="neutral">
            {tag}
          </Pill>
        ))}
      </div>
      {meta.length > 0 ? (
        <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-small-body text-subtle">
          {meta.map((entry, index) => (
            <span className="contents" key={entry}>
              {index > 0 ? <span aria-hidden="true">·</span> : null}
              <span className={index === 0 ? "font-mono" : undefined}>{entry}</span>
            </span>
          ))}
        </div>
      ) : null}
      {lede ? <p className="max-w-prose text-small-body text-muted">{lede}</p> : null}
    </div>
  );
}
