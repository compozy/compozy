import { LockKeyhole, Plug } from "lucide-react";

import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
  FormSection,
  Input,
  MetadataList,
  Pill,
  RadioCard,
  SecretField,
  Spinner,
} from "@agh/ui";

import type { MarketplaceEntryResponse, MCPInstallRequest, MCPInstallResponse } from "../types";
import { bindingValuePresent, type MCPEnvField, type MCPFieldBinding } from "./mcp-install-model";
import { marketplaceErrorMessage } from "./marketplace-ui";
import { useMCPInstallDialog } from "./use-mcp-install-dialog";

interface MCPInstallDialogProps {
  data: MarketplaceEntryResponse;
  open: boolean;
  workspaceId?: string | null;
  onInstall: (request: MCPInstallRequest) => Promise<MCPInstallResponse>;
  onOpenChange: (open: boolean) => void;
}

function MCPInstallDialog({
  data,
  open,
  workspaceId,
  onInstall,
  onOpenChange,
}: MCPInstallDialogProps) {
  const { entry, mcp } = data;
  const {
    bindings,
    canonicalRef,
    createSecret,
    fields,
    install,
    installError,
    installing,
    putVault,
    remote,
    requiredComplete,
    scope,
    setScope,
    updateBinding,
    vaultQuery,
  } = useMCPInstallDialog({ data, onInstall, onOpenChange, workspaceId });

  if (!mcp) return null;

  const command = [mcp.command, ...(mcp.args ?? [])].filter(Boolean).join(" ");
  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent
        aria-describedby="mcp-install-description"
        className="sm:max-w-(--width-modal-sm)"
        data-testid="mcp-install-dialog"
        unframed
      >
        <DialogHeader className="pr-10" variant="ruled">
          <div className="flex items-start gap-3">
            <span className="inline-flex size-8 shrink-0 items-center justify-center rounded-md bg-surface-glaze text-muted">
              <Plug aria-hidden="true" className="size-4" />
            </span>
            <div className="flex flex-col gap-1.5">
              <p className="eyebrow text-muted">MCP · {mcp.transport} · curated</p>
              <DialogTitle>Install {entry.name}</DialogTitle>
              <DialogDescription id="mcp-install-description">
                {remote
                  ? "AGH writes the configuration, then opens authorization when required."
                  : "Choose a scope and provide the configuration required by this server."}
              </DialogDescription>
            </div>
          </div>
        </DialogHeader>

        <div className="flex max-h-[min(76vh,42rem)] flex-col gap-4 overflow-y-auto px-5 py-4">
          <MetadataList>
            <MetadataList.Row label={remote ? "URL" : "Command"}>
              <code className="break-all font-mono text-form-input text-fg">
                {remote ? mcp.url : command}
              </code>
            </MetadataList.Row>
            <MetadataList.Row label="Transport">{mcp.transport}</MetadataList.Row>
          </MetadataList>

          {remote && mcp.oauth ? (
            <div className="rounded-md border border-info bg-info-tint px-3 py-3">
              <div className="flex items-center gap-2 text-small-body font-medium text-info">
                <LockKeyhole aria-hidden="true" className="size-3.5" />
                Authorization · OAuth 2 PKCE
              </div>
              <MetadataList className="mt-2">
                {mcp.oauth.issuer_url ? (
                  <MetadataList.Row label="Issuer">{mcp.oauth.issuer_url}</MetadataList.Row>
                ) : null}
                {mcp.oauth.scopes?.length ? (
                  <MetadataList.Row label="Scopes">{mcp.oauth.scopes.join(", ")}</MetadataList.Row>
                ) : null}
              </MetadataList>
            </div>
          ) : null}
          {remote ? (
            <p className="text-form-hint text-muted">
              No secret is stored for remote servers. You authorize in the browser after install.
            </p>
          ) : null}

          <FormSection description="Choose where this server is available." title="Scope">
            <div
              aria-label="MCP install scope"
              className="grid grid-cols-1 gap-2 sm:grid-cols-2"
              role="radiogroup"
            >
              <RadioCard
                className="bg-canvas ring-1 ring-line ring-inset data-[selected]:bg-elevated data-[selected]:ring-line-strong"
                description="Available only in this workspace."
                disabled={!workspaceId}
                onSelect={() => setScope("workspace")}
                selected={scope === "workspace"}
                title="Workspace"
              />
              <RadioCard
                className="bg-canvas ring-1 ring-line ring-inset data-[selected]:bg-elevated data-[selected]:ring-line-strong"
                description="Available in every workspace."
                onSelect={() => setScope("global")}
                selected={scope === "global"}
                title="Global"
              />
            </div>
          </FormSection>

          {!remote && fields.length > 0 ? (
            <FormSection title="Required configuration">
              <FieldGroup>
                {fields.map(field => {
                  const binding = bindings[field.name];
                  if (!binding) return null;
                  return (
                    <MCPInstallField
                      binding={binding}
                      canonicalRef={canonicalRef(field)}
                      createPending={putVault.isPending}
                      field={field}
                      key={field.name}
                      onChange={next => updateBinding(field.name, next)}
                      onCreate={() => void createSecret(field)}
                      vaultError={
                        vaultQuery.error
                          ? marketplaceErrorMessage(
                              vaultQuery.error,
                              "Failed to load Vault metadata"
                            )
                          : null
                      }
                      vaultLoading={vaultQuery.isLoading}
                      vaultSecrets={vaultQuery.data ?? []}
                    />
                  );
                })}
              </FieldGroup>
            </FormSection>
          ) : null}

          {installError ? (
            <p
              className="rounded-md bg-danger-tint px-3 py-2 text-small-body text-danger"
              data-testid="mcp-install-error"
              role="alert"
            >
              {installError}
            </p>
          ) : null}
        </div>

        <DialogFooter variant="ruled">
          <Button
            disabled={installing}
            onClick={() => onOpenChange(false)}
            type="button"
            variant="ghost"
          >
            Cancel
          </Button>
          <Button
            data-testid="mcp-install-confirm"
            disabled={!requiredComplete || installing}
            onClick={() => void install()}
            type="button"
          >
            {installing ? <Spinner aria-hidden="true" className="size-3" /> : null}
            {installing ? "Installing…" : "Install"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

interface MCPInstallFieldProps {
  binding: MCPFieldBinding;
  canonicalRef: string;
  createPending: boolean;
  field: MCPEnvField;
  vaultError?: string | null;
  vaultLoading: boolean;
  vaultSecrets: ReadonlyArray<{ present: boolean; ref: string }>;
  onChange: (next: MCPFieldBinding) => void;
  onCreate: () => void;
}

/**
 * Binds one marketplace env field to a control: secrets go through the shared
 * write-only `SecretField`, plain configuration values through a text input.
 */
function MCPInstallField({
  binding,
  canonicalRef,
  createPending,
  field,
  vaultError,
  vaultLoading,
  vaultSecrets,
  onChange,
  onCreate,
}: MCPInstallFieldProps) {
  const id = `mcp-field-${field.name}`;
  const invalid = field.required && !bindingValuePresent(binding);
  const showError = binding.touched && invalid;
  const description =
    field.prompt || (field.required ? "Required by this server." : "Optional configuration.");
  const badges = field.required ? (
    <Pill mono size="xs" tone="warning">
      required
    </Pill>
  ) : null;

  if (!field.secret) {
    return (
      <Field data-invalid={showError ? "true" : undefined}>
        <div className="flex flex-wrap items-center gap-2">
          <FieldLabel className="font-mono" htmlFor={id}>
            {field.name}
          </FieldLabel>
          {badges}
        </div>
        <FieldDescription id={`${id}-description`}>{description}</FieldDescription>
        <Input
          aria-describedby={showError ? `${id}-description ${id}-error` : `${id}-description`}
          aria-invalid={showError ? true : undefined}
          autoComplete="off"
          id={id}
          onBlur={() => onChange({ ...binding, touched: true })}
          onChange={event => onChange({ ...binding, typedValue: event.target.value })}
          placeholder={field.default}
          required={field.required}
          type="text"
          value={binding.typedValue}
        />
        {showError ? <FieldError id={`${id}-error`}>Enter a value.</FieldError> : null}
      </Field>
    );
  }

  return (
    <SecretField
      badges={
        <>
          <Pill mono size="xs">
            secret
          </Pill>
          {badges}
        </>
      }
      binding={{
        create: {
          error: binding.createError,
          onOpenChange: open => onChange({ ...binding, createOpen: open }),
          onSubmit: onCreate,
          onValueChange: next =>
            onChange({ ...binding, createError: undefined, createValue: next }),
          open: binding.createOpen,
          openLabel: "Create Vault secret",
          valueLabel: `New Vault value for ${field.name}`,
          pending: createPending,
          ref: canonicalRef,
          value: binding.createValue,
        },
        emptyLabel: "No MCP secrets are stored yet.",
        error: vaultError,
        loading: vaultLoading,
        namespaceLabel: "namespace=mcp",
        onSelectRef: ref => onChange({ ...binding, touched: true, vaultRef: ref }),
        selectedRef: binding.vaultRef,
        sources: vaultSecrets,
      }}
      createTestId={`mcp-create-secret-${field.name}`}
      description={description}
      error={showError ? "Choose a present Vault ref or enter a value." : undefined}
      id={id}
      label={field.name}
      labelClassName="font-mono"
      mode={binding.mode === "vault" ? "source" : "value"}
      onBlur={() => onChange({ ...binding, touched: true })}
      onModeChange={next => onChange({ ...binding, mode: next === "source" ? "vault" : "typed" })}
      onValueChange={next => onChange({ ...binding, typedValue: next })}
      placeholder="Stored write-only during install"
      required={field.required}
      sourceModeLabel="Use Vault"
      sourcesTestId={`mcp-vault-selector-${field.name}`}
      value={binding.typedValue}
    />
  );
}

export { MCPInstallDialog };
export type { MCPInstallDialogProps };
