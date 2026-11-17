import {
  Eyebrow,
  Field,
  FieldDescription,
  FieldError,
  FormSection,
  Input,
  NativeSelect,
  NativeSelectOption,
  RadioCardGroup,
} from "@compozy/ui";
import { FolderOpen, Globe } from "@compozy/ui/icons";
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
      {hint ? <Eyebrow className="ml-auto text-muted-foreground">{hint}</Eyebrow> : null}
    </label>
  );
}

/** Where a new server applies: every project (user scope) or the open project only. */
export interface MCPScopeChoice {
  value: "user" | "workspace";
  projectName: string;
  /** An existing server keeps the scope its definition lives in. */
  disabled?: boolean;
  onChange?: (scope: "user" | "workspace") => void;
}

/**
 * "Name and where to use it": the section title names the field, so the input carries only an
 * accessible name. The name is the tool prefix agents see, so it locks once the server exists.
 */
export function MCPNameSection({
  name,
  isCreate,
  error,
  scopeChoice,
  onBlur,
  onChange,
}: {
  name: string;
  isCreate: boolean;
  error?: string;
  scopeChoice?: MCPScopeChoice;
  onBlur?: () => void;
  onChange: (name: string) => void;
}) {
  return (
    <FormSection
      data-testid="settings-mcp-editor-identity"
      icon={FolderOpen}
      title="Name and where to use it"
    >
      <Field data-invalid={error ? "true" : undefined}>
        <Input
          aria-describedby="mcp-editor-name-description"
          aria-invalid={error ? true : undefined}
          aria-label="Name"
          className="font-mono"
          data-testid="settings-mcp-servers-editor-name-input"
          disabled={!isCreate}
          id="mcp-editor-name"
          onBlur={onBlur}
          onChange={event => onChange(event.target.value)}
          placeholder="Name, like linear"
          value={name}
        />
        {error ? <FieldError>{error}</FieldError> : null}
        <FieldDescription id="mcp-editor-name-description">
          {isCreate
            ? "Agents see this name next to its tools."
            : "Agents know the server by this name, so it can't change."}
        </FieldDescription>
      </Field>
      {scopeChoice ? (
        <RadioCardGroup
          aria-label="Use it in"
          className="grid-cols-2"
          data-testid="settings-mcp-servers-editor-scope"
          disabled={scopeChoice.disabled}
          onValueChange={value => scopeChoice.onChange?.(value)}
          options={[
            {
              value: "user",
              icon: Globe,
              title: "All projects",
              "data-testid": "settings-mcp-servers-editor-scope-user",
            },
            {
              value: "workspace",
              icon: FolderOpen,
              title: `Only ${scopeChoice.projectName}`,
              titleClassName: "min-w-0 flex-1 truncate",
              "data-testid": "settings-mcp-servers-editor-scope-workspace",
            },
          ]}
          value={scopeChoice.value}
        />
      ) : null}
    </FormSection>
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
      <MCPFieldLabel htmlFor="mcp-editor-target">Saved in</MCPFieldLabel>
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
