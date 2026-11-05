import { FormSection, Switch } from "@compozy/ui";

import type { MCPDraft } from "../lib/mcp-editor-model";
import { SettingsFieldRow } from "./settings-field-row";

export function MCPEditorOAuthSection({
  draft,
  onChange,
}: {
  draft: MCPDraft;
  onChange: (updater: (draft: MCPDraft) => MCPDraft) => void;
}) {
  return (
    <FormSection
      data-testid="settings-mcp-editor-oauth"
      help={
        draft.headersEndpoint
          ? "This server has configured headers. To change its URL or headers, edit its settings file."
          : "CompozyOS discovers sign-in requirements when you sign in. Credentials are not stored in this form."
      }
      title="Authorization"
    >
      <SettingsFieldRow
        label="Use OAuth"
        control={
          <Switch
            aria-label="Use OAuth"
            checked={draft.oauth.enabled}
            disabled={Boolean(draft.headersEndpoint) && !draft.oauth.enabled}
            onCheckedChange={enabled =>
              onChange(current => ({
                ...current,
                oauth: { ...current.oauth, enabled },
              }))
            }
          />
        }
      />
    </FormSection>
  );
}
