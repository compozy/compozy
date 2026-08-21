import { Globe, TerminalSquare } from "lucide-react";

import { FormSection, ImmutableIdentity, Input, RadioCard } from "@compozy/ui";

import type { MCPDraft, MCPDraftErrors } from "../lib/mcp-editor-model";
import { withTransport } from "../lib/mcp-editor-model";
import type { SettingsLayeredScope, SettingsMCPServerTarget } from "../types";

import { MCPFieldLabel, MCPNameField, MCPTargetField } from "./mcp-editor-fields";

/**
 * Local vs remote is the consequence-bearing choice. HTTP is the sole remote
 * transport, so a wire-level picker would imply unsupported runtime behavior.
 */

export interface MCPEditorConnectionSectionProps {
  draft: MCPDraft;
  errors: MCPDraftErrors;
  isCreate: boolean;
  scope: SettingsLayeredScope;
  target: SettingsMCPServerTarget;
  availableTargets: SettingsMCPServerTarget[];
  onChange: (updater: (draft: MCPDraft) => MCPDraft) => void;
  onTargetChange: (target: SettingsMCPServerTarget) => void;
}

/**
 * Simple tier: how the server runs and how it is reached.
 *
 * Switching transport replaces the launch configuration outright — `toRequest`
 * omits the other branch's fields entirely — so the choice sits above the
 * fields it governs.
 */
export function MCPEditorConnectionSection({
  draft,
  errors,
  isCreate,
  scope,
  target,
  availableTargets,
  onChange,
  onTargetChange,
}: MCPEditorConnectionSectionProps) {
  const isRemote = draft.transport !== "stdio";

  return (
    <FormSection
      data-testid="settings-mcp-editor-connection"
      description="Switching transport replaces the launch configuration."
      title={isCreate ? "How does it run?" : "Connection"}
    >
      {isCreate ? null : (
        <ImmutableIdentity
          hint="The tool prefix agents see cannot change — create a new server to rename it."
          rows={[
            { label: "Server", mono: true, value: draft.name },
            { label: "Scope", mono: true, value: scope },
          ]}
        />
      )}

      <div
        aria-label="Transport"
        className="grid gap-2 sm:grid-cols-2"
        data-testid="settings-mcp-servers-editor-transport"
        role="radiogroup"
      >
        <RadioCard
          data-testid="settings-mcp-servers-editor-transport-stdio"
          description="CompozyOS spawns a command and talks over stdio."
          icon={TerminalSquare}
          onSelect={() => onChange(current => withTransport(current, "stdio"))}
          selected={!isRemote}
          title="Local process"
          titleClassName="min-w-0 flex-1 truncate"
        />
        <RadioCard
          data-testid="settings-mcp-servers-editor-transport-remote"
          description="Connects to a hosted MCP server, optionally with OAuth."
          icon={Globe}
          onSelect={() => onChange(current => withTransport(current, "http"))}
          selected={isRemote}
          title="Remote endpoint"
          titleClassName="min-w-0 flex-1 truncate"
        />
      </div>

      {isCreate ? (
        <MCPNameField
          error={errors.name}
          isCreate={isCreate}
          name={draft.name}
          onChange={name => onChange(current => ({ ...current, name }))}
        />
      ) : null}

      {isRemote ? (
        <>
          <div>
            <MCPFieldLabel hint="required" htmlFor="mcp-editor-url">
              URL
            </MCPFieldLabel>
            <Input
              aria-invalid={errors.url ? true : undefined}
              className="font-mono"
              data-testid="settings-mcp-servers-editor-url-input"
              id="mcp-editor-url"
              onChange={event => onChange(current => ({ ...current, url: event.target.value }))}
              placeholder="https://mcp.linear.app/mcp"
              value={draft.url}
            />
            {errors.url ? <p className="mt-1.5 text-caption text-danger">{errors.url}</p> : null}
          </div>
          {errors.remoteFields ? (
            <p className="text-caption text-danger" data-testid="settings-mcp-editor-remote-error">
              {errors.remoteFields}
            </p>
          ) : null}
        </>
      ) : (
        <div>
          <MCPFieldLabel hint="required" htmlFor="mcp-editor-command">
            Command
          </MCPFieldLabel>
          <Input
            aria-invalid={errors.command ? true : undefined}
            className="font-mono"
            data-testid="settings-mcp-servers-editor-command-input"
            id="mcp-editor-command"
            onChange={event => onChange(current => ({ ...current, command: event.target.value }))}
            placeholder="npx"
            value={draft.command}
          />
          {errors.command ? (
            <p className="mt-1.5 text-caption text-danger">{errors.command}</p>
          ) : null}
        </div>
      )}

      <MCPTargetField
        availableTargets={availableTargets}
        onChange={onTargetChange}
        target={target}
      />
    </FormSection>
  );
}
