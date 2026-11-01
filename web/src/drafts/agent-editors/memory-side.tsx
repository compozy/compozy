/** S9's right column: how full memory is, what got put away, and history. */
import { Archive, History } from "@compozy/ui/icons";

import { Button, Disclosure, Timeline, TimelineEvent } from "@compozy/ui";

import { EditorCard } from "../stays-on-agents-shared/rich-controls";
import { PUT_AWAY, REVISIONS } from "./memory-fixtures";

export type { MemoryFullness as Fullness } from "@/systems/agent/lib/agent-memory-notes";
export { FullnessCard } from "@/systems/agent/components/agent-editors/memory-side";

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
        <ul className="flex flex-col gap-1.5 text-small-body text-muted-foreground">
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
        <History aria-hidden="true" className="size-3.5 text-muted-foreground" />
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
