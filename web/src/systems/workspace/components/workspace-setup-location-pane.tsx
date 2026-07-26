import { Home, Sparkles } from "lucide-react";

import {
  Button,
  Eyebrow,
  Field,
  FieldContent,
  FieldDescription,
  FieldLabel,
  FormSection,
  ImmutableIdentity,
  Input,
  Pill,
  Spinner,
} from "@agh/ui";

import { DirectoryBrowser } from "@/systems/onboarding";

import { WORKSPACE_SETUP_COPY } from "../lib/workspace-setup-copy";
import type { WorkspaceSetupContent } from "../hooks/use-workspace-setup-content";
import { OptionCard } from "./option-card";

interface WorkspaceSetupLocationPaneProps {
  setup: WorkspaceSetupContent;
  size: "comfortable" | "compact";
}

/**
 * Left pane of the split shell: the global-default shortcut, the filesystem
 * browser that chooses the root, and the display name.
 *
 * The root is picked in the browser — a plain path input is forbidden for root
 * selection (`MODAL-STANDARD.md` § Component grammar).
 */
export function WorkspaceSetupLocationPane({ setup, size }: WorkspaceSetupLocationPaneProps) {
  const globalMeta = setup.userHomeDir || setup.globalUnavailableReason;
  const isGlobalDisabled = Boolean(setup.globalUnavailableReason) || setup.submissionMode !== null;
  const isGlobalPending = setup.submissionMode === "global";

  return (
    <div className="flex min-h-0 flex-col gap-4">
      {/* Production-authorized addition the design does not cover: the one-click
          global-default shortcut stays above Location (`_techspec.md` §4.3). */}
      <OptionCard data-testid="workspace-setup-global-card" size={size}>
        <OptionCard.Header
          eyebrow="Global"
          right={<Pill tone="success">{WORKSPACE_SETUP_COPY.global.badge}</Pill>}
        />
        <OptionCard.Body>
          <OptionCard.Icon tone="neutral">
            <Home className="size-4" />
          </OptionCard.Icon>
          <OptionCard.Content>
            <OptionCard.Title>{WORKSPACE_SETUP_COPY.global.title}</OptionCard.Title>
            <OptionCard.Description>
              {WORKSPACE_SETUP_COPY.global.description}
            </OptionCard.Description>
            {globalMeta ? (
              <OptionCard.Meta data-testid="workspace-global-meta">{globalMeta}</OptionCard.Meta>
            ) : null}
          </OptionCard.Content>
        </OptionCard.Body>
        <OptionCard.Action>
          {/* Outline, not accent: the footer "Add workspace" is the single accent
              target on this surface, and a solid shortcut here would compete with it. */}
          <Button
            className="w-full justify-between"
            data-testid="workspace-use-global"
            disabled={isGlobalDisabled}
            onClick={() => void setup.handleUseGlobalWorkspace()}
            variant="outline"
          >
            <span>{WORKSPACE_SETUP_COPY.global.action}</span>
            {isGlobalPending ? <Spinner /> : <Sparkles />}
          </Button>
        </OptionCard.Action>
      </OptionCard>

      <FormSection
        className="flex min-h-0 flex-col"
        description="Browse the daemon's filesystem and pick the folder sessions run in."
        title="Location"
      >
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
              testIdPrefix="workspace-setup-browser"
            />
          </div>

          {setup.draft.rootDir ? (
            <ImmutableIdentity
              data-testid="workspace-setup-selected-root"
              hint="Sessions read and write inside this root — it cannot change later."
              rows={[{ label: "Selected root", value: setup.draft.rootDir, mono: true }]}
            />
          ) : (
            <p className="text-form-hint text-subtle" data-testid="workspace-setup-root-empty">
              Pick the workspace root in the browser above.
            </p>
          )}

          <Field>
            <FieldContent>
              <FieldLabel htmlFor="workspace-setup-name">Display name</FieldLabel>
              <FieldDescription>Defaults to the folder name.</FieldDescription>
            </FieldContent>
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
