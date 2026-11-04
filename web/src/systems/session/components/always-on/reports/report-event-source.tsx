/**
 * S13: the event behind a proposal, read before any choice. The summary is untrusted event data:
 * shown as plain text, never as Markdown or a link.
 */
import { ExternalLink, Plug, Zap } from "@compozy/ui/icons";

import { Eyebrow, PillLink } from "@compozy/ui";

import type { ReportEventSource, ReportItemSource } from "../../../lib/session-report";

export function ReportEventSourceLine({ source }: { source: ReportEventSource }) {
  return (
    <div className="flex flex-col gap-1.5" data-testid="session-report-event-source">
      <Eyebrow className="text-subtle">Prompted by an event</Eyebrow>
      <div className="flex items-start gap-2.5">
        <Zap aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-subtle" />
        <span className="flex min-w-0 flex-col gap-0.5">
          <span className="text-body text-fg">
            {source.triggerName || "A trigger"}
            {source.event ? (
              <span className="font-mono text-small-body text-muted-foreground">
                {" "}
                · {source.event}
              </span>
            ) : null}
          </span>
          {source.summary ? (
            <span className="line-clamp-3 text-small-body whitespace-pre-line text-muted-foreground">
              {source.summary}
            </span>
          ) : null}
        </span>
      </div>
    </div>
  );
}

interface SourceGroup {
  sourceId: string;
  sourceRef: string;
  items: ReportItemSource[];
}

function groupBySource(sources: readonly ReportItemSource[]) {
  const groups: SourceGroup[] = [];
  for (const source of sources) {
    const group = groups.find(item => item.sourceId === source.sourceId);
    if (group) group.items.push(source);
    else groups.push({ sourceId: source.sourceId, sourceRef: source.sourceRef, items: [source] });
  }
  return groups;
}

/**
 * S4: the check-in sources behind a proposal, "From New GitHub issues", with a link to each item
 * that has one. Item keys and links are source data: shown as text, opened only on a click.
 */
export function ReportItemSourcesLine({
  sources,
  titleOf,
}: {
  sources: readonly ReportItemSource[];
  /** The source's title: its kind's title or script name. */
  titleOf: (source: Pick<ReportItemSource, "sourceId" | "sourceRef">) => string;
}) {
  return (
    <div className="flex flex-col gap-2" data-testid="session-report-item-source">
      {groupBySource(sources).map(group => {
        const linked = group.items.filter(item => item.url);
        return (
          <div className="flex items-start gap-2.5" key={group.sourceId}>
            <Plug aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-subtle" />
            <span className="flex min-w-0 flex-col gap-1">
              <span className="text-body text-fg">{`From ${titleOf(group)}`}</span>
              {linked.length > 0 ? (
                <span className="flex flex-wrap gap-1.5">
                  {linked.map(item => {
                    const label = item.itemKey || "Open";
                    return (
                      <PillLink
                        key={item.itemKey}
                        mono
                        render={
                          <a aria-label={label} href={item.url} rel="noreferrer" target="_blank" />
                        }
                        tone="neutral"
                      >
                        {label}
                        <ExternalLink aria-hidden="true" className="size-3" />
                      </PillLink>
                    );
                  })}
                </span>
              ) : null}
            </span>
          </div>
        );
      })}
    </div>
  );
}
