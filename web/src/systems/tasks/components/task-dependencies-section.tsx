import { ChevronRight } from "lucide-react";
import { useReducedMotionConfig } from "motion/react";
import { useState } from "react";

import { cn, Collapsible, CollapsibleContent, CollapsibleTrigger, Section } from "@agh/ui";

import type { TaskDetailView } from "../types";
import { TaskLinkedRow, type TaskLinkedRowState } from "./task-linked-row";

type DependencyReference = NonNullable<TaskDetailView["dependency_references"]>[number];

function dependencyRowState(dep: DependencyReference): TaskLinkedRowState {
  const status = dep.depends_on.status;
  if (status === "completed") return "done";
  if (status === "in_progress" || status === "needs_attention") return "active";
  return "todo";
}

function isResolved(dep: DependencyReference): boolean {
  return dependencyRowState(dep) === "done";
}

/**
 * Overview "Blocked by" section: open dependencies render as linked
 * rows; resolved ones collapse behind a quiet disclosure so a healthy task
 * reads as one line, not a table. Renders nothing without dependencies.
 *
 * @see docs/design/opendesign/tasks/TASK-DETAILS-REDESIGN-PLAN.md §4.3
 */
export function TaskDependenciesSection({
  dependencies,
}: {
  dependencies: readonly DependencyReference[];
}) {
  const [showResolved, setShowResolved] = useState(false);
  const reduceMotion = useReducedMotionConfig();
  if (dependencies.length === 0) return null;

  const open = dependencies.filter(dep => !isResolved(dep));
  const resolved = dependencies.filter(isResolved);

  return (
    <Section
      count={open.length > 0 ? open.length : undefined}
      data-testid="tasks-detail-dependencies"
      label="Blocked by"
    >
      <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft">
        {open.map(dep => (
          <TaskLinkedRow
            key={dep.depends_on.id}
            owner={dep.depends_on.owner}
            state={dependencyRowState(dep)}
            taskId={dep.depends_on.id}
            testId={`tasks-detail-dependency-${dep.depends_on.id}`}
            title={dep.depends_on.title ?? dep.depends_on.identifier ?? dep.depends_on.id}
          />
        ))}
        {resolved.length > 0 ? (
          <Collapsible onOpenChange={setShowResolved} open={showResolved}>
            <CollapsibleTrigger
              className={cn(
                "flex w-full items-center gap-2 px-4 py-2.5 text-left text-small-body text-muted",
                "border-t border-line-soft transition-colors duration-fast first:border-t-0",
                "hover:bg-row-hover hover:text-fg focus-visible:outline-none focus-visible:shadow-focus-ring"
              )}
              data-testid="tasks-detail-resolved-toggle"
            >
              <ChevronRight
                aria-hidden="true"
                className={cn(
                  "size-3 text-faint",
                  !reduceMotion && "transition-transform duration-fast",
                  showResolved && "rotate-90"
                )}
              />
              {resolved.length} resolved {resolved.length === 1 ? "dependency" : "dependencies"}
            </CollapsibleTrigger>
            <CollapsibleContent
              className="border-t border-line-soft"
              id="tasks-detail-resolved-dependencies"
            >
              {resolved.map(dep => (
                <TaskLinkedRow
                  key={dep.depends_on.id}
                  owner={dep.depends_on.owner}
                  state="done"
                  taskId={dep.depends_on.id}
                  testId={`tasks-detail-dependency-${dep.depends_on.id}`}
                  title={dep.depends_on.title ?? dep.depends_on.identifier ?? dep.depends_on.id}
                />
              ))}
            </CollapsibleContent>
          </Collapsible>
        ) : null}
      </div>
    </Section>
  );
}
