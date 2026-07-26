import { BookOpen, ClipboardList, MessageSquare, Plus, Tag } from "lucide-react";
import { useState } from "react";

import {
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  Eyebrow,
  Field,
  FieldContent,
  FieldDescription,
  FieldLabel,
  FormSection,
  Input,
  RadioCard,
  Textarea,
} from "@agh/ui";

import type { MemoryType } from "@/systems/knowledge/types";

interface KnowledgeCreateInput {
  type: MemoryType;
  name: string;
  description?: string;
  content: string;
}

interface KnowledgeCreateDialogProps {
  open: boolean;
  onOpenChange: (next: boolean) => void;
  scope: string;
  defaultType: MemoryType;
  isPending: boolean;
  error?: string | null;
  onConfirm: (input: KnowledgeCreateInput) => Promise<void>;
}

interface TypeOption {
  value: MemoryType;
  title: string;
  description: string;
  icon: typeof BookOpen;
}

const TYPE_OPTIONS: ReadonlyArray<TypeOption> = [
  {
    value: "user",
    title: "User",
    description: "Operator preferences and identity guidance.",
    icon: ClipboardList,
  },
  {
    value: "feedback",
    title: "Feedback",
    description: "Coaching notes captured from prior runs.",
    icon: MessageSquare,
  },
  {
    value: "project",
    title: "Project",
    description: "Long-lived decisions with rationale.",
    icon: BookOpen,
  },
  {
    value: "reference",
    title: "Reference",
    description: "Pointer to docs, code, or external systems.",
    icon: Tag,
  },
];

function KnowledgeCreateDialog({
  open,
  onOpenChange,
  scope,
  defaultType,
  isPending,
  error,
  onConfirm,
}: KnowledgeCreateDialogProps) {
  const [selectedType, setSelectedType] = useState<MemoryType | null>(null);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [content, setContent] = useState("");
  const type = selectedType ?? defaultType;

  const resetDraft = () => {
    setSelectedType(null);
    setName("");
    setDescription("");
    setContent("");
  };

  const updateDialogOpen = (next: boolean) => {
    if (next) {
      resetDraft();
    }
    onOpenChange(next);
  };

  const handleSubmit = async () => {
    const trimmedDescription = description.trim();
    try {
      await onConfirm({
        type,
        name: name.trim(),
        description: trimmedDescription === "" ? undefined : trimmedDescription,
        content,
      });
    } catch {
      // Error state is surfaced through `error` and the dialog stays open.
    }
  };

  const submitDisabled = isPending || name.trim().length === 0 || content.trim().length === 0;

  return (
    <Dialog onOpenChange={updateDialogOpen} open={open}>
      <DialogContent
        className={`grid-rows-[auto_minmax(0,1fr)_auto_auto] ${dialogShellClass("sm")}`}
        data-testid="knowledge-create-dialog"
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description={`Add knowledge in the ${scope} scope. The entry is recorded as a decision and becomes available to matching future recall.`}
          eyebrow="Catalog · Knowledge"
          icon={BookOpen}
          onClose={() => updateDialogOpen(false)}
          title="Create knowledge entry"
        />
        <EntityDialogBody className="flex flex-col">
          <FormSection
            description="The form decides how agents consume it."
            title="What kind of knowledge?"
          >
            <div
              aria-label="Knowledge type"
              className="grid grid-cols-1 gap-2 sm:grid-cols-2"
              data-testid="knowledge-create-type-grid"
              role="radiogroup"
            >
              {TYPE_OPTIONS.map(option => (
                <RadioCard
                  data-testid={`knowledge-create-type-${option.value}`}
                  description={option.description}
                  icon={option.icon}
                  key={option.value}
                  onSelect={() => setSelectedType(option.value)}
                  selected={type === option.value}
                  title={option.title}
                />
              ))}
            </div>
          </FormSection>
          <FormSection
            description="A stable name and a retrieval-friendly description help agents find it."
            title="The content"
          >
            <div className="flex flex-col gap-4">
              <Field>
                <FieldContent>
                  <FieldLabel htmlFor="knowledge-create-name">
                    Name
                    <Eyebrow className="ml-1.5 text-accent-strong">required</Eyebrow>
                  </FieldLabel>
                </FieldContent>
                <Input
                  className="font-mono"
                  data-testid="knowledge-create-name"
                  id="knowledge-create-name"
                  onChange={event => setName(event.target.value)}
                  placeholder="Canonical knowledge name"
                  value={name}
                />
              </Field>
              <Field>
                <FieldContent>
                  <FieldLabel htmlFor="knowledge-create-description">Description</FieldLabel>
                  <FieldDescription>What should retrieval match?</FieldDescription>
                </FieldContent>
                <Input
                  data-testid="knowledge-create-description"
                  id="knowledge-create-description"
                  onChange={event => setDescription(event.target.value)}
                  placeholder="Optional summary"
                  value={description}
                />
              </Field>
              <Field>
                <FieldContent>
                  <FieldLabel htmlFor="knowledge-create-content">
                    Content
                    <Eyebrow className="ml-1.5 text-accent-strong">required</Eyebrow>
                  </FieldLabel>
                </FieldContent>
                <Textarea
                  className="h-60 font-mono text-small-body"
                  data-testid="knowledge-create-content"
                  id="knowledge-create-content"
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
            data-testid="knowledge-create-dialog-error"
            role="alert"
          >
            {error}
          </div>
        ) : null}
        <EntityDialogFooter
          cancelTestId="cancel-create-memory-btn"
          hint={`Stays scoped to ${scope} — other scopes never retrieve it.`}
          isSaving={isPending}
          onCancel={() => updateDialogOpen(false)}
          onPrimary={handleSubmit}
          primaryDisabled={submitDisabled}
          primaryIcon={Plus}
          primaryLabel="Create entry"
          primaryTestId="confirm-create-memory-btn"
        />
      </DialogContent>
    </Dialog>
  );
}

export { KnowledgeCreateDialog };
export type { KnowledgeCreateDialogProps, KnowledgeCreateInput };
