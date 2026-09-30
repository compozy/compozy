import { Download, Upload } from "lucide-react";
import { useRef, type ReactNode } from "react";

import { Button } from "@compozy/ui";

import {
  windowManagerApplyMessage,
  type WindowManagerConfig,
  type WindowManagerSettingsSection,
} from "@/systems/os";
import {
  LayoutProfileGrid,
  LayoutStage,
  SettingsGroup,
  SettingsPageFrame,
  SettingsPageState,
  SettingsSaveBar,
  useLayoutsSettingsData,
  useSettingsSaveBarState,
  useSettingsTopbar,
  useWindowManagerConfigEditor,
  useWindowManagerKeyboardEditors,
  useWindowManagerLayoutEditor,
  useWindowManagerLayoutProfiles,
  WindowManagerConfigEditor,
  type WindowManagerLayoutResourceRecord,
  type WindowManagerLayoutState,
} from "@/systems/settings";

/**
 * The workspace layout and the layouts saved from it. Both read one draft, so a
 * saved layout captures exactly what is on the canvas, and loading one asks
 * before it discards unapplied edits.
 */
function LayoutSections({
  config,
  initial,
  profileName,
  profiles,
  workspaceId,
}: {
  config: WindowManagerConfig;
  initial: WindowManagerLayoutState;
  profileName: string;
  profiles: readonly WindowManagerLayoutResourceRecord[];
  workspaceId: string;
}) {
  const fileInput = useRef<HTMLInputElement>(null);
  const editor = useWindowManagerLayoutEditor(workspaceId, profileName, initial);
  const savedLayouts = useWindowManagerLayoutProfiles({
    workspaceId,
    profileId: profileName,
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
              variant="secondary"
              onClick={() => fileInput.current?.click()}
            >
              <Upload aria-hidden="true" className="size-3.5" />
              Import layout file
            </Button>
            <Button
              data-testid="layout-export"
              size="sm"
              type="button"
              variant="secondary"
              onClick={exportDocument}
            >
              <Download aria-hidden="true" className="size-3.5" />
              Export layout file
            </Button>
          </>
        }
        bare
        help="Drag tiles, dividers and group edges."
        description="CompozyOS checks the result before anything is applied."
        title="Project layout"
      >
        <LayoutStage config={config} editor={editor} />
      </SettingsGroup>

      <SettingsGroup
        bare
        help="Load one into the editor, then review and apply it like any other edit."
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
  clientId,
  config,
  focusCommandId,
  layout,
  meta,
  profileName,
  profiles,
  section,
  workspaceId,
}: {
  clientId: string | undefined;
  config: WindowManagerConfig;
  focusCommandId?: string;
  layout: WindowManagerLayoutState | null;
  meta: ReadonlyArray<{ key: string; content: ReactNode }>;
  profileName: string;
  profiles: readonly WindowManagerLayoutResourceRecord[];
  section: WindowManagerSettingsSection;
  workspaceId: string;
}) {
  const configEditor = useWindowManagerConfigEditor(config);
  // Keyboard state is daemon-owned and applies live, so it writes through its
  // own path rather than joining the page's draft (US-022.AC-3).
  const { aliases, globalRecorder, recorder, bindingApply } = useWindowManagerKeyboardEditors(
    section,
    workspaceId,
    clientId
  );
  const saveBarState = useSettingsSaveBarState({
    isDirty: configEditor.dirty || configEditor.error !== null,
    isInvalid: configEditor.problems.length > 0,
    isSaving: configEditor.phase === "saving",
    error: configEditor.error instanceof Error ? configEditor.error.message : null,
    warnings: [
      ...configEditor.problems.map(problem => problem.message),
      ...(configEditor.result?.apply.warnings ?? []),
      ...(configEditor.result &&
      (configEditor.result.apply.next_action !== "none" ||
        (!configEditor.result.apply.applied && !configEditor.result.apply.skipped))
        ? [windowManagerApplyMessage(configEditor.result.apply)]
        : []),
    ],
    lastAppliedLabel: configEditor.result
      ? windowManagerApplyMessage(configEditor.result.apply)
      : null,
  });

  return (
    <SettingsPageFrame
      description="The layout applies from its own Apply button, window settings wait for Save, and shortcuts apply right away."
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
        <SettingsGroup title="Project layout">
          <p className="px-4 py-5 text-form-label text-subtle">
            Open a project to see and change its layout.
          </p>
        </SettingsGroup>
      ) : (
        <LayoutSections
          config={config}
          initial={layout}
          key={`${workspaceId}:${profileName}`}
          profileName={profileName}
          profiles={profiles}
          workspaceId={workspaceId}
        />
      )}
      <WindowManagerConfigEditor
        aliases={aliases}
        bindingApply={bindingApply}
        editor={configEditor}
        focusCommandId={focusCommandId}
        globalRecorder={globalRecorder}
        recorder={recorder}
        section={section}
      />
    </SettingsPageFrame>
  );
}

/** Global window-manager defaults plus the active workspace's authoritative layout. */
export function LayoutsSettingsPage({ focusCommandId }: { focusCommandId?: string }) {
  useSettingsTopbar();
  const data = useLayoutsSettingsData();

  if (data.isPending) return <SettingsPageState slug="layouts" state="loading" />;
  if (data.error !== null) {
    return (
      <SettingsPageState error={data.error} onRetry={data.retry} slug="layouts" state="error" />
    );
  }
  if (data.config === null || data.keyboard === null) {
    return <SettingsPageState slug="layouts" state="loading" />;
  }

  const meta = [
    data.workspaceName ? { key: "workspace", content: <span>{data.workspaceName}</span> } : null,
    {
      key: "profiles",
      content: (
        <span>
          {data.profiles.length} saved layout{data.profiles.length === 1 ? "" : "s"}
        </span>
      ),
    },
  ].filter((entry): entry is NonNullable<typeof entry> => entry !== null);

  return (
    <LayoutsSettingsView
      clientId={data.clientId}
      config={data.config}
      focusCommandId={focusCommandId}
      layout={data.layout}
      meta={meta}
      profileName={data.profileName}
      profiles={data.profiles}
      section={data.keyboard}
      workspaceId={data.workspaceId}
    />
  );
}
