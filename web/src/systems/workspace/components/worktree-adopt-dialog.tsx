import { ShieldAlert } from "lucide-react";

import {
  Alert,
  AlertDescription,
  Button,
  ConfirmDialog,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Eyebrow,
  MonoId,
} from "@compozy/ui";

import type { WorktreeRefusal } from "../lib/worktree-refusal";
import type { DiscoveredWorktreePayload } from "../types";
import { WorktreePath } from "./worktree-path";

export interface WorktreeAdoptDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  discovered: DiscoveredWorktreePayload | null;
  /** Set when the daemon refused adoption; replaces the confirm entirely. */
  refusal: WorktreeRefusal | null;
  isAdopting: boolean;
  onConfirm: () => void;
}

/**
 * Selecting a discovered row is the adoption gesture (ADR-002), so this is the
 * one consent moment. It states plainly what adoption does *not* do, because
 * the surprising outcomes would be a re-run bootstrap or a moved directory.
 */
export function WorktreeAdoptDialog({
  open,
  onOpenChange,
  discovered,
  refusal,
  isAdopting,
  onConfirm,
}: WorktreeAdoptDialogProps) {
  if (refusal) {
    return (
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent
          aria-describedby="worktree-adopt-refusal-reason"
          aria-labelledby="worktree-adopt-refusal-title"
          className="sm:max-w-md"
          data-testid="worktree-adopt-refusal"
          role="alertdialog"
          showCloseButton={false}
          unframed
        >
          <DialogHeader variant="ruled">
            <Eyebrow className="text-warning">Adoption blocked</Eyebrow>
            <DialogTitle id="worktree-adopt-refusal-title">
              {discovered ? `${discovered.name} cannot be adopted` : "Adoption blocked"}
            </DialogTitle>
          </DialogHeader>
          <div className="flex min-w-0 flex-col gap-4 px-5 pt-4 pb-5">
            <Alert variant="danger">
              <AlertDescription
                data-testid="worktree-adopt-refusal-reason"
                id="worktree-adopt-refusal-reason"
              >
                {refusal.message}
              </AlertDescription>
            </Alert>
            <p className="text-form-hint text-subtle">The directory stays exactly where it is.</p>
          </div>
          <DialogFooter variant="ruled">
            <Button size="sm" onClick={() => onOpenChange(false)}>
              Close
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    );
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      tone="accent"
      title={discovered ? `Adopt ${discovered.name}?` : "Adopt worktree?"}
      description="Its metadata resolves to this repository. CompozyOS manages it in place — bootstrap is not re-run, the directory is not moved."
      note="Sessions and runs can target it once adopted."
      noteTone="info"
      body={
        discovered ? (
          <div
            data-testid="worktree-adopt-target"
            className="flex min-w-0 flex-col gap-1 rounded-md border border-line bg-canvas-soft p-3"
          >
            <span className="min-w-0 truncate text-small-body font-semibold text-fg-strong">
              {discovered.name}
            </span>
            <MonoId className="w-full" value={discovered.branch} preserveCase size="sm" />
            {/* Discovered externals keep their foreign path after adoption. */}
            <WorktreePath className="w-full" path={discovered.path} />
          </div>
        ) : null
      }
      confirmLabel="Adopt worktree"
      cancelLabel="Cancel"
      isPending={isAdopting}
      onConfirm={onConfirm}
      confirmIcon={ShieldAlert}
      contentProps={{ "data-testid": "worktree-adopt-confirm" }}
      confirmButtonProps={{ "data-testid": "worktree-adopt-submit" }}
    />
  );
}
