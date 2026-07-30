import { useState } from "react";

import {
  Button,
  Checkbox,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
  Input,
  Label,
  RadioGroup,
  RadioGroupItem,
  Spinner,
} from "@compozy/ui";

import type { ExtensionInstallRequest } from "../types";
import {
  buildExtensionInstallRequest,
  createExtensionInstallForm,
  EXTENSION_INSTALL_SOURCES,
  validateExtensionInstallForm,
  type ExtensionInstallFieldError,
  type ExtensionInstallForm,
  type ExtensionInstallSource,
} from "./extension-install-model";

export interface ExtensionInstallDialogProps {
  error?: string | null;
  open: boolean;
  pending?: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (request: ExtensionInstallRequest) => void;
}

/**
 * Source-union install for extensions the curated catalog does not carry. Consent for unverified
 * archives is not granted here — the daemon's explicit trust dialog owns that decision.
 */
export function ExtensionInstallDialog({
  error,
  open,
  pending = false,
  onOpenChange,
  onSubmit,
}: ExtensionInstallDialogProps) {
  const [form, setForm] = useState<ExtensionInstallForm>(createExtensionInstallForm);
  const [fieldErrors, setFieldErrors] = useState<ExtensionInstallFieldError>({});
  const source = EXTENSION_INSTALL_SOURCES.find(item => item.value === form.source);
  const patch = (next: Partial<ExtensionInstallForm>) => {
    setFieldErrors({});
    setForm(current => ({ ...current, ...next }));
  };

  return (
    <Dialog
      onOpenChange={next => {
        if (!next) {
          setForm(createExtensionInstallForm());
          setFieldErrors({});
        }
        onOpenChange(next);
      }}
      open={open}
    >
      <DialogContent
        className="sm:max-w-(--width-modal-sm)"
        data-testid="extension-install-dialog"
        unframed
      >
        <form
          onSubmit={event => {
            event.preventDefault();
            const errors = validateExtensionInstallForm(form);
            setFieldErrors(errors);
            if (Object.keys(errors).length > 0) return;
            onSubmit(buildExtensionInstallRequest(form));
          }}
        >
          <DialogHeader variant="ruled">
            <DialogTitle>Install an extension</DialogTitle>
            <DialogDescription>
              Install from a local build, a GitHub release, or a git repository. The daemon
              validates the source and records provenance.
            </DialogDescription>
          </DialogHeader>

          <div className="flex flex-col gap-4 px-5 py-4">
            <Field>
              <FieldLabel id="extension-install-source-label">Source</FieldLabel>
              <RadioGroup
                aria-labelledby="extension-install-source-label"
                className="grid gap-2"
                data-testid="extension-install-source"
                onValueChange={value => patch({ source: value as ExtensionInstallSource })}
                value={form.source}
              >
                {EXTENSION_INSTALL_SOURCES.map(option => (
                  <div className="flex items-center gap-2" key={option.value}>
                    <RadioGroupItem
                      data-testid={`extension-install-source-${option.value}`}
                      id={`extension-install-source-${option.value}`}
                      value={option.value}
                    />
                    <Label htmlFor={`extension-install-source-${option.value}`}>
                      {option.label}
                    </Label>
                  </div>
                ))}
              </RadioGroup>
            </Field>

            <Field data-invalid={fieldErrors.ref ? true : undefined}>
              <FieldLabel htmlFor="extension-install-ref">
                {source?.refLabel ?? "Reference"}
              </FieldLabel>
              <Input
                aria-describedby={
                  fieldErrors.ref ? "extension-install-ref-error" : "extension-install-ref-hint"
                }
                aria-invalid={fieldErrors.ref ? true : undefined}
                data-testid="extension-install-ref"
                id="extension-install-ref"
                onChange={event => patch({ ref: event.target.value })}
                placeholder={source?.refPlaceholder}
                value={form.ref}
              />
              {fieldErrors.ref ? (
                <FieldError
                  data-testid="extension-install-ref-error"
                  id="extension-install-ref-error"
                >
                  {fieldErrors.ref}
                </FieldError>
              ) : (
                <FieldDescription id="extension-install-ref-hint">{source?.hint}</FieldDescription>
              )}
            </Field>

            {form.source !== "local_path" ? (
              <Field>
                <FieldLabel htmlFor="extension-install-version">Version</FieldLabel>
                <Input
                  aria-describedby="extension-install-version-hint"
                  data-testid="extension-install-version"
                  id="extension-install-version"
                  onChange={event => patch({ version: event.target.value })}
                  placeholder="0.4.2"
                  value={form.version}
                />
                <FieldDescription id="extension-install-version-hint">
                  Optional. Leave empty to take the latest release the source resolves.
                </FieldDescription>
              </Field>
            ) : null}

            {form.source === "github" ? (
              <Field>
                <FieldLabel htmlFor="extension-install-asset">Asset</FieldLabel>
                <Input
                  aria-describedby="extension-install-asset-hint"
                  data-testid="extension-install-asset"
                  id="extension-install-asset"
                  onChange={event => patch({ asset: event.target.value })}
                  placeholder="hello_darwin_arm64.tar.gz"
                  value={form.asset}
                />
                <FieldDescription id="extension-install-asset-hint">
                  Optional. Required only when a release publishes several archives.
                </FieldDescription>
              </Field>
            ) : null}

            <Field orientation="horizontal">
              <Checkbox
                aria-describedby="extension-install-unverified-hint"
                checked={form.allowUnverified}
                data-testid="extension-install-allow-unverified"
                id="extension-install-unverified"
                onCheckedChange={checked => patch({ allowUnverified: checked === true })}
              />
              <div className="grid gap-1">
                <FieldLabel htmlFor="extension-install-unverified">
                  Allow an unverified archive
                </FieldLabel>
                <FieldDescription id="extension-install-unverified-hint">
                  Opens an explicit consent step before anything is installed.
                </FieldDescription>
              </div>
            </Field>

            {error ? (
              <p
                className="rounded-md bg-danger-tint px-3 py-2 text-small-body text-danger"
                data-testid="extension-install-error"
                role="alert"
              >
                {error}
              </p>
            ) : null}
          </div>

          <DialogFooter variant="ruled">
            <Button
              disabled={pending}
              onClick={() => onOpenChange(false)}
              type="button"
              variant="ghost"
            >
              Cancel
            </Button>
            <Button data-testid="extension-install-submit" disabled={pending} type="submit">
              {pending ? <Spinner aria-hidden="true" className="size-3" /> : null}
              {pending ? "Installing…" : "Install"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
