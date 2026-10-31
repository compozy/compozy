import { useState, type FormEvent } from "react";
import { FolderPlus, Pencil, Trash2 } from "@compozy/ui/icons";

import {
  Button,
  ConfirmDialog,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Field,
  FieldError,
  FieldLabel,
  Input,
  Spinner,
} from "@compozy/ui";

import type { SessionFolderDialogState } from "../../hooks/use-session-list-folder-layout";

/** Mirrors the daemon's folder name bound (bytes; ASCII-dominant names stay well inside). */
const FOLDER_NAME_MAX_CHARACTERS = 120;

export interface SessionFolderDialogsProps {
  state: SessionFolderDialogState<{ id: string; name: string }>;
  /** What the folder holds; names what returns to the list when it is deleted. */
  holds?: "sessions" | "projects";
  onClose: () => void;
  onCreate: (name: string) => Promise<unknown>;
  onRename: (folderId: string, name: string) => Promise<unknown>;
  onDelete: (folderId: string) => Promise<unknown>;
  /** Pickers for the parts of the folder's destination the opener did not settle. */
  destination?: React.ReactNode;
}

/** New folder, rename folder, and the delete confirmation for the session list. */
export function SessionFolderDialogs({
  state,
  onClose,
  onCreate,
  onRename,
  onDelete,
  destination,
  holds = "sessions",
}: SessionFolderDialogsProps) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const run = (work: () => Promise<unknown>) => {
    setPending(true);
    setError(null);
    work()
      .then(onClose)
      .catch((cause: unknown) => setError(cause instanceof Error ? cause.message : String(cause)))
      .finally(() => setPending(false));
  };
  const close = (open: boolean) => {
    if (open || pending) return;
    setError(null);
    onClose();
  };

  if (state?.kind === "delete") {
    return (
      <ConfirmDialog
        open
        onOpenChange={close}
        tone="danger"
        icon={Trash2}
        title="Delete folder?"
        description={
          holds === "projects"
            ? "Projects in this folder move back to the list. No projects or sessions are deleted."
            : "Sessions in this folder move back to the list. No sessions are deleted."
        }
        confirmLabel="Delete folder"
        cancelLabel="Cancel"
        isPending={pending}
        error={error}
        onConfirm={() => run(() => onDelete(state.folder.id))}
        contentProps={{ "data-testid": "session-folder-delete-dialog" }}
        confirmButtonProps={{ "data-testid": "session-folder-delete-confirm" }}
      />
    );
  }

  const rename = state?.kind === "rename" ? state.folder : null;
  return (
    <Dialog open={state !== null} onOpenChange={close}>
      <DialogContent
        showCloseButton={!pending}
        className="max-w-md"
        data-testid="session-folder-name-dialog"
      >
        {state !== null ? (
          <SessionFolderNameForm
            key={rename?.id ?? "create"}
            {...folderNameCopy(state)}
            initialName={rename?.name ?? ""}
            destination={rename ? undefined : destination}
            pending={pending}
            requestError={error}
            onCancel={() => close(false)}
            onConfirm={name => run(() => (rename ? onRename(rename.id, name) : onCreate(name)))}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  );
}

/** Title, description, and confirm button for the create and rename dialogs. */
function folderNameCopy(
  state: Exclude<SessionFolderDialogState<{ id: string; name: string }>, null | { kind: "delete" }>
) {
  if (state.kind === "rename") {
    return { title: "Rename folder", confirmLabel: "Rename folder", confirmIcon: Pencil };
  }
  const count = state.moveCount;
  if (count === undefined) {
    return { title: "New folder", confirmLabel: "Create folder", confirmIcon: FolderPlus };
  }
  return {
    title: "New folder",
    description:
      count === 1
        ? "The session you selected moves into it."
        : `The ${count} sessions you selected move into it.`,
    confirmLabel: "Create and move",
    confirmIcon: FolderPlus,
  };
}

function SessionFolderNameForm({
  title,
  description,
  confirmLabel,
  confirmIcon: ConfirmIcon,
  initialName,
  destination,
  pending,
  requestError,
  onCancel,
  onConfirm,
}: {
  title: string;
  description?: string;
  confirmLabel: string;
  confirmIcon: typeof Pencil;
  initialName: string;
  destination?: React.ReactNode;
  pending: boolean;
  requestError: string | null;
  onCancel: () => void;
  onConfirm: (name: string) => void;
}) {
  const [name, setName] = useState(initialName);
  const normalized = name.trim();
  const validationError =
    normalized.length === 0
      ? "Enter a folder name."
      : Array.from(normalized).length > FOLDER_NAME_MAX_CHARACTERS
        ? `Use ${FOLDER_NAME_MAX_CHARACTERS} characters or fewer.`
        : null;
  // An empty field is the starting state, not a mistake worth flagging.
  const shownError = (normalized.length > 0 ? validationError : null) ?? requestError;
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (pending || validationError) return;
    onConfirm(normalized);
  };
  return (
    <form onSubmit={submit} className="flex flex-col gap-4">
      <DialogHeader>
        <DialogTitle>{title}</DialogTitle>
        {description ? <DialogDescription>{description}</DialogDescription> : null}
      </DialogHeader>
      {destination}
      <Field data-invalid={Boolean(shownError)}>
        <FieldLabel htmlFor="session-folder-name">Name</FieldLabel>
        <Input
          id="session-folder-name"
          value={name}
          onChange={event => setName(event.target.value)}
          disabled={pending}
          aria-invalid={Boolean(shownError)}
          autoFocus
          data-testid="session-folder-name"
        />
        {shownError ? (
          <FieldError data-testid="session-folder-name-error">{shownError}</FieldError>
        ) : null}
      </Field>
      <DialogFooter className="gap-2">
        <Button type="button" variant="ghost" onClick={onCancel} disabled={pending}>
          Cancel
        </Button>
        <Button
          type="submit"
          disabled={pending || Boolean(validationError)}
          data-testid="session-folder-name-confirm"
        >
          {pending ? <Spinner className="size-3" /> : <ConfirmIcon className="size-3" />}
          {confirmLabel}
        </Button>
      </DialogFooter>
    </form>
  );
}
