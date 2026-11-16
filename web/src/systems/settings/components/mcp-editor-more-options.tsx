import { Disclosure, MetadataList, MetadataListRow } from "@compozy/ui";

import type { MCPDraft, MCPDraftErrors } from "../lib/mcp-editor-model";
import type {
  SettingsLayeredScope,
  SettingsMCPServerEntry,
  SettingsMCPServerTarget,
} from "../types";

import { MCPTargetField } from "./mcp-editor-fields";
import { MCPEditorOAuthSection } from "./mcp-editor-oauth-section";
import { MCPEditorProcessSection, type MCPSecretRowKeys } from "./mcp-editor-stdio-section";
import { mcpScopeWords } from "./mcp-server-labels";
import { SettingsSourceBadge } from "./settings-source-badge";

export interface MCPEditorMoreOptionsProps {
  draft: MCPDraft;
  errors: MCPDraftErrors;
  entry: SettingsMCPServerEntry | null;
  scope: SettingsLayeredScope;
  /** The project an "Only <project>" scope names. */
  projectName?: string;
  target: SettingsMCPServerTarget;
  availableTargets: SettingsMCPServerTarget[];
  statusLabel?: string;
  defaultOpen: boolean;
  secretRowKeys: MCPSecretRowKeys;
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
  projectName,
  target,
  availableTargets,
  statusLabel,
  defaultOpen,
  secretRowKeys,
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
          <MCPEditorProcessSection
            draft={draft}
            errors={errors}
            onChange={onChange}
            secretRowKeys={secretRowKeys}
          />
        )}
        {availableTargets.length > 1 ? (
          <MCPTargetField
            availableTargets={availableTargets}
            onChange={onTargetChange}
            target={target}
          />
        ) : null}
        <MetadataList>
          <MetadataListRow
            label="Used in"
            valueProps={{ "data-testid": "settings-mcp-servers-editor-scope-fact" }}
          >
            {mcpScopeWords(scope, { projectName, profile: entry?.profile })}
          </MetadataListRow>
          {statusLabel ? (
            <MetadataListRow
              label="Status"
              valueProps={{ "data-testid": "settings-mcp-servers-editor-status" }}
            >
              {statusLabel}
            </MetadataListRow>
          ) : null}
          {entry ? (
            <MetadataListRow label="Comes from">
              <SettingsSourceBadge
                data-testid="settings-mcp-servers-editor-source"
                shadowed={entry.source_metadata.shadowed_sources ?? []}
                source={entry.source_metadata.effective_source}
              />
            </MetadataListRow>
          ) : null}
        </MetadataList>
      </div>
    </Disclosure>
  );
}
