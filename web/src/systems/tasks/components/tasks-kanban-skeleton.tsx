import { SkeletonRegion } from "@compozy/ui";

import { groupTasksForKanban } from "../lib/task-grouping";
import { TaskKanbanCardSkeleton } from "./task-kanban-card-skeleton";
import { TaskKanbanColumn } from "./task-kanban-column";
import { KANBAN_BOARD_CLASS, KANBAN_LOADING_CARDS, kanbanBoardStyle } from "./tasks-kanban-grid";

/**
 * The board before any card has arrived: every column in its place with its real heading, and a
 * few placeholder cards in each. Nothing in it takes focus; its one status region is the board's.
 */
export function TasksKanbanBoardSkeleton() {
  const columns = groupTasksForKanban([]);
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <SkeletonRegion
        className="flex min-h-0 flex-1 flex-col"
        data-testid="tasks-kanban-loading"
        label="Loading kanban board"
      >
        <ul
          className={KANBAN_BOARD_CLASS}
          data-testid="tasks-kanban-board"
          style={kanbanBoardStyle(columns.length)}
        >
          {columns.map((group, columnIndex) => (
            <TaskKanbanColumn column={group.column} count={0} key={group.column.id}>
              {(KANBAN_LOADING_CARDS[columnIndex % KANBAN_LOADING_CARDS.length] ?? []).map(card => (
                <TaskKanbanCardSkeleton
                  key={`${group.column.id}-skeleton-${card.id}`}
                  titleLines={card.titleLines}
                />
              ))}
            </TaskKanbanColumn>
          ))}
        </ul>
      </SkeletonRegion>
    </div>
  );
}
