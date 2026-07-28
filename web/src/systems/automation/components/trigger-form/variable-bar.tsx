import { Plus } from "lucide-react";

interface VariableBarProps {
  variables: string[];
  onInsert: (variable: string) => void;
}

/** Click-to-insert variable chips derived from the selected event's fields. */
export function VariableBar({ variables, onInsert }: VariableBarProps) {
  return (
    <div className="mb-2 flex flex-wrap gap-1.5">
      {variables.map(variable => (
        <button
          aria-label={`Insert ${variable}`}
          className="inline-flex items-center gap-1 rounded-xs border border-line-soft bg-badge-fill px-1.5 py-1 font-mono text-mono-id font-medium text-fg outline-none transition-colors hover:border-accent-dim hover:bg-accent-tint hover:text-accent-strong focus-visible:shadow-focus-ring"
          key={variable}
          onClick={() => onInsert(variable)}
          type="button"
        >
          <Plus aria-hidden="true" className="size-2.5 opacity-60" />
          {variable}
        </button>
      ))}
    </div>
  );
}
