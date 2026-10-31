import {
  DirectoryPickerInput,
  Field,
  FieldDescription,
  FieldError,
  FieldHeader,
  FieldLabel,
  FormSection,
  HelpTip,
  Input,
  RequiredMark,
} from "@compozy/ui";

import type { WorkspaceSetupContent } from "../hooks/use-workspace-setup-content";

interface WorkspaceSetupLocationPaneProps {
  setup: WorkspaceSetupContent;
}

/**
 * Left pane of the split shell: the project folder and the display name.
 *
 * The folder is typed with completion, or chosen in the OS folder dialog when
 * the desktop shell offers one.
 */
export function WorkspaceSetupLocationPane({ setup }: WorkspaceSetupLocationPaneProps) {
  const root = setup.rootField;
  return (
    <div className="flex min-h-0 flex-col gap-4">
      <FormSection className="flex min-h-0 flex-col" title="Location">
        <div className="flex min-h-0 flex-col gap-4">
          <Field data-invalid={root.status === "invalid" || undefined}>
            <FieldHeader>
              <FieldLabel htmlFor="workspace-setup-root">
                Root directory
                <RequiredMark />
              </FieldLabel>
            </FieldHeader>
            <DirectoryPickerInput
              aria-describedby="workspace-setup-root-hint"
              browsePending={root.browsePending}
              data-testid="workspace-setup-root-input"
              disabled={setup.submissionMode !== null}
              id="workspace-setup-root"
              invalid={root.status === "invalid"}
              onBrowse={root.browse}
              onValueChange={root.onValueChange}
              placeholder="~/code/checkout-platform"
              suggestions={root.suggestions}
              value={root.value}
            />
            {root.message ? (
              <FieldError data-testid="workspace-setup-root-error" id="workspace-setup-root-hint">
                {root.message}
              </FieldError>
            ) : (
              <FieldDescription id="workspace-setup-root-hint">
                Sessions read and write inside this folder — it cannot change later.
              </FieldDescription>
            )}
          </Field>

          <Field>
            <FieldHeader>
              <FieldLabel htmlFor="workspace-setup-name">Display name</FieldLabel>
              <HelpTip label="About display name">Defaults to the folder name.</HelpTip>
            </FieldHeader>
            <Input
              data-testid="workspace-setup-name-input"
              disabled={setup.submissionMode !== null}
              id="workspace-setup-name"
              onChange={event => setup.setName(event.target.value)}
              placeholder="Checkout platform"
              value={setup.draft.name}
            />
          </Field>
        </div>
      </FormSection>
    </div>
  );
}
