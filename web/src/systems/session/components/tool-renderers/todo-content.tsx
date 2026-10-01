import { Check } from "lucide-react";

import { cn } from "@/lib/utils";
import { Eyebrow, StateGlyph } from "@compozy/ui";

import type { UIMessage } from "../../types";
import { GenericContent } from "./generic-content";

type TodoState = "done" | "active" | "pending";

interface TodoItem {
  content: string;
  key: string;
  state: TodoState;
}

function todoState(status: unknown): TodoState {
  if (status === "completed") return "done";
  if (status === "in_progress") return "active";
  return "pending";
}

/** Parse the TodoWrite `todos` arg into renderable items; null when unrecognizable. */
function parseTodos(value: unknown): TodoItem[] | null {
  if (!Array.isArray(value) || value.length === 0) return null;
  const items: TodoItem[] = [];
  const fallbackOccurrences = new Map<string, number>();
  for (const candidate of value) {
    if (typeof candidate !== "object" || candidate === null) return null;
    const record = candidate as Record<string, unknown>;
    const content =
      typeof record.content === "string" && record.content.trim().length > 0
        ? record.content.trim()
        : typeof record.subject === "string"
          ? record.subject.trim()
          : "";
    if (!content) return null;
    const explicitKey = [record.id, record.todo_id, record.task_id].find(
      value => typeof value === "string" && value.trim().length > 0
    );
    const occurrence = (fallbackOccurrences.get(content) ?? 0) + 1;
    fallbackOccurrences.set(content, occurrence);
    items.push({
      content,
      key: typeof explicitKey === "string" ? explicitKey.trim() : `${content}\u0000${occurrence}`,
      state: todoState(record.status),
    });
  }
  return items;
}

// Done is the transcript's grey check (completion is the resting state, ADR-009);
// active and pending take the shared running / queued glyphs.
function TodoGlyph({ state }: { state: TodoState }) {
  if (state === "done") {
    return <Check aria-hidden="true" className="size-3 text-subtle" strokeWidth={2} />;
  }
  return <StateGlyph size="sm" state={state === "active" ? "running" : "queued"} />;
}

/**
 * TodoWrite plan renderer — task lines, never JSON: a "Plan · X of N" caption
 * over rows whose state reads as done (grey check + line-through), active
 * (running ring) or pending (queued dashed ring). Unrecognizable payloads fall
 * back to the generic JSON detail.
 */
export function TodoContent({ message }: { message: UIMessage }) {
  const todos = parseTodos(message.toolInput?.todos);
  if (!todos) {
    return <GenericContent message={message} />;
  }
  const doneCount = todos.filter(item => item.state === "done").length;

  return (
    <div className="flex min-w-0 flex-col gap-px" data-testid="todo-content">
      <Eyebrow className="mb-0.5 text-faint">
        Plan · {doneCount} of {todos.length}
      </Eyebrow>
      {todos.map(item => (
        <div
          key={item.key}
          data-state={item.state}
          className={cn(
            "flex min-h-transcript-row items-start gap-2 text-small-body leading-normal",
            item.state === "done" ? "text-subtle line-through decoration-faint" : null,
            item.state === "active" ? "text-fg" : null,
            item.state === "pending" ? "text-muted" : null
          )}
        >
          <span className="mt-px grid size-4 shrink-0 place-items-center">
            <TodoGlyph state={item.state} />
          </span>
          <span className="min-w-0">{item.content}</span>
        </div>
      ))}
    </div>
  );
}
