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
  SecretField,
  Spinner,
  Switch,
} from "@compozy/ui";

import type { MarketplaceEntryResponse, MCPInstallRequest, MCPInstallResponse } from "../types";
import { bindingValuePresent, type MCPFieldBinding, type MCPInputField } from "./mcp-install-model";
import { formatMarketplaceMCPLaunch, marketplaceErrorMessage } from "./marketplace-ui";
import { useMCPInstallDialog } from "./use-mcp-install-dialog";
import { WorkspaceScopeStatement, destinationLabel } from "@/systems/workspace";

interface MCPInstallDialogProps {
  data: MarketplaceEntryResponse;
  open: boolean;
  workspaceId?: string | null;
  scope?: "global" | "workspace";
  workspaceName?: string | null;
  onInstall: (request: MCPInstallRequest) => Promise<MCPInstallResponse>;
  onOpenChange: (open: boolean) => void;
}

function MCPInstallDialog({
  data,
  open,
  workspaceId,
  scope: requestedScope,
  workspaceName,
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
    updateBinding,
    vaultQuery,
  } = useMCPInstallDialog({ data, onInstall, onOpenChange, workspaceId, scope: requestedScope });

  if (!mcp) return null;

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
              <p className="eyebrow text-muted">MCP · curated</p>
              <DialogTitle>Install {entry.name}</DialogTitle>
              <DialogDescription id="mcp-install-description">
                Provide the configuration required by this server. After installation, use the
                Authorize action when the catalog requires it.
              </DialogDescription>
            </div>
          </div>
        </DialogHeader>

        <div className="flex max-h-[min(76vh,42rem)] flex-col gap-4 overflow-y-auto px-5 py-4">
          <MetadataList>
            <MetadataList.Row label="Launch">
              <code className="break-all font-mono text-form-input text-fg">
                {formatMarketplaceMCPLaunch(mcp.launch)}
              </code>
            </MetadataList.Row>
            <MetadataList.Row label="Connection">
              {remote ? "Hosted HTTP" : "Local process"}
            </MetadataList.Row>
          </MetadataList>

          {mcp.auth ? (
            <div className="rounded-md border border-info bg-info-tint px-3 py-3">
              <div className="flex items-center gap-2 text-small-body font-medium text-info">
                <LockKeyhole aria-hidden="true" className="size-3.5" />
                Authorization · OAuth
              </div>
              <MetadataList className="mt-2">
                <MetadataList.Row label="Registration">
                  {mcp.auth.registration === "pre_registered" ? "Pre-registered" : "Automatic"}
                </MetadataList.Row>
                {mcp.auth.scopes?.length ? (
                  <MetadataList.Row label="Scopes">{mcp.auth.scopes.join(", ")}</MetadataList.Row>
                ) : null}
              </MetadataList>
            </div>
          ) : null}

          {fields.length > 0 ? (
            <FormSection title="Required configuration">
              <FieldGroup>
                {fields.map(field => {
                  const binding = bindings[field.id];
                  if (!binding) return null;
                  return (
                    <MCPInstallField
                      binding={binding}
                      canonicalRef={canonicalRef(field)}
                      createPending={putVault.isPending}
                      field={field}
                      key={field.id}
                      onChange={next => updateBinding(field.id, next)}
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
          <p className="min-w-0 flex-1 text-form-hint text-muted">
            <WorkspaceScopeStatement
              destination={destinationLabel(scope, workspaceName)}
              kind="install"
              scope={scope}
              variant="note"
            />
          </p>
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
  field: MCPInputField;
  vaultError?: string | null;
  vaultLoading: boolean;
  vaultSecrets: ReadonlyArray<{ present: boolean; ref: string }>;
  onChange: (next: MCPFieldBinding) => void;
  onCreate: () => void;
}

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
  const id = `mcp-input-${field.id}`;
  const invalid = field.required && !bindingValuePresent(binding);
  const showError = binding.touched && invalid;
  const badges = field.required ? (
    <Pill mono size="xs" tone="warning">
      required
    </Pill>
  ) : null;

  if (field.type === "boolean") {
    return (
      <Field>
        <div className="flex items-start justify-between gap-3">
          <div className="flex flex-col gap-1">
            <div className="flex flex-wrap items-center gap-2">
              <FieldLabel htmlFor={id}>{field.id}</FieldLabel>
              {badges}
            </div>
            <FieldDescription id={`${id}-description`}>{field.prompt}</FieldDescription>
          </div>
          <Switch
            aria-describedby={`${id}-description`}
            checked={binding.typedValue === "true"}
            id={id}
            onCheckedChange={next =>
              onChange({ ...binding, touched: true, typedValue: String(next) })
            }
          />
        </div>
      </Field>
    );
  }

  if (field.type !== "secret") {
    return (
      <Field data-invalid={showError ? "true" : undefined}>
        <div className="flex flex-wrap items-center gap-2">
          <FieldLabel className="font-mono" htmlFor={id}>
            {field.id}
          </FieldLabel>
          {badges}
        </div>
        <FieldDescription id={`${id}-description`}>{field.prompt}</FieldDescription>
        <Input
          aria-describedby={showError ? `${id}-description ${id}-error` : `${id}-description`}
          aria-invalid={showError ? true : undefined}
          autoComplete="off"
          id={id}
          onBlur={() => onChange({ ...binding, touched: true })}
          onChange={event => onChange({ ...binding, typedValue: event.target.value })}
          placeholder={typeof field.default === "string" ? field.default : undefined}
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
          valueLabel: `New Vault value for ${field.id}`,
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
      createTestId={`mcp-create-secret-${field.id}`}
      description={field.prompt}
      error={showError ? "Choose a present Vault ref or enter a value." : undefined}
      id={id}
      label={field.id}
      labelClassName="font-mono"
      mode={binding.mode === "vault" ? "source" : "value"}
      onBlur={() => onChange({ ...binding, touched: true })}
      onModeChange={next => onChange({ ...binding, mode: next === "source" ? "vault" : "typed" })}
      onValueChange={next => onChange({ ...binding, typedValue: next })}
      placeholder="Stored write-only during install"
      required={field.required}
      sourceModeLabel="Use Vault"
      sourcesTestId={`mcp-vault-selector-${field.id}`}
      value={binding.typedValue}
    />
  );
}

export { MCPInstallDialog };
export type { MCPInstallDialogProps };
