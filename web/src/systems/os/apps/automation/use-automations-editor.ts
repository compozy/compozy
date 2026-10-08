import { useNavigate } from "@tanstack/react-router";

import {
  automationEditorSeed,
  useAutomationEditor,
  type AutomationsRouteSearch,
} from "@/systems/automation";

import { useAutomationCreateSeed, useAutomationCreateSeedStore } from "./use-automation-page-base";

/** A Cancel or dismiss of the editor is the operator's close: it also clears a `?create=` link. */
function withSeedClose<T extends { editor: { onCancel: () => void } | null }>(
  props: T,
  closeSeed: () => void
): T {
  const { editor } = props;
  if (!editor) return props;
  return {
    ...props,
    editor: {
      ...editor,
      onCancel: () => {
        closeSeed();
        editor.onCancel();
      },
    },
  };
}

type AutomationEditorParams = Parameters<typeof useAutomationEditor>[0];
type AutomationSaved = Parameters<NonNullable<AutomationEditorParams["onSaved"]>>[0];

function detailRoute(saved: AutomationSaved) {
  return saved.entity === "job"
    ? ({ to: "/automations/jobs/$jobId", params: { jobId: saved.automation.id } } as const)
    : ({
        to: "/automations/triggers/$triggerId",
        params: { triggerId: saved.automation.id },
      } as const);
}

/**
 * The listing's one editor: "New automation", row Edit and the `?create=` deep
 * link. A save lands on the new automation's detail page.
 */
export function useAutomationsEditor({
  search,
  activeWorkspaceId,
  workspaces,
  workspaceResolved,
  startView,
}: {
  search: AutomationsRouteSearch;
  activeWorkspaceId: string | null | undefined;
  workspaces: AutomationEditorParams["workspaces"];
  workspaceResolved: boolean;
  startView: "schedule" | "event" | null;
}) {
  const navigate = useNavigate();
  const seedStore = useAutomationCreateSeedStore();
  const editor = useAutomationEditor({
    activeWorkspaceId,
    workspaces,
    onSaved: saved => {
      seedStore.trigger.saved();
      void navigate(detailRoute(saved));
    },
  });
  const closeSeed = useAutomationCreateSeed(
    seedStore,
    automationEditorSeed(search),
    { activeWorkspaceId, editorOpen: editor.editor !== null, resolved: workspaceResolved },
    seed => editor.openCreate({ loop: seed.loop, start: seed.start })
  );

  /** "New automation": the given start, else the current Start view, else a schedule. */
  const create = (start?: "schedule" | "event" | "webhook" | null) =>
    editor.openCreate({ start: (start === undefined ? startView : start) ?? "schedule" });

  return {
    create,
    editorDialogProps: withSeedClose(editor.editorDialogProps, closeSeed),
    openEdit: editor.openEdit,
  };
}
