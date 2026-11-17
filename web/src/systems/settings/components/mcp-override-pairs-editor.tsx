import { Plus, Trash2 } from "@compozy/ui/icons";

import { Button, Empty, Input } from "@compozy/ui";

import { useLocalRowKeys } from "@/hooks/use-local-row-keys";

import type { MCPOverridePair } from "../lib/mcp-override-model";

import { MCPFieldLabel } from "./mcp-editor-fields";

/** Name/value rows an extension server override adds: env for a program, headers for a URL. */
export function MCPOverridePairsEditor({
  field,
  label,
  keyPlaceholder,
  addLabel,
  pairs,
  errors,
  disabled,
  onChange,
}: {
  field: "env" | "headers";
  label: string;
  keyPlaceholder: string;
  addLabel: string;
  pairs: MCPOverridePair[];
  errors?: Record<number, string>;
  disabled: boolean;
  onChange: (next: MCPOverridePair[]) => void;
}) {
  const rowKeys = useLocalRowKeys(pairs, `mcp-override-${field}`);
  const testPrefix = `settings-mcp-override-editor-${field}`;
  const noun = field === "env" ? "setting" : "header";
  return (
    <div>
      <MCPFieldLabel hint="name / value">{label}</MCPFieldLabel>
      <div className="flex flex-col gap-1.5" data-testid={`${testPrefix}-list`}>
        {pairs.length === 0 ? (
          <Empty
            data-testid={`${testPrefix}-empty`}
            description="The extension's own values apply."
            size="compact"
            title="Nothing changed yet"
          />
        ) : null}
        {pairs.map((pair, index) => {
          const rowError = errors?.[index];
          const errorId = rowError ? `${rowKeys.keys[index]}-error` : undefined;
          return (
            <div key={rowKeys.keys[index]}>
              <div className="flex items-center gap-2">
                <Input
                  aria-describedby={errorId}
                  aria-invalid={rowError ? true : undefined}
                  aria-label={`${label} ${noun} name ${index + 1}`}
                  className="w-40 font-mono"
                  data-testid={`${testPrefix}-key-${index}`}
                  disabled={disabled}
                  onChange={event => {
                    const next = [...pairs];
                    next[index] = { ...pair, key: event.target.value };
                    onChange(next);
                  }}
                  placeholder={keyPlaceholder}
                  value={pair.key}
                />
                <Input
                  aria-describedby={errorId}
                  aria-invalid={rowError ? true : undefined}
                  aria-label={`${label} ${noun} value ${index + 1}`}
                  className="flex-1 font-mono"
                  data-testid={`${testPrefix}-value-${index}`}
                  disabled={disabled}
                  onChange={event => {
                    const next = [...pairs];
                    next[index] = { ...pair, value: event.target.value };
                    onChange(next);
                  }}
                  placeholder="value"
                  value={pair.value}
                />
                <Button
                  aria-label={`Remove ${noun} ${index + 1}`}
                  data-testid={`${testPrefix}-remove-${index}`}
                  disabled={disabled}
                  onClick={() => {
                    rowKeys.remove(index);
                    onChange(pairs.filter((_, i) => i !== index));
                  }}
                  size="icon-sm"
                  type="button"
                  variant="ghost"
                >
                  <Trash2 aria-hidden="true" />
                </Button>
              </div>
              {rowError ? (
                <p className="mt-1 text-caption text-destructive" id={errorId} role="alert">
                  {rowError}
                </p>
              ) : null}
            </div>
          );
        })}
        <Button
          className="self-start"
          data-testid={`${testPrefix}-add`}
          disabled={disabled}
          onClick={() => {
            rowKeys.append();
            onChange([...pairs, { key: "", value: "" }]);
          }}
          size="sm"
          type="button"
          variant="ghost"
        >
          <Plus aria-hidden="true" />
          {addLabel}
        </Button>
      </div>
    </div>
  );
}
