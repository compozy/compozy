/**
 * Copy to CompozyOS (`_uiux.md` S2): confirm where the copy lives and its name. Values move into
 * CompozyOS secrets daemon-side; the runtime's own file is never touched.
 */
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

import type { RuntimeServer } from "./runtime-fixtures";

export type CopyDialogCase = "default" | "name-taken" | "source-changed" | "submitting";

type CopyScope = "user" | "profile" | "workspace";

export interface CopyDialogProps {
  server: RuntimeServer;
  runtime: string;
  canTurnOff: boolean;
  which: CopyDialogCase;
  onClose: () => void;
}

export function CopyToCompozyDialog({
  server,
  runtime,
  canTurnOff,
  which,
  onClose,
}: CopyDialogProps) {
  const [scope, setScope] = useState<CopyScope>("user");
  const [name, setName] = useState(server.name);
  const keys = [...server.envKeys, ...server.headerKeys];
  const remote = server.transport !== "stdio";
  const submitting = which === "submitting";
  return (
    <Dialog onOpenChange={next => (next ? undefined : onClose())} open>
      <DialogContent
        className={`grid-rows-[auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("sm")}`}
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description={
            <>
              From <span className="font-mono">{server.sourceLabel}</span>. {runtime}'s file stays
              as it is.
            </>
          }
          eyebrow={`${runtime} · MCP server`}
          icon={Copy}
          onClose={submitting ? undefined : onClose}
          title={`Copy ${server.name} to CompozyOS`}
        />
        <EntityDialogBody className="flex flex-col gap-5">
          <div className="flex items-center gap-2 rounded-md bg-surface-2 px-3 py-2 font-mono text-small-body">
            <span className="text-fg">{server.name}</span>
            <span className="text-subtle">·</span>
            <span className="truncate text-muted-foreground">{server.target}</span>
          </div>
          {which === "source-changed" ? (
            <Alert role="alert" variant="warning">
              <AlertDescription>
                The server {server.name} is no longer in {server.sourceLabel}. Close this and check
                the list again.
              </AlertDescription>
            </Alert>
          ) : null}
          <Field>
            <FieldLabel>Available in</FieldLabel>
            <PillGroup<CopyScope>
              aria-label="Available in"
              items={[
                { value: "user", label: "Personal" },
                { value: "profile", label: "This profile" },
                { value: "workspace", label: "This project" },
              ]}
              onChange={setScope}
              size="sm"
              value={scope}
            />
          </Field>
          <Field data-invalid={which === "name-taken" ? "true" : undefined}>
            <FieldLabel htmlFor="copy-mcp-name">Name in CompozyOS</FieldLabel>
            <Input
              aria-invalid={which === "name-taken"}
              className="font-mono"
              disabled={submitting}
              id="copy-mcp-name"
              onChange={event => setName(event.target.value)}
              value={name}
            />
            {which === "name-taken" ? (
              <FieldError>
                A server named {server.name} is already in CompozyOS here. Pick another name.
              </FieldError>
            ) : null}
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
              {runtime} will still load its own copy.
              {canTurnOff ? " You can turn its servers off for CompozyOS sessions." : ""}
            </li>
          </ul>
          <FieldDescription>Open sessions can use it from their next message.</FieldDescription>
        </EntityDialogBody>
        <EntityDialogFooter
          cancelDisabled={submitting}
          isSaving={submitting}
          onCancel={onClose}
          onPrimary={onClose}
          primaryDisabled={which === "source-changed" || submitting}
          primaryIcon={Copy}
          primaryLabel={submitting ? "Copying…" : "Copy to CompozyOS"}
        />
      </DialogContent>
    </Dialog>
  );
}
