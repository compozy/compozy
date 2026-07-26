import { useQuery } from "@tanstack/react-query";
import { AlertCircle, Download, Upload } from "lucide-react";
import { useRef, type ReactNode } from "react";

import { Button, Spinner } from "@agh/ui";

import { windowManagerConfigOptions } from "@/systems/os";
import {
  LayoutProfileGrid,
  LayoutStage,
  SettingsGroup,
  SettingsPageFrame,
  SettingsSaveBar,
  useSettingsSaveBarState,
  useSettingsTopbar,
  WindowManagerConfigEditor,
  useWindowManagerConfigEditor,
  useWindowManagerLayoutEditor,
  useWindowManagerLayoutProfiles,
  windowManagerLayoutOptions,
  windowManagerLayoutProfilesOptions,
} from "@/systems/settings";
import type {
  WindowManagerLayoutResourceRecord,
  WindowManagerLayoutState,
} from "@/systems/settings";
import type { WindowManagerConfig } from "@/systems/os";
import { useActiveWorkspace } from "@/systems/workspace";

function LoadingState() {
  return (
    <div
      className="flex flex-1 items-center justify-center"
      data-testid="settings-page-layouts-loading"
    >
      <Spinner className="size-5 text-subtle" />
    </div>
  );
}

function ErrorState({ error, onRetry }: { error: Error; onRetry: () => void }) {
  return (
    <div
      className="flex flex-1 items-center justify-center"
      data-testid="settings-page-layouts-error"
    >
      <div className="flex max-w-sm flex-col items-center gap-2 text-center">
        <AlertCircle aria-hidden="true" className="size-6 text-danger" />
        <p className="text-small-body text-subtle">{error.message}</p>
        <Button size="cta-lg" type="button" variant="outline" onClick={onRetry}>
          Retry
        </Button>
      </div>
    </div>
  );
}

/**
 * The workspace layout and the layouts saved from it. Both read one draft, so a
 * saved layout captures exactly what is on the canvas, and loading one asks
 * before it discards unapplied edits.
 */
function LayoutSections({
  config,
  initial,
  profiles,
  workspaceId,
}: {
  config: WindowManagerConfig;
  initial: WindowManagerLayoutState;
  profiles: readonly WindowManagerLayoutResourceRecord[];
  workspaceId: string;
}) {
  const fileInput = useRef<HTMLInputElement>(null);
  const editor = useWindowManagerLayoutEditor(workspaceId, initial);
  const savedLayouts = useWindowManagerLayoutProfiles({
    workspaceId,
    document: editor.draft,
    profiles,
    draftDirty: editor.dirty,
    onLoad: editor.updateDraft,
  });

  const exportDocument = () => {
    const blob = new Blob([JSON.stringify(editor.exportDocument(), null, 2)], {
      type: "application/json",
    });
    const url = URL.createObjectURL(blob);
    const anchor = window.document.createElement("a");
    anchor.href = url;
    anchor.download = `layout-${workspaceId}.json`;
    window.document.body.append(anchor);
    anchor.click();
    anchor.remove();
    URL.revokeObjectURL(url);
  };

  return (
    <>
      <SettingsGroup
        action={
          <>
            <input
              accept="application/json,.json"
              className="hidden"
              data-testid="layout-import-input"
              ref={fileInput}
              type="file"
              onChange={event => {
                const file = event.target.files?.[0];
                event.target.value = "";
                if (file) void editor.importDocument(file);
              }}
            />
            <Button
              data-testid="layout-import"
              size="sm"
              type="button"
              variant="outline"
              onClick={() => fileInput.current?.click()}
            >
              <Upload aria-hidden="true" className="size-3.5" />
              Import JSON
            </Button>
            <Button
              data-testid="layout-export"
              size="sm"
              type="button"
              variant="outline"
              onClick={exportDocument}
            >
              <Download aria-hidden="true" className="size-3.5" />
              Export JSON
            </Button>
          </>
        }
        bare
        description="The arrangement this workspace restores. Drag tiles, dividers and group edges — the daemon checks the result before anything is applied."
        title="Workspace layout"
      >
        <LayoutStage config={config} editor={editor} />
      </SettingsGroup>

      <SettingsGroup
        bare
        description="Named snapshots of an arrangement. Load one into the editor, then review and apply it like any other edit."
        title="Saved layouts"
      >
        <LayoutProfileGrid document={editor.draft} editor={savedLayouts} />
      </SettingsGroup>
    </>
  );
}

/**
 * The loaded page. Split from the route component so the config editor's hooks
 * sit above the frame — the one floating save bar on this page belongs to the
 * global config, and the layout document reviews inside its own card.
 */
function LayoutsSettingsView({
  config,
  layout,
  meta,
  profiles,
  workspaceId,
}: {
  config: WindowManagerConfig;
  layout: WindowManagerLayoutState | null;
  meta: ReadonlyArray<{ key: string; content: ReactNode }>;
  profiles: readonly WindowManagerLayoutResourceRecord[];
  workspaceId: string;
}) {
  const configEditor = useWindowManagerConfigEditor(config);
  const saveBarState = useSettingsSaveBarState({
    isDirty: configEditor.dirty,
    isInvalid: configEditor.problems.length > 0,
    isSaving: configEditor.isSaving,
    error: configEditor.error instanceof Error ? configEditor.error.message : null,
    warnings: configEditor.problems.map(problem => problem.message),
  });

  return (
    <SettingsPageFrame
      description="Shape how windows tile, snap and move — drag the layout instead of typing coordinates."
      meta={meta}
      saveBar={
        <SettingsSaveBar
          slug="layouts"
          state={saveBarState}
          onReset={configEditor.reset}
          onSave={configEditor.save}
        />
      }
      slug="layouts"
      width="canvas"
    >
      {workspaceId === "" || layout === null ? (
        <SettingsGroup description="A layout belongs to a workspace." title="Workspace layout">
          <p className="px-4 py-5 text-form-label text-subtle">
            Select a workspace to see and change its layout.
          </p>
        </SettingsGroup>
      ) : (
        <LayoutSections
          config={config}
          initial={layout}
          key={workspaceId}
          profiles={profiles}
          workspaceId={workspaceId}
        />
      )}
      <WindowManagerConfigEditor editor={configEditor} />
    </SettingsPageFrame>
  );
}

/** Global window-manager defaults plus the active workspace's authoritative layout. */
export function LayoutsSettingsPage() {
  useSettingsTopbar("layouts");
  const workspace = useActiveWorkspace();
  const workspaceId = workspace.activeWorkspaceId ?? "";
  const settings = useQuery(windowManagerConfigOptions());
  const profiles = useQuery(windowManagerLayoutProfilesOptions(workspaceId));
  const layout = useQuery(windowManagerLayoutOptions(workspaceId));
  const waitingForWorkspace = !workspace.hasHydrated || workspace.isLoading;
  const isPending =
    settings.isPending ||
    (workspaceId !== "" && profiles.isPending) ||
    waitingForWorkspace ||
    (workspaceId !== "" && layout.isPending);
  const error =
    settings.error instanceof Error
      ? settings.error
      : profiles.error instanceof Error
        ? profiles.error
        : workspace.error instanceof Error
          ? workspace.error
          : layout.error instanceof Error
            ? layout.error
            : null;

  if (isPending) return <LoadingState />;
  if (error !== null) {
    return (
      <ErrorState
        error={error}
        onRetry={() => {
          void Promise.all([
            settings.refetch(),
            workspace.refetch(),
            ...(workspaceId === "" ? [] : [profiles.refetch(), layout.refetch()]),
          ]);
        }}
      />
    );
  }
  if (!settings.data) return <LoadingState />;

  const profileRecords = profiles.data ?? [];
  const activeWorkspaceName = workspace.activeWorkspace?.name ?? null;
  const meta = [
    activeWorkspaceName ? { key: "workspace", content: <span>{activeWorkspaceName}</span> } : null,
    layout.data ? { key: "revision", content: <span>Revision {layout.data.revision}</span> } : null,
    {
      key: "profiles",
      content: (
        <span>
          {profileRecords.length} saved layout{profileRecords.length === 1 ? "" : "s"}
        </span>
      ),
    },
  ].filter((entry): entry is NonNullable<typeof entry> => entry !== null);

  return (
    <LayoutsSettingsView
      config={settings.data}
      layout={layout.data ?? null}
      meta={meta}
      profiles={profileRecords}
      workspaceId={workspaceId}
    />
  );
}
