import { BookOpen, Check } from "lucide-react";
import { useReducer } from "react";

import {
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  Field,
  FieldContent,
  FieldHeader,
  FieldLabel,
  FormSection,
  HelpTip,
  ImmutableIdentity,
  Input,
  RequiredMark,
  Textarea,
} from "@compozy/ui";

import { knowledgeTypeLabel } from "@/systems/knowledge/lib/knowledge-formatters";
import type { MemoryType } from "@/systems/knowledge/types";

interface KnowledgeEditDialogProps {
  open: boolean;
  onOpenChange: (next: boolean) => void;
  filename: string;
  /** Locked identity — `MemoryEditRequest` cannot rename an entry. */
  name: string;
  /** Locked identity — retrieval keys off the type, so it cannot change. */
  type: MemoryType;
  initialContent: string;
  initialDescription?: string;
  isPending: boolean;
  error?: string | null;
  onConfirm: (input: { content: string; description?: string }) => Promise<void>;
}

function stringReducer(_: string, next: string): string {
  return next;
}

function KnowledgeEditDialog({
  open,
  onOpenChange,
  filename,
  name,
  type,
  initialContent,
  initialDescription,
  isPending,
  error,
  onConfirm,
}: KnowledgeEditDialogProps) {
  const [content, setContent] = useReducer(stringReducer, initialContent);
  const [description, setDescription] = useReducer(stringReducer, initialDescription ?? "");

  if (!open) {
    return null;
  }

  const handleOpenChange = (next: boolean) => {
    if (next) {
      setContent(initialContent);
      setDescription(initialDescription ?? "");
    }
    onOpenChange(next);
  };

  const handleSubmit = async () => {
    const trimmedDescription = description.trim();
    await onConfirm({
      content,
      description: trimmedDescription === "" ? undefined : trimmedDescription,
    });
  };

  const initialDescriptionValue = initialDescription ?? "";
  const isDirty = content !== initialContent || description !== initialDescriptionValue;
  const submitDisabled = isPending || content.trim().length === 0 || !isDirty;

  return (
    <Dialog onOpenChange={handleOpenChange} open={open}>
      <DialogContent
        className={`grid-rows-[auto_minmax(0,1fr)_auto_auto] ${dialogShellClass("sm")}`}
        data-testid="knowledge-edit-dialog"
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description="Update the summary or the content."
          eyebrow="Knowledge"
          icon={BookOpen}
          onClose={() => onOpenChange(false)}
          title="Edit knowledge entry"
        />
        <EntityDialogBody className="flex flex-col">
          <ImmutableIdentity
            data-testid="knowledge-edit-identity"
            hint="Name and kind can't change. Create a new entry to use different ones."
            rows={[
              { label: "Name", value: name, mono: true },
              { label: "Kind", value: knowledgeTypeLabel(type) },
              { label: "File", value: filename, mono: true },
            ]}
          />
          <FormSection title="The content">
            <div className="flex flex-col gap-4">
              <Field>
                <FieldHeader>
                  <FieldLabel htmlFor="knowledge-edit-description">Description</FieldLabel>
                  <HelpTip label="About description">
                    A short summary agents use to find this.
                  </HelpTip>
                </FieldHeader>
                <Input
                  data-testid="knowledge-edit-description"
                  id="knowledge-edit-description"
                  onChange={event => setDescription(event.target.value)}
                  placeholder="Optional summary"
                  value={description}
                />
              </Field>
              <Field>
                <FieldContent>
                  <FieldLabel htmlFor="knowledge-edit-content">
                    Content
                    <RequiredMark />
                  </FieldLabel>
                </FieldContent>
                <Textarea
                  className="h-60 font-mono text-small-body"
                  data-testid="knowledge-edit-content"
                  id="knowledge-edit-content"
                  onChange={event => setContent(event.target.value)}
                  value={content}
                />
              </Field>
            </div>
          </FormSection>
        </EntityDialogBody>
        {error ? (
          <div
            className="border-t border-line px-5 py-3 text-form-hint text-danger"
            data-testid="knowledge-edit-dialog-error"
            role="alert"
          >
            {error}
          </div>
        ) : null}
        <EntityDialogFooter
          cancelTestId="cancel-edit-memory-btn"
          isSaving={isPending}
          onCancel={() => onOpenChange(false)}
          onPrimary={handleSubmit}
          primaryDisabled={submitDisabled}
          primaryIcon={Check}
          primaryLabel="Save changes"
          primaryTestId="confirm-edit-memory-btn"
        />
      </DialogContent>
    </Dialog>
  );
}

export { KnowledgeEditDialog };
export type { KnowledgeEditDialogProps };
