import { Switch } from "@compozy/ui";

import type { MCPDraft } from "../lib/mcp-editor-model";
import { SettingsFieldRow } from "./settings-field-row";

/** More options for a remote server: whether it signs in with OAuth. */
export function MCPEditorOAuthSection({
  draft,
  hasSavedOAuth,
  onChange,
}: {
  draft: MCPDraft;
  hasSavedOAuth: boolean;
  onChange: (updater: (draft: MCPDraft) => MCPDraft) => void;
}) {
  return (
    <div className="flex flex-col gap-1.5" data-testid="settings-mcp-editor-oauth">
      <SettingsFieldRow
        label="Use OAuth"
        control={
          <Switch
            aria-label="Use OAuth"
            checked={draft.oauth.enabled}
            disabled={Boolean(draft.headersEndpoint) && !hasSavedOAuth}
            onCheckedChange={enabled =>
              onChange(current => ({
                ...current,
                oauth: { ...current.oauth, enabled },
              }))
            }
          />
        }
      />
      <p className="text-form-hint text-muted-foreground">
        {draft.headersEndpoint
          ? "This server has configured headers. To change its URL or headers, edit its settings file."
          : "CompozyOS finds out how it signs in when you sign in. Credentials are never stored in this form."}
      </p>
    </div>
  );
}
