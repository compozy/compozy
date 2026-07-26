import { Section, StackedProgress } from "@agh/ui";

import type { TaskChildSummary } from "../types";
import { TaskLinkedRow, type TaskLinkedRowState } from "./task-linked-row";

function childRowState(child: TaskChildSummary): TaskLinkedRowState {
  if (child.status === "completed") return "done";
  if (child.status === "in_progress" || child.status === "needs_attention") return "active";
  return "todo";
}

/**
 * Overview "Subtasks" section: thin done/active progress strip + one
 * linked row per child. Renders nothing when the task has no children — the
 * quiet page rule beats an empty frame.
 *
 * @see docs/design/opendesign/tasks/TASK-DETAILS-REDESIGN-PLAN.md §4.3
 */
export function TaskSubtasksSection({ items }: { items: readonly TaskChildSummary[] }) {
  if (items.length === 0) return null;

  const done = items.filter(child => child.status === "completed").length;
  const active = items.filter(
    child => child.status === "in_progress" || child.status === "needs_attention"
  ).length;
  const pending = items.length - done - active;

  return (
    <Section
      count={items.length}
      data-testid="tasks-detail-subtasks"
      label="Subtasks"
      right={
        <span className="text-eyebrow font-medium text-muted">
          {done} of {items.length} done
        </span>
      }
    >
      <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft">
        <div className="px-4 pt-3.5 pb-0.5">
          <StackedProgress
            ariaLabel={`${done} done, ${active} active, ${pending} not started; ${items.length} subtasks total`}
            className="h-[3px]"
            segments={[
              { value: done, tone: "success", label: "done" },
              { value: active, tone: "neutral", label: "active" },
            ]}
            total={items.length}
          />
        </div>
        {items.map(child => (
          <TaskLinkedRow
            key={child.id}
            owner={child.owner}
            state={childRowState(child)}
            taskId={child.id}
            testId={`tasks-detail-subtask-${child.id}`}
            lastActivityAt={child.last_activity_at ?? null}
            title={child.title}
          />
        ))}
      </div>
    </Section>
  );
}
