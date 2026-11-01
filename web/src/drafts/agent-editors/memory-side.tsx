/** S9's right column: how full memory is, what got put away, and history. */
import { Archive, History } from "lucide-react";

import {
  Button,
  Disclosure,
  StackedProgress,
  StatusDot,
  Timeline,
  TimelineEvent,
  cn,
} from "@compozy/ui";

import { EditorCard } from "../stays-on-agents-shared/rich-controls";
import { PUT_AWAY, REVISIONS } from "./memory-fixtures";

export type Fullness = "fine" | "almost" | "full";

const FULLNESS_COPY: Record<Fullness, string> = {
  fine: "Plenty of room.",
  almost: "When it fills up, the oldest learned notes get put away. Yours stay.",
  full: "It's all notes you wrote, so nothing can be put away and it can't learn more. Remove a few to make room.",
};

export function FullnessCard({
  percent,
  fullness,
  learned,
  yours,
  onShowYours,
}: {
  percent: number;
  fullness: Fullness;
  learned: number;
  yours: number;
  onShowYours: () => void;
}) {
  const share = learned + yours === 0 ? 0 : learned / (learned + yours);
  const tone = fullness === "full" ? "danger" : fullness === "almost" ? "warning" : "neutral";
  return (
    <EditorCard className="gap-3">
      <div className="flex items-baseline gap-1.5">
        <span
          className={cn(
            "text-display-2xl font-medium tabular-nums",
            fullness === "full" ? "text-danger" : fullness === "almost" ? "text-warning" : "text-fg"
          )}
        >
          {percent}%
        </span>
        <span className="text-small-body text-muted">full</span>
      </div>
      <StackedProgress
        ariaLabel={`Memory is ${percent}% full`}
        className="h-2"
        segments={[
          { value: Math.round(percent * share), tone, label: "Learned" },
          {
            value: Math.round(percent * (1 - share)),
            tone: fullness === "full" ? "danger" : "info",
            label: "Yours",
          },
        ]}
        total={100}
      />
      <div className="flex gap-3 text-eyebrow text-muted tabular-nums empty:hidden">
        {learned > 0 ? (
          <span className="flex items-center gap-1.5">
            <StatusDot size="sm" tone={tone === "neutral" ? "faint" : tone} />
            {learned} learned
          </span>
        ) : null}
        {yours > 0 ? (
          <span className="flex items-center gap-1.5">
            <StatusDot size="sm" tone={fullness === "full" ? "danger" : "info"} />
            {yours} yours
          </span>
        ) : null}
      </div>
      <p className="text-small-body text-fg-2">{FULLNESS_COPY[fullness]}</p>
      {fullness === "full" ? (
        <Button
          className="self-start"
          onClick={onShowYours}
          size="sm"
          type="button"
          variant="secondary"
        >
          Show my notes
        </Button>
      ) : null}
    </EditorCard>
  );
}

export function PutAwayCard({ count }: { count: number }) {
  return (
    <EditorCard className="gap-2 py-3">
      <Disclosure
        label={
          <span className="flex items-center gap-2">
            <Archive aria-hidden="true" className="size-3.5" />
            {count} older notes put away
          </span>
        }
        size="md"
      >
        <ul className="flex flex-col gap-1.5 text-small-body text-muted">
          {PUT_AWAY.map(note => (
            <li key={note}>{note}</li>
          ))}
        </ul>
      </Disclosure>
    </EditorCard>
  );
}

export function HistoryCard({
  restored,
  onRestore,
}: {
  restored: string | null;
  onRestore: (id: string, keep: number) => void;
}) {
  return (
    <EditorCard className="gap-4">
      <h2 className="flex items-center gap-2 text-item-title font-medium text-fg">
        <History aria-hidden="true" className="size-3.5 text-muted" />
        History
      </h2>
      <Timeline ariaLabel="Memory history">
        {REVISIONS.map((revision, index) => (
          <TimelineEvent
            key={revision.id}
            meta={
              index === 0 ? (
                "Current version"
              ) : restored === revision.id ? (
                "Back to this version, not saved yet"
              ) : (
                <Button
                  className="-ml-2"
                  onClick={() => onRestore(revision.id, revision.keep)}
                  size="xs"
                  type="button"
                  variant="ghost"
                >
                  Go back to this
                </Button>
              )
            }
            time={revision.when}
            title={revision.what}
          />
        ))}
      </Timeline>
    </EditorCard>
  );
}
