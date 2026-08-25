import { FolderPlus } from "lucide-react";
import { useState } from "react";

import { Button, Input, MonoId } from "@compozy/ui";

import type { SkillSourceEntryError } from "../lib/skill-source-draft";
import type { SkillSourceView } from "../lib/skill-sources-view";
import { SettingsSkillSourceRow } from "./settings-skill-source-row";

interface SettingsSkillCustomSourcesProps {
  /** Measured custom sources, straight from the daemon read model. */
  sources: readonly SkillSourceView[];
  /** Draft entries; an entry added but not yet saved has no measured row. */
  entries: readonly string[];
  disabled: boolean;
  onAdd: (path: string) => void;
  onRemove: (path: string) => void;
  validate: (entry: string) => SkillSourceEntryError | null;
}

const TEST_ID = "settings-page-skills-custom-sources";
const INPUT_ID = `${TEST_ID}-path`;
const ERROR_ID = `${TEST_ID}-error-message`;

/**
 * Folders the operator registered by hand. Measured entries render as ordinary
 * source rows; the add field validates against the same rules the daemon
 * applies, so a duplicate or a wrong-scope path is refused next to the input
 * rather than after a round trip.
 */
export function SettingsSkillCustomSources({
  sources,
  entries,
  disabled,
  onAdd,
  onRemove,
  validate,
}: SettingsSkillCustomSourcesProps) {
  const [draft, setDraft] = useState("");
  const [error, setError] = useState<SkillSourceEntryError | null>(null);
  const measuredPaths = new Set(sources.map(source => source.path ?? ""));
  const unmeasured = entries.filter(entry => !measuredPaths.has(entry));

  const commit = () => {
    const entry = draft.trim();
    if (entry === "") return;
    const failure = validate(entry);
    if (failure !== null) {
      setError(failure);
      return;
    }
    setError(null);
    setDraft("");
    onAdd(entry);
  };

  return (
    <div className="flex min-w-0 flex-col" data-testid={TEST_ID}>
      {sources.map(source => (
        <SettingsSkillSourceRow
          disabled={disabled}
          key={source.slug}
          onRemove={() => onRemove(source.path ?? source.slug)}
          source={source}
        />
      ))}
      {unmeasured.map(entry => (
        <PendingCustomRow
          disabled={disabled}
          entry={entry}
          key={entry}
          onRemove={() => onRemove(entry)}
        />
      ))}
      <div className="flex min-w-0 flex-wrap items-center gap-2 px-3 py-2.5">
        <label className="mr-auto text-sm text-fg" htmlFor={INPUT_ID}>
          Add your own folder
        </label>
        <Input
          aria-describedby={error !== null ? ERROR_ID : undefined}
          aria-invalid={error !== null}
          className="h-7 w-64 font-mono text-form-input"
          data-testid={`${TEST_ID}-input`}
          disabled={disabled}
          id={INPUT_ID}
          name="skill-source-path"
          onChange={event => {
            setDraft(event.target.value);
            if (error !== null) setError(null);
          }}
          onKeyDown={event => {
            if (event.key !== "Enter") return;
            event.preventDefault();
            commit();
          }}
          placeholder="~/path/to/skills"
          spellCheck={false}
          value={draft}
        />
        <Button
          data-testid={`${TEST_ID}-add`}
          disabled={disabled || draft.trim() === ""}
          onClick={commit}
          size="sm"
          type="button"
          variant="neutral"
        >
          Add
        </Button>
      </div>
      {error !== null ? (
        <p
          className="flex flex-wrap items-center gap-1.5 px-3 pb-2.5 text-form-hint text-danger"
          data-testid={`${TEST_ID}-error`}
          id={ERROR_ID}
        >
          {error.message}
          <MonoId className="text-faint" preserveCase value={error.code} />
        </p>
      ) : null}
    </div>
  );
}

/**
 * A folder added in this draft has no measured row yet. Showing it without
 * counts keeps the promise that a number always means something was scanned.
 */
function PendingCustomRow({
  entry,
  disabled,
  onRemove,
}: {
  entry: string;
  disabled: boolean;
  onRemove: () => void;
}) {
  return (
    <div
      className="flex min-h-11 min-w-0 items-center gap-2 border-b border-line px-3 last:border-b-0"
      data-testid={`${TEST_ID}-pending-${entry}`}
    >
      <FolderPlus aria-hidden="true" className="size-3.5 shrink-0 text-faint" />
      <span className="truncate font-mono text-xs text-muted">{entry}</span>
      <span aria-hidden="true" className="flex-1" />
      <span className="shrink-0 text-form-hint text-subtle">not scanned yet</span>
      <Button
        aria-label={`Remove ${entry}`}
        disabled={disabled}
        onClick={onRemove}
        size="sm"
        type="button"
        variant="ghost"
      >
        Remove
      </Button>
    </div>
  );
}
