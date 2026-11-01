/** S9's right column: how full memory is, and history. */
import { History } from "@compozy/ui/icons";

import { Button, StackedProgress, StatusDot, Timeline, TimelineEvent, cn } from "@compozy/ui";

import type { MemoryFullness } from "../../lib/agent-memory-notes";
import { EditorCard } from "./rich-controls";

const FULLNESS_COPY: Record<MemoryFullness, string> = {
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
  fullness: MemoryFullness;
  learned: number;
  yours: number;
  onShowYours: () => void;
}) {
  const share = learned + yours === 0 ? 0 : learned / (learned + yours);
  const tone = fullness === "full" ? "destructive" : fullness === "almost" ? "warning" : "neutral";
  return (
    <EditorCard className="gap-3">
      <div className="flex items-baseline gap-1.5">
        <span
          className={cn(
            "text-display-2xl font-medium tabular-nums",
            fullness === "full"
              ? "text-destructive"
              : fullness === "almost"
                ? "text-warning"
                : "text-fg"
          )}
        >
          {percent}%
        </span>
        <span className="text-small-body text-muted-foreground">full</span>
      </div>
      <StackedProgress
        ariaLabel={`Memory is ${percent}% full`}
        className="h-2"
        segments={[
          { value: Math.round(percent * share), tone, label: "Learned" },
          {
            value: Math.round(percent * (1 - share)),
            tone: fullness === "full" ? "destructive" : "info",
            label: "Yours",
          },
        ]}
        total={100}
      />
      <div className="flex gap-3 text-eyebrow text-muted-foreground tabular-nums empty:hidden">
        {learned > 0 ? (
          <span className="flex items-center gap-1.5">
            <StatusDot size="sm" tone={tone === "neutral" ? "faint" : tone} />
            {learned} learned
          </span>
        ) : null}
        {yours > 0 ? (
          <span className="flex items-center gap-1.5">
            <StatusDot size="sm" tone={fullness === "full" ? "destructive" : "info"} />
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

export function HistoryCard({
  revisions,
  disabled,
  onRestore,
}: {
  /** Newest first. */
  revisions: readonly { id: string; name: string }[];
  /** Going back replaces the file, so edits on screen have to settle first. */
  disabled: boolean;
  onRestore: (id: string) => void;
}) {
  return (
    <EditorCard className="gap-4">
      <h2 className="flex items-center gap-2 text-item-title font-medium text-fg">
        <History aria-hidden="true" className="size-3.5 text-muted-foreground" />
        History
      </h2>
      <Timeline ariaLabel="Memory history">
        {revisions.map((revision, index) => (
          <TimelineEvent
            key={revision.id}
            meta={
              index === 0 ? (
                "Current version"
              ) : (
                <Button
                  aria-label={`Go back to ${revision.name}`}
                  className="-ml-2"
                  disabled={disabled}
                  onClick={() => onRestore(revision.id)}
                  size="xs"
                  type="button"
                  variant="ghost"
                >
                  Go back to this
                </Button>
              )
            }
            title={revision.name}
          />
        ))}
      </Timeline>
    </EditorCard>
  );
}
