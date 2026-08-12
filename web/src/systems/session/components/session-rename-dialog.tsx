import { useState, type FormEvent } from "react";
import { Pencil } from "lucide-react";

import {
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Field,
  FieldError,
  FieldLabel,
  Input,
  Spinner,
} from "@compozy/ui";

import type { SessionPayload } from "../types";

const SESSION_NAME_MAX_CHARACTERS = 64;

export interface SessionRenameDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  session: SessionPayload;
  isRenaming: boolean;
  onConfirm: (name: string) => void;
}

interface SessionRenameFormProps {
  session: SessionPayload;
  isRenaming: boolean;
  onCancel: () => void;
  onConfirm: (name: string) => void;
}

function SessionRenameForm({ session, isRenaming, onCancel, onConfirm }: SessionRenameFormProps) {
  const [name, setName] = useState(session.name ?? "");
  const normalizedName = name.trim();
  const characterCount = Array.from(normalizedName).length;
  const error =
    normalizedName.length === 0
      ? "Enter a session name."
      : characterCount > SESSION_NAME_MAX_CHARACTERS
        ? `Use ${SESSION_NAME_MAX_CHARACTERS} characters or fewer.`
        : null;

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (isRenaming || error) return;
    onConfirm(normalizedName);
  };

  return (
    <form onSubmit={submit} className="flex flex-col gap-4">
      <DialogHeader>
        <DialogTitle>Rename session</DialogTitle>
      </DialogHeader>
      <Field data-invalid={Boolean(error)}>
        <FieldLabel htmlFor="session-rename-name">Name</FieldLabel>
        <Input
          id="session-rename-name"
          value={name}
          onChange={event => setName(event.target.value)}
          disabled={isRenaming}
          aria-invalid={Boolean(error)}
          autoFocus
          data-testid="session-rename-name"
        />
        {error ? <FieldError data-testid="session-rename-error">{error}</FieldError> : null}
      </Field>
      <DialogFooter className="gap-2">
        <Button
          type="button"
          variant="ghost"
          onClick={onCancel}
          disabled={isRenaming}
          data-testid="session-rename-cancel"
        >
          Cancel
        </Button>
        <Button
          type="submit"
          disabled={isRenaming || Boolean(error)}
          data-testid="session-rename-confirm"
        >
          {isRenaming ? (
            <>
              <Spinner className="size-3" />
              Renaming
            </>
          ) : (
            <>
              <Pencil className="size-3" />
              Rename session
            </>
          )}
        </Button>
      </DialogFooter>
    </form>
  );
}

/** Shared, workspace-scoped rename surface for every user-session entry point. */
export function SessionRenameDialog({
  open,
  onOpenChange,
  session,
  isRenaming,
  onConfirm,
}: SessionRenameDialogProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={nextOpen => {
        if (isRenaming && !nextOpen) return;
        onOpenChange(nextOpen);
      }}
    >
      <DialogContent
        showCloseButton={!isRenaming}
        className="max-w-md"
        data-testid="session-rename-dialog"
      >
        <SessionRenameForm
          key={`${session.id}:${open ? "open" : "closed"}`}
          session={session}
          isRenaming={isRenaming}
          onCancel={() => onOpenChange(false)}
          onConfirm={onConfirm}
        />
      </DialogContent>
    </Dialog>
  );
}
