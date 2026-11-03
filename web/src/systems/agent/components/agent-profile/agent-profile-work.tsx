/** What the agent has actually done: findings first, then the running log. */
import { Check } from "@compozy/ui/icons";

import { Panel, Pill, Section, Timeline, TimelineEvent, cn } from "@compozy/ui";

import {
  HOME_RUN_ROW_CLASS,
  HOME_RUN_SUBTITLE_CLASS,
  HOME_RUN_TITLE_CLASS,
} from "@/systems/dashboard/components/home-zone-shape";

import type { AgentFinding } from "../../lib/agent-reports";

export interface AgentWorkItem {
  id: string;
  title: string;
  detail: string;
  when: string;
  tone: "neutral" | "success" | "destructive";
}

export function FindingRow({
  finding,
  onOpen,
}: {
  finding: Pick<AgentFinding, "title" | "scope" | "when"> & { waiting?: boolean };
  onOpen: () => void;
}) {
  return (
    <button
      aria-label={`Open: ${finding.title}`}
      className={cn(
        HOME_RUN_ROW_CLASS,
        "w-full text-left transition-colors duration-base hover:bg-surface-2 focus-visible:shadow-focus-inset focus-visible:outline-none"
      )}
      onClick={onOpen}
      type="button"
    >
      <span className="min-w-0">
        <span className={HOME_RUN_TITLE_CLASS}>{finding.title}</span>
        <span className={HOME_RUN_SUBTITLE_CLASS}>
          <span className="truncate">
            {finding.scope} · {finding.when}
          </span>
        </span>
      </span>
      {finding.waiting ? (
        <Pill size="sm" tone="brand">
          Your call
        </Pill>
      ) : null}
    </button>
  );
}

export function AgentFindingsPanel({
  findings,
  onOpen,
}: {
  findings: readonly AgentFinding[];
  onOpen: (finding: AgentFinding) => void;
}) {
  if (findings.length === 0) return null;
  return (
    <Section count={findings.length} label="What it found">
      <Panel bodyClassName="p-0" className="overflow-hidden">
        <div className="divide-y divide-line-soft">
          {findings.map(finding => (
            <FindingRow finding={finding} key={finding.id} onOpen={() => onOpen(finding)} />
          ))}
        </div>
      </Panel>
    </Section>
  );
}

export function AgentRecentWork({
  items,
  onOpen,
}: {
  items: readonly AgentWorkItem[];
  onOpen: (id: string) => void;
}) {
  if (items.length === 0) return null;
  return (
    <Section label="Recent work">
      <Panel>
        <Timeline ariaLabel="Recent work" className="gap-3">
          {items.map(item => (
            <TimelineEvent
              description={item.detail}
              icon={item.tone === "success" ? Check : undefined}
              key={item.id}
              time={item.when}
              title={
                <button
                  className="text-left outline-none hover:underline focus-visible:shadow-focus-ring"
                  data-testid={`agent-session-link-${item.id}`}
                  onClick={() => onOpen(item.id)}
                  type="button"
                >
                  {item.title}
                </button>
              }
              tone={item.tone}
            />
          ))}
        </Timeline>
      </Panel>
    </Section>
  );
}
