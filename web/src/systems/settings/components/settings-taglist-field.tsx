import { X } from "lucide-react";
import { useState } from "react";

import { Button, Input, Pill } from "@compozy/ui";

export interface SettingsTaglistFieldProps {
  /** Current entries, rendered as removable chips. */
  value: readonly string[];
  onChange: (next: string[]) => void;
  /** Accessible label for the add input, e.g. "Allowed MCP installs". */
  label: string;
  placeholder?: string;
  emptyLabel?: string;
  disabled?: boolean;
  "data-testid"?: string;
}

/**
 * Chip-list editor for allow-lists (prototype `.taglist`): entries render as
 * removable chips; Enter (or Add) commits the trimmed input.
 */
export function SettingsTaglistField({
  value,
  onChange,
  label,
  placeholder = "Add entry…",
  emptyLabel = "Nothing allowed yet",
  disabled = false,
  "data-testid": testId,
}: SettingsTaglistFieldProps) {
  const [draft, setDraft] = useState("");

  const commitDraft = () => {
    const entry = draft.trim();
    if (!entry) return;
    setDraft("");
    if (value.includes(entry)) return;
    onChange([...value, entry]);
  };

  const removeEntry = (entry: string) => {
    onChange(value.filter(candidate => candidate !== entry));
  };

  return (
    <div className="flex w-full flex-col gap-2" data-testid={testId ?? "settings-taglist"}>
      <div className="flex flex-wrap items-center gap-1.5">
        {value.length === 0 ? (
          <span className="text-form-hint text-subtle" data-slot="settings-taglist-empty">
            {emptyLabel}
          </span>
        ) : (
          value.map(entry => (
            <Pill key={entry} mono tone="neutral">
              {entry}
              <Button
                aria-label={`Remove ${entry}`}
                disabled={disabled}
                onClick={() => removeEntry(entry)}
                size="icon-xs"
                type="button"
                variant="ghost"
              >
                <X aria-hidden="true" className="size-2.5" />
              </Button>
            </Pill>
          ))
        )}
      </div>
      <div className="flex items-center gap-1.5">
        <Input
          aria-label={label}
          className="h-7 w-64 font-mono text-form-input"
          disabled={disabled}
          onChange={event => setDraft(event.target.value)}
          onKeyDown={event => {
            if (event.key !== "Enter") return;
            event.preventDefault();
            commitDraft();
          }}
          placeholder={placeholder}
          value={draft}
        />
        <Button
          disabled={disabled || draft.trim() === ""}
          onClick={commitDraft}
          size="sm"
          type="button"
          variant="neutral"
        >
          Add
        </Button>
      </div>
    </div>
  );
}
