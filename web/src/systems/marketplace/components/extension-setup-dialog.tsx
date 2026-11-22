import {
  Alert,
  AlertActions,
  AlertDescription,
  AlertTitle,
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Pill,
  Spinner,
} from "@compozy/ui";

import type { ExtensionEntry, ExtensionInputStatus } from "@/systems/extensions";

import { ExtensionInputFields, type ExtensionInputAnnotation } from "./extension-input-fields";
import { ExtensionSetupSignInHint } from "./extension-setup-sign-in";
import { pendingField, useExtensionSetupDialog } from "./use-extension-setup-dialog";

// Only the extension process misses a daemon_env value; an MCP server reads the daemon environment.
function daemonEnvNote(input: ExtensionInputStatus): string {
  return input.targets.includes("subprocess")
    ? "Uses the daemon environment; the extension process does not receive it."
    : "Uses the daemon environment. A value saved here is stored for the MCP server.";
}

interface ExtensionSetupDialogProps {
  extension: ExtensionEntry;
  onClose: () => void;
}

/**
 * Setup for one installed instance. Every label keys off the daemon's `state`, never the `set`
 * boolean, so a `daemon_env` input is never shown as Set. Fields start empty: an empty field keeps
 * the current value, so Save sends only what the operator typed. Mount it only while open.
 */
export function ExtensionSetupDialog({ extension, onClose }: ExtensionSetupDialogProps) {
  const {
    firstPending,
    fieldErrors,
    form,
    formRef,
    inputs,
    remaining,
    save,
    saveError,
    state,
    submit,
  } = useExtensionSetupDialog(extension, onClose);

  return (
    <Dialog onOpenChange={open => (open ? undefined : onClose())} open>
      <DialogContent
        className="flex max-h-[min(var(--height-modal-md),80vh)] flex-col sm:max-w-(--width-modal-sm)"
        data-testid="extension-setup-dialog"
        initialFocus={() => pendingField(formRef.current, firstPending) ?? true}
        unframed
      >
        <form
          className="flex min-h-0 flex-1 flex-col"
          onSubmit={event => {
            event.preventDefault();
            if (!save.isPending && state.data && form.valid) void submit();
          }}
          ref={formRef}
        >
          <DialogHeader variant="ruled">
            <DialogTitle>Set up {extension.name}</DialogTitle>
            <DialogDescription>
              Leave a field empty to keep its current value. Secrets are never shown.
            </DialogDescription>
          </DialogHeader>
          <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-5 py-4">
            {state.isLoading ? (
              <Spinner aria-label="Loading setup" data-testid="extension-setup-loading" />
            ) : state.error ? (
              <Alert data-testid="extension-setup-load-error" variant="destructive">
                <AlertTitle>Setup could not be loaded</AlertTitle>
                <AlertDescription>{state.error.message}</AlertDescription>
                <AlertActions>
                  <Button onClick={() => void state.refetch()} size="sm" type="button">
                    Retry
                  </Button>
                </AlertActions>
              </Alert>
            ) : (
              <>
                {remaining.length > 0 ? (
                  <Alert data-testid="extension-setup-remaining" role="status" variant="warning">
                    <AlertTitle>Still needs setup</AlertTitle>
                    <AlertDescription>{remaining.join(", ")}</AlertDescription>
                  </Alert>
                ) : null}
                {saveError ? (
                  <Alert data-testid="extension-setup-save-error" variant="destructive">
                    <AlertTitle>Setup was not saved</AlertTitle>
                    <AlertDescription>{saveError.message}</AlertDescription>
                  </Alert>
                ) : null}
                <ExtensionInputFields
                  annotate={definition => annotateInput(inputs, definition.id)}
                  disabled={save.isPending}
                  fieldErrors={fieldErrors}
                  form={form}
                />
                <ExtensionSetupSignInHint
                  serverNames={state.data?.mcp_auth_servers ?? []}
                  servers={extension.mcp_servers}
                />
              </>
            )}
          </div>
          <DialogFooter variant="ruled">
            <Button disabled={save.isPending} onClick={onClose} type="button" variant="ghost">
              Cancel
            </Button>
            <Button
              data-testid="extension-setup-save"
              disabled={save.isPending || !state.data || !form.valid}
              type="submit"
            >
              {save.isPending ? <Spinner aria-hidden="true" className="size-3.5" /> : null}
              {save.isPending ? "Saving…" : "Save"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

/**
 * Form definitions for setup: nothing is required and nothing is prefilled, so an empty field
 * keeps the stored value and only typed fields reach the request. State drives the markers.
 */

function annotateInput(
  inputs: readonly ExtensionInputStatus[],
  id: string
): ExtensionInputAnnotation | undefined {
  const input = inputs.find(item => item.id === id);
  if (!input) return undefined;
  switch (input.state) {
    case "set":
      return {
        marker: <span className="text-form-hint whitespace-nowrap text-muted-foreground">Set</span>,
      };
    case "invalid":
      return {
        marker: <SetupStateMarker tone="destructive">Invalid</SetupStateMarker>,
        description: "The stored value is no longer valid. Enter a new one.",
      };
    case "daemon_env":
      return {
        marker: <SetupStateMarker tone="warning">Missing</SetupStateMarker>,
        description: daemonEnvNote(input),
      };
    default:
      return input.required
        ? { marker: <SetupStateMarker tone="warning">Missing</SetupStateMarker> }
        : undefined;
  }
}

function SetupStateMarker({
  children,
  tone,
}: {
  children: string;
  tone: "warning" | "destructive";
}) {
  return (
    <Pill form="plain" tone={tone}>
      <Pill.Dot />
      {children}
    </Pill>
  );
}
