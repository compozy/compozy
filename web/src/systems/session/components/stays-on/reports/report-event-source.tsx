/**
 * S13: the event behind a proposal, read before any choice. The summary is untrusted event data:
 * shown as plain text, never as Markdown or a link.
 */
import { Zap } from "@compozy/ui/icons";

import { Eyebrow, Item } from "@compozy/ui";

import type { ReportEventSource } from "../../../lib/session-report";

export function ReportEventSourceLine({ source }: { source: ReportEventSource }) {
  return (
    <div className="flex flex-col gap-1.5" data-testid="session-report-event-source">
      <Eyebrow className="text-subtle">Prompted by an event</Eyebrow>
      <Item className="items-start gap-2.5 px-0 py-0">
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
      </Item>
    </div>
  );
}
