import { Copy } from "@compozy/ui/icons";
import { useState } from "react";

import {
  Alert,
  AlertDescription,
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
  Input,
  PillGroup,
} from "@compozy/ui";

import type { MCPCopyFailure } from "../lib/mcp-runtime-view-model";
import type { RuntimeMCPImportRequest, RuntimeMCPImportScope, RuntimeMCPServer } from "../types";

type CopyScope = Exclude<RuntimeMCPImportScope, "agent">;

export interface MCPCopyToCompozyDialogProps {
  server: RuntimeMCPServer;
  runtimeName: string;
  /** The runtime can turn its own servers off for CompozyOS sessions. */
  canTurnOff: boolean;
  /** Named profile of the shell's lens; null on the default profile (no "This profile"). */
  profile: string | null;
  /** Project in view; null hides "This project" as a choice. */
  workspaceId: string | null;
  submitting: boolean;
  failure: MCPCopyFailure | null;
  onSubmit: (request: RuntimeMCPImportRequest) => void;
  onClose: () => void;
}

const NAME_TAKEN = "mcp_import_name_taken";
const SOURCE_GONE = "mcp_server_not_found";

/**
 * Copy to CompozyOS: confirm where the copy lives and its name. Values move into CompozyOS secrets
 * daemon-side; the runtime's own file is never touched. A taken name keeps the dialog open with
 * the error on the name field; a server gone from its file says so and blocks the copy.
 */
export function MCPCopyToCompozyDialog({
  server,
  runtimeName,
  canTurnOff,
  profile,
  workspaceId,
  submitting,
  failure,
  onSubmit,
  onClose,
}: MCPCopyToCompozyDialogProps) {
  const [scope, setScope] = useState<CopyScope>("user");
  const [name, setName] = useState(server.name);
  const keys = [...server.env_keys, ...server.header_keys];
  const remote = server.transport !== "stdio";
  const nameTaken = failure?.code === NAME_TAKEN;
  const sourceGone = failure?.code === SOURCE_GONE;
  const otherFailure = failure && !nameTaken && !sourceGone ? failure : null;
  const trimmed = name.trim();
  const submit = () => {
    if (submitting || sourceGone || trimmed === "") return;
    onSubmit({
      scope,
      ...(trimmed !== server.name ? { name: trimmed } : {}),
      ...(scope === "profile" && profile ? { profile } : {}),
      ...(scope === "workspace" && workspaceId ? { workspace_id: workspaceId } : {}),
    });
  };
  return (
    <Dialog onOpenChange={next => (next || submitting ? undefined : onClose())} open>
      <DialogContent
        className={`grid-rows-[auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("sm")}`}
        data-testid="settings-mcp-copy-dialog"
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description={
            <>
              From <span className="font-mono">{server.source_label}</span>. {runtimeName}'s file
              stays as it is.
            </>
          }
          eyebrow={`${runtimeName} · MCP server`}
          icon={Copy}
          onClose={submitting ? undefined : onClose}
          title={`Copy ${server.name} to CompozyOS`}
        />
        <EntityDialogBody className="flex flex-col gap-5">
          {sourceGone ? (
            <Alert role="alert" variant="warning">
              <AlertDescription>{failure.message}</AlertDescription>
            </Alert>
          ) : null}
          {otherFailure ? (
            <Alert role="alert" variant="destructive">
              <AlertDescription>{otherFailure.message}</AlertDescription>
            </Alert>
          ) : null}
          <Field>
            <FieldLabel>Available in</FieldLabel>
            <PillGroup<CopyScope>
              aria-label="Available in"
              items={[
                { value: "user", label: "Personal" },
                ...(profile ? [{ value: "profile" as const, label: "This profile" }] : []),
                { value: "workspace", label: "This project", disabled: !workspaceId },
              ]}
              onChange={setScope}
              size="sm"
              value={scope}
            />
          </Field>
          <Field data-invalid={nameTaken ? "true" : undefined}>
            <FieldLabel htmlFor="copy-mcp-name">Name in CompozyOS</FieldLabel>
            <Input
              aria-invalid={nameTaken || undefined}
              className="font-mono"
              disabled={submitting}
              id="copy-mcp-name"
              onChange={event => setName(event.target.value)}
              value={name}
            />
            {nameTaken ? <FieldError>{failure.message}. Pick another name.</FieldError> : null}
          </Field>
          <ul className="flex flex-col gap-1.5 text-small-body text-muted-foreground">
            {keys.length > 0 ? (
              <li>
                Values for <span className="font-mono">{keys.join(", ")}</span> are stored as
                CompozyOS secrets.
              </li>
            ) : null}
            {remote ? <li>If it asks you to sign in, you'll sign in once in CompozyOS.</li> : null}
            <li>
              {runtimeName} will still load its own copy.
              {canTurnOff ? " You can turn its servers off for CompozyOS sessions." : ""}
            </li>
          </ul>
          <FieldDescription>Open sessions can use it from their next message.</FieldDescription>
        </EntityDialogBody>
        <EntityDialogFooter
          cancelDisabled={submitting}
          isSaving={submitting}
          onCancel={onClose}
          onPrimary={submit}
          primaryDisabled={sourceGone || submitting || trimmed === ""}
          primaryIcon={Copy}
          primaryLabel={submitting ? "Copying…" : "Copy to CompozyOS"}
          primaryTestId="settings-mcp-copy-submit"
        />
      </DialogContent>
    </Dialog>
  );
}
