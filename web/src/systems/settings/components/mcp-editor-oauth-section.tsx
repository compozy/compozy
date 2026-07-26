import { FormSection, Input, RadioCard } from "@agh/ui";

import type { MCPDraftErrors, MCPOAuthDiscovery, MCPOAuthDraft } from "../lib/mcp-editor-model";

import { MCPFieldLabel } from "./mcp-editor-fields";
import { MCPSecretBindingControl, type MCPVaultInventory } from "./mcp-secret-binding";

const DISCOVERY_OPTIONS: { value: MCPOAuthDiscovery; title: string; description: string }[] = [
  { value: "issuer", title: "Issuer URL", description: "Discover from issuer metadata" },
  { value: "metadata", title: "Metadata URL", description: "Use a metadata document" },
  { value: "endpoints", title: "Endpoint pair", description: "Authorization + token URLs" },
];

export interface MCPEditorOAuthSectionProps {
  oauth: MCPOAuthDraft;
  vaultInventory: MCPVaultInventory;
  errors: MCPDraftErrors;
  onChange: (oauth: MCPOAuthDraft) => void;
}

export function MCPEditorOAuthSection({
  oauth,
  vaultInventory,
  errors,
  onChange,
}: MCPEditorOAuthSectionProps) {
  return (
    <FormSection
      title="Authorization"
      rightLabel="optional"
      data-testid="settings-mcp-editor-oauth"
    >
      <div className="grid grid-cols-2 gap-1.5">
        <RadioCard
          selected={!oauth.enabled}
          title="None"
          description="No provider authorization"
          onSelect={() => onChange({ ...oauth, enabled: false })}
          data-testid="settings-mcp-editor-oauth-none"
        />
        <RadioCard
          selected={oauth.enabled}
          title="OAuth 2.1 + PKCE"
          description="Browser or manual completion"
          onSelect={() => onChange({ ...oauth, enabled: true })}
          data-testid="settings-mcp-editor-oauth-enabled"
        />
      </div>

      {oauth.enabled ? (
        <div className="mt-3 flex flex-col gap-3" data-testid="settings-mcp-editor-oauth-fields">
          <div>
            <MCPFieldLabel htmlFor="mcp-oauth-client-id" hint="required">
              Client ID
            </MCPFieldLabel>
            <Input
              id="mcp-oauth-client-id"
              className="font-mono"
              value={oauth.clientId}
              placeholder="agh-linear-public"
              aria-invalid={errors.clientId ? true : undefined}
              onChange={event => onChange({ ...oauth, clientId: event.target.value })}
              data-testid="settings-mcp-editor-oauth-client-id"
            />
            {errors.clientId ? (
              <p className="mt-1.5 text-caption text-danger">{errors.clientId}</p>
            ) : null}
          </div>

          <div>
            <MCPFieldLabel hint="choose one">Metadata strategy</MCPFieldLabel>
            <div className="grid grid-cols-3 gap-1.5">
              {DISCOVERY_OPTIONS.map(option => (
                <RadioCard
                  key={option.value}
                  selected={oauth.discovery === option.value}
                  title={option.title}
                  description={option.description}
                  onSelect={() => onChange({ ...oauth, discovery: option.value })}
                  data-testid={`settings-mcp-editor-oauth-discovery-${option.value}`}
                />
              ))}
            </div>
          </div>

          <MetadataFields oauth={oauth} error={errors.metadata} onChange={onChange} />

          <div>
            <MCPFieldLabel hint="optional">Client secret</MCPFieldLabel>
            <MCPSecretBindingControl
              binding={oauth.clientSecret}
              vaultInventory={vaultInventory}
              idPrefix="mcp-oauth-secret"
              valueLabel="New client secret"
              onChange={clientSecret => onChange({ ...oauth, clientSecret })}
            />
            {errors.clientSecret ? (
              <p className="mt-1.5 text-caption text-danger">{errors.clientSecret}</p>
            ) : null}
          </div>

          <div>
            <MCPFieldLabel htmlFor="mcp-oauth-scopes">Scopes</MCPFieldLabel>
            <Input
              id="mcp-oauth-scopes"
              className="font-mono"
              value={oauth.scopes}
              placeholder="read write"
              onChange={event => onChange({ ...oauth, scopes: event.target.value })}
              data-testid="settings-mcp-editor-oauth-scopes"
            />
          </div>
        </div>
      ) : (
        <p
          className="mt-3 rounded-md bg-info-tint p-2.5 text-caption text-info"
          data-testid="settings-mcp-editor-oauth-none-note"
        >
          No authorization fields are persisted. Select OAuth 2.1 + PKCE to configure discovery,
          client credentials, and scopes.
        </p>
      )}
    </FormSection>
  );
}

function MetadataFields({
  oauth,
  error,
  onChange,
}: {
  oauth: MCPOAuthDraft;
  error?: string;
  onChange: (oauth: MCPOAuthDraft) => void;
}) {
  return (
    <div data-testid="settings-mcp-editor-oauth-metadata">
      {oauth.discovery === "issuer" ? (
        <>
          <MCPFieldLabel htmlFor="mcp-oauth-issuer" hint="required">
            Issuer URL
          </MCPFieldLabel>
          <Input
            id="mcp-oauth-issuer"
            className="font-mono"
            value={oauth.issuerUrl}
            placeholder="https://auth.linear.app"
            onChange={event => onChange({ ...oauth, issuerUrl: event.target.value })}
            data-testid="settings-mcp-editor-oauth-issuer"
          />
        </>
      ) : null}
      {oauth.discovery === "metadata" ? (
        <>
          <MCPFieldLabel htmlFor="mcp-oauth-metadata" hint="required">
            Metadata URL
          </MCPFieldLabel>
          <Input
            id="mcp-oauth-metadata"
            className="font-mono"
            value={oauth.metadataUrl}
            placeholder="https://auth.linear.app/.well-known/oauth-authorization-server"
            onChange={event => onChange({ ...oauth, metadataUrl: event.target.value })}
            data-testid="settings-mcp-editor-oauth-metadata-url"
          />
        </>
      ) : null}
      {oauth.discovery === "endpoints" ? (
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
          <div>
            <MCPFieldLabel htmlFor="mcp-oauth-authz">Authorization URL</MCPFieldLabel>
            <Input
              id="mcp-oauth-authz"
              className="font-mono"
              value={oauth.authorizationUrl}
              placeholder="https://auth.linear.app/oauth/authorize"
              onChange={event => onChange({ ...oauth, authorizationUrl: event.target.value })}
              data-testid="settings-mcp-editor-oauth-authorization-url"
            />
          </div>
          <div>
            <MCPFieldLabel htmlFor="mcp-oauth-token">Token URL</MCPFieldLabel>
            <Input
              id="mcp-oauth-token"
              className="font-mono"
              value={oauth.tokenUrl}
              placeholder="https://auth.linear.app/oauth/token"
              onChange={event => onChange({ ...oauth, tokenUrl: event.target.value })}
              data-testid="settings-mcp-editor-oauth-token-url"
            />
          </div>
        </div>
      ) : null}
      {error ? <p className="mt-1.5 text-caption text-danger">{error}</p> : null}
    </div>
  );
}
