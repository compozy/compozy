import { FormSection } from "@compozy/ui";

/**
 * OAuth metadata, client registrations, and credentials are daemon-owned.
 * The editor only stores a remote endpoint; authorization discovery happens
 * after save through the scoped authorization flow.
 */
export function MCPEditorOAuthSection() {
  return (
    <FormSection
      data-testid="settings-mcp-editor-oauth"
      help="CompozyOS discovers OAuth requirements when you authorize this server. Provider credentials and authorization codes are not stored in this form."
      title="Authorization"
    >
      {null}
    </FormSection>
  );
}
