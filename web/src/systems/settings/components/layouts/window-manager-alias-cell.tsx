import { Input } from "@compozy/ui";

import { ALIAS_RULE_HINT, type AliasCellState } from "../../hooks/use-window-manager-alias-editor";

export interface WindowManagerAliasCellProps {
  commandId: string;
  commandTitle: string;
  state: AliasCellState;
  onChange: (commandId: string, value: string) => void;
  onCommit: (commandId: string) => void;
  onCancel: (commandId: string) => void;
}

/**
 * The alias for one command, edited in place.
 *
 * The field carries the ceiling itself (`maxLength`), so the only rule the
 * operator can break is whitespace — which is why the rule text appears on that
 * failure instead of sitting under the column as permanent instruction.
 */
export function WindowManagerAliasCell({
  commandId,
  commandTitle,
  state,
  onChange,
  onCommit,
  onCancel,
}: WindowManagerAliasCellProps) {
  const hintId = `alias-hint-${commandId}`;
  const invalid = state.problem !== null;
  return (
    <div className="flex flex-col gap-1">
      <Input
        aria-describedby={invalid ? hintId : undefined}
        aria-invalid={invalid || undefined}
        aria-label={`Alias for ${commandTitle}`}
        autoComplete="off"
        className="h-7 max-w-36 px-2 text-form-input"
        data-testid={`shortcut-alias-${commandId}`}
        disabled={state.saving}
        maxLength={32}
        placeholder="add alias"
        spellCheck={false}
        value={state.value}
        onBlur={() => onCommit(commandId)}
        onChange={event => onChange(commandId, event.target.value)}
        onKeyDown={event => {
          if (event.key === "Enter") {
            event.preventDefault();
            onCommit(commandId);
            return;
          }
          if (event.key === "Escape") {
            event.preventDefault();
            onCancel(commandId);
          }
        }}
      />
      {invalid ? (
        <p className="text-form-hint text-danger" id={hintId} role="status">
          {state.problem === "grammar" ? ALIAS_RULE_HINT : state.problem}
        </p>
      ) : null}
    </div>
  );
}
