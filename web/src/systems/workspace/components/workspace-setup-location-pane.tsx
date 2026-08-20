import {
  Eyebrow,
  Field,
  FieldHeader,
  FieldLabel,
  FormSection,
  HelpTip,
  ImmutableIdentity,
  Input,
} from "@compozy/ui";

import type { WorkspaceSetupContent } from "../hooks/use-workspace-setup-content";
import { DirectoryBrowser } from "@/systems/onboarding";

interface WorkspaceSetupLocationPaneProps {
  setup: WorkspaceSetupContent;
}

/**
 * Left pane of the split shell: the filesystem browser that chooses the root,
 * and the display name.
 *
 * The root is picked in the browser — a plain path input is forbidden for root
 * selection (`MODAL-STANDARD.md` § Component grammar).
 */
export function WorkspaceSetupLocationPane({ setup }: WorkspaceSetupLocationPaneProps) {
  return (
    <div className="flex min-h-0 flex-col gap-4">
      <FormSection className="flex min-h-0 flex-col" title="Location">
        <div className="flex min-h-0 flex-col gap-4">
          <div className="flex min-h-0 flex-col gap-1.5">
            <span className="flex items-center text-form-label text-fg">
              Root directory
              <Eyebrow className="ml-1.5 text-accent-strong">required</Eyebrow>
            </span>
            <DirectoryBrowser
              browseError={setup.browse.browseError}
              currentPath={setup.browse.currentPath}
              entries={setup.browse.entries}
              homePath={setup.browse.homePath}
              isBrowsing={setup.browse.isBrowsing}
              isPicked={path => path === setup.draft.rootDir}
              onGoHome={setup.browse.goHome}
              onGoParent={setup.browse.goToParent}
              onNavigate={setup.browse.navigateTo}
              onPick={setup.selectRoot}
              parentPath={setup.browse.parentPath}
              pickRowLabel={name => `Use ${name} as the workspace root`}
              roots={setup.browse.roots}
              testIdPrefix="workspace-setup-browser"
            />
          </div>

          {setup.draft.rootDir ? (
            <ImmutableIdentity
              data-testid="workspace-setup-selected-root"
              hint="Sessions read and write inside this root — it cannot change later."
              rows={[{ label: "Selected root", value: setup.draft.rootDir, mono: true }]}
            />
          ) : null}

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
