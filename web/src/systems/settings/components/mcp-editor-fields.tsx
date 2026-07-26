import { Eyebrow, Input, NativeSelect, NativeSelectOption } from "@agh/ui";
import type { ReactNode } from "react";

import type { SettingsMCPServerTarget } from "../types";

import { mcpTargetLabel } from "./mcp-server-labels";

export function MCPFieldLabel({
  htmlFor,
  hint,
  children,
}: {
  htmlFor?: string;
  hint?: string;
  children: ReactNode;
}) {
  return (
    <label
      className="mb-1.5 flex items-center gap-2 text-form-label font-medium text-fg"
      htmlFor={htmlFor}
    >
      {children}
      {hint ? <Eyebrow className="ml-auto text-muted">{hint}</Eyebrow> : null}
    </label>
  );
}

export function MCPNameField({
  name,
  isCreate,
  error,
  onChange,
}: {
  name: string;
  isCreate: boolean;
  error?: string;
  onChange: (name: string) => void;
}) {
  return (
    <div>
      <MCPFieldLabel htmlFor="mcp-editor-name" hint={isCreate ? "required" : "locked"}>
        Name
      </MCPFieldLabel>
      <Input
        id="mcp-editor-name"
        className="font-mono disabled:opacity-60"
        value={name}
        placeholder="e.g. filesystem"
        disabled={!isCreate}
        aria-invalid={error ? true : undefined}
        onChange={event => onChange(event.target.value)}
        data-testid="settings-mcp-servers-editor-name-input"
      />
      {error ? <p className="mt-1.5 text-caption text-danger">{error}</p> : null}
    </div>
  );
}

export function MCPTargetField({
  target,
  availableTargets,
  onChange,
}: {
  target: SettingsMCPServerTarget;
  availableTargets: SettingsMCPServerTarget[];
  onChange: (target: SettingsMCPServerTarget) => void;
}) {
  return (
    <div>
      <MCPFieldLabel htmlFor="mcp-editor-target">Persistence target</MCPFieldLabel>
      <NativeSelect
        id="mcp-editor-target"
        className="font-mono"
        value={target}
        onChange={event => onChange(event.target.value as SettingsMCPServerTarget)}
        data-testid="settings-mcp-servers-editor-target-input"
      >
        {availableTargets.map(candidate => (
          <NativeSelectOption key={candidate} value={candidate}>
            {mcpTargetLabel(candidate)}
          </NativeSelectOption>
        ))}
      </NativeSelect>
    </div>
  );
}
