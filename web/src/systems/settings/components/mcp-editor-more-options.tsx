import { Disclosure } from "@compozy/ui";

import type { MCPDraft, MCPDraftErrors } from "../lib/mcp-editor-model";
import type {
  SettingsLayeredScope,
  SettingsMCPServerEntry,
  SettingsMCPServerTarget,
} from "../types";

import { MCPTargetField } from "./mcp-editor-fields";
import { MCPEditorOAuthSection } from "./mcp-editor-oauth-section";
import { MCPEditorProcessSection } from "./mcp-editor-stdio-section";
import { SettingsSourceBadge } from "./settings-source-badge";

export interface MCPEditorMoreOptionsProps {
  draft: MCPDraft;
  errors: MCPDraftErrors;
  entry: SettingsMCPServerEntry | null;
  scope: SettingsLayeredScope;
  target: SettingsMCPServerTarget;
  availableTargets: SettingsMCPServerTarget[];
  statusLabel?: string;
  defaultOpen: boolean;
  onChange: (updater: (draft: MCPDraft) => MCPDraft) => void;
  onTargetChange: (target: SettingsMCPServerTarget) => void;
}

/**
 * The framed "More options" fold: everything a server can set beyond its source and keys.
 * A local program gets its arguments, settings and key names; a remote server its OAuth choice.
 * Where the definition is saved and where it currently comes from close the fold.
 */
export function MCPEditorMoreOptions({
  draft,
  errors,
  entry,
  scope,
  target,
  availableTargets,
  statusLabel,
  defaultOpen,
  onChange,
  onTargetChange,
}: MCPEditorMoreOptionsProps) {
  const isRemote = draft.transport === "http";
  return (
    <Disclosure
      className="mt-6"
      contentProps={{ "data-testid": "settings-mcp-servers-editor-more-options" }}
      defaultOpen={defaultOpen}
      label="More options"
      // The fold replaced the Simple/Advanced toolbar; its trigger keeps that toolbar's id.
      triggerProps={{ "data-testid": "settings-mcp-servers-editor-mode-advanced" }}
      variant="framed"
    >
      <div className="flex flex-col gap-5 p-4">
        {isRemote ? (
          <MCPEditorOAuthSection
            draft={draft}
            hasSavedOAuth={Boolean(entry?.auth)}
            onChange={onChange}
          />
        ) : (
          <MCPEditorProcessSection draft={draft} errors={errors} onChange={onChange} />
        )}
        {availableTargets.length > 1 ? (
          <MCPTargetField
            availableTargets={availableTargets}
            onChange={onTargetChange}
            target={target}
          />
        ) : null}
        <dl className="grid grid-cols-[auto_1fr] items-center gap-x-4 gap-y-2 text-meta">
          <dt className="text-faint">Scope</dt>
          <dd
            className="truncate font-mono text-muted-foreground"
            data-testid="settings-mcp-servers-editor-scope-fact"
          >
            {scope}
          </dd>
          {statusLabel ? (
            <>
              <dt className="text-faint">Status</dt>
              <dd
                className="truncate font-mono text-muted-foreground"
                data-testid="settings-mcp-servers-editor-status"
              >
                {statusLabel}
              </dd>
            </>
          ) : null}
          {entry ? (
            <>
              <dt className="text-faint">Comes from</dt>
              <dd className="min-w-0">
                <SettingsSourceBadge
                  data-testid="settings-mcp-servers-editor-source"
                  shadowed={entry.source_metadata.shadowed_sources ?? []}
                  source={entry.source_metadata.effective_source}
                />
              </dd>
            </>
          ) : null}
        </dl>
      </div>
    </Disclosure>
  );
}
