import {
  cn,
  EntityDialogBody,
  EntityDialogFooter,
  Eyebrow,
  Field,
  FieldHeader,
  FieldLabel,
  FormSection,
  Skeleton,
  SkeletonRegion,
  SkeletonText,
} from "@compozy/ui";

/** The four priority segments, as wide as their labels and the dot before each. */
const PRIORITY_SEGMENTS = [
  { id: "low", width: "w-14" },
  { id: "medium", width: "w-20" },
  { id: "high", width: "w-14" },
  { id: "urgent", width: "w-18" },
] as const;

function SegmentsSkeleton({ widths }: { widths: ReadonlyArray<{ id: string; width: string }> }) {
  return (
    <div className="flex flex-wrap items-center gap-1">
      {widths.map(segment => (
        <Skeleton
          className={cn("h-(--height-pill-group-segment-sm) rounded-pill", segment.width)}
          key={segment.id}
        />
      ))}
    </div>
  );
}

const ATTEMPT_SEGMENTS = [
  { id: "1", width: "w-8" },
  { id: "2", width: "w-8" },
  { id: "3", width: "w-8" },
  { id: "5", width: "w-8" },
] as const;

const APPROVAL_SEGMENTS = [
  { id: "none", width: "w-20" },
  { id: "manual", width: "w-32" },
] as const;

/**
 * The edit form while its task loads: the real section titles and field labels, with each
 * control a block of the height of the control it stands in for, then the footer with its
 * save action held. The header stays the modal's own, so the form arrives into the same dialog.
 * The first section sheds the rule and top padding a following one carries, because the
 * region's visually hidden label is its first child.
 */
export function TaskEditorLoading({ onCancel }: { onCancel: () => void }) {
  return (
    <>
      <SkeletonRegion
        className="flex min-h-0 flex-1 flex-col"
        data-testid="task-editor-modal-loading"
        label="Loading task"
      >
        <EntityDialogBody aria-hidden="true">
          <FormSection className="mt-0 border-t-0 pt-0" title="What needs doing">
            <div className="flex flex-col gap-4">
              <Field>
                <div className="flex items-center justify-between gap-3">
                  <FieldLabel>Title</FieldLabel>
                  <Eyebrow className="text-subtle">Required</Eyebrow>
                </div>
                <Skeleton className="h-input rounded-md" />
              </Field>
              <Field>
                <FieldLabel>Description</FieldLabel>
                <Skeleton className="h-textarea-min rounded-md" />
              </Field>
            </div>
          </FormSection>

          <FormSection title="Priority">
            <SegmentsSkeleton widths={PRIORITY_SEGMENTS} />
          </FormSection>

          <FormSection title="Who works on it">
            <div className="flex flex-col gap-4">
              <Field>
                <FieldHeader>
                  <FieldLabel>Owner</FieldLabel>
                </FieldHeader>
                <Skeleton className="h-input rounded-md" />
                <Skeleton className="mt-2 h-input rounded-md" />
              </Field>
              <div className="grid gap-4 md:grid-cols-2">
                <Field>
                  <FieldLabel>Max attempts</FieldLabel>
                  <SegmentsSkeleton widths={ATTEMPT_SEGMENTS} />
                </Field>
                <Field>
                  <FieldLabel>Approval</FieldLabel>
                  <SegmentsSkeleton widths={APPROVAL_SEGMENTS} />
                </Field>
              </div>
              <p className="text-form-hint text-subtle">
                <SkeletonText className="w-3/4" />
              </p>
            </div>
          </FormSection>
        </EntityDialogBody>
      </SkeletonRegion>

      <EntityDialogFooter
        cancelTestId="task-editor-modal-cancel"
        onCancel={onCancel}
        primaryDisabled
        primaryLabel="Save changes"
        primaryTestId="task-editor-modal-submit"
      />
    </>
  );
}
