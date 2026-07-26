import { AlertCircle } from "lucide-react";
import { ReactFlowProvider } from "@xyflow/react";
import { toast } from "sonner";

import { Empty, Skeleton, useTopbarSlot, type TopbarSlotValue } from "@agh/ui";

import { useLoopEditor, type UseLoopEditorResult } from "../../hooks/use-loop-editor";
import type { LoopDefinition, LoopDetail } from "../../types";
import { LoopEditorCanvas } from "./loop-editor-canvas";
import { LoopEditorDslView } from "./loop-editor-dsl-view";
import { LoopEditorPalette } from "./loop-editor-palette";
import { LoopEditorSidebar } from "./loop-editor-sidebar";
import { LoopEditorStartSummary } from "./loop-editor-start-summary";
import { LoopEditorToolbar } from "./loop-editor-toolbar";
import { LoopEditorTopbarActions, LoopEditorTopbarStatus } from "./loop-editor-topbar-actions";
import { LoopLinterDock } from "./loop-linter-dock";

interface LoopEditorProps {
  workspaceId: string;
  name: string;
  topbarIdentity?: Pick<TopbarSlotValue, "crumb" | "crumbs" | "onBack">;
  /** Called after a successful publish with the updated loop (route → toast / navigate to run). */
  onPublished?: (loop: LoopDetail) => void;
}

type ReadyEditor = UseLoopEditorResult & {
  loop: LoopDetail;
  definition: LoopDefinition;
};

/**
 * The fork-and-edit visual editor (task 22): the `@xyflow/react` DAG canvas + node
 * palette + per-node inspector + linter dock + Graph/DSL toggle over the ONE canonical
 * Loop definition. The bijective codec + shared-linter authority live in the view-model
 * (useLoopEditor); this composition wires the surfaces and the publish → run handoff.
 */
export function LoopEditor({ workspaceId, name, topbarIdentity, onPublished }: LoopEditorProps) {
  const editor = useLoopEditor(workspaceId, name);
  const readyEditor: ReadyEditor | null =
    editor.status === "ready" && editor.loop && editor.definition
      ? { ...editor, loop: editor.loop, definition: editor.definition }
      : null;

  const handlePublish = async () => {
    if (!readyEditor) return;
    const updated = await readyEditor.publish();
    if (updated) {
      toast.success(`Published ${updated.name} v${updated.version}`);
      onPublished?.(updated);
    }
  };

  useTopbarSlot({
    ...topbarIdentity,
    status: readyEditor ? (
      <LoopEditorTopbarStatus
        version={readyEditor.version}
        isDirty={readyEditor.isDirty}
        positionsDirty={readyEditor.positionsDirty}
      />
    ) : undefined,
    actions: readyEditor ? (
      <LoopEditorTopbarActions
        busy={readyEditor.busy}
        publishDisabled={readyEditor.publishDisabled}
        onValidate={() => void readyEditor.validate()}
        onPublish={() => void handlePublish()}
      />
    ) : undefined,
  });

  if (editor.status === "no-workspace") {
    return (
      <CenteredState testId="loop-editor-no-workspace">
        <Empty
          className="max-w-md"
          title="No workspace selected"
          description="Select a workspace to edit this Loop."
        />
      </CenteredState>
    );
  }
  if (editor.status === "loading") {
    return <LoopEditorSkeleton />;
  }
  if (!readyEditor) {
    return (
      <CenteredState testId="loop-editor-not-found">
        <div className="flex flex-col items-center gap-2 text-center">
          <AlertCircle className="size-6 text-danger" />
          <p className="text-sm text-muted">{editor.errorMessage || `Loop ${name} not found.`}</p>
        </div>
      </CenteredState>
    );
  }

  return <LoopEditorReady editor={readyEditor} />;
}

function LoopEditorReady({ editor }: { editor: ReadyEditor }) {
  const definition = editor.definition;

  return (
    <ReactFlowProvider>
      <div className="flex min-h-0 flex-1 flex-col gap-0" data-testid="loop-editor">
        <LoopEditorToolbar
          source={editor.loop.source}
          busy={editor.busy}
          positionsDirty={editor.positionsDirty}
          view={editor.view}
          onViewChange={editor.setView}
          onAutoLayout={editor.autoLayout}
          onSaveLayout={() => void editor.savePositions()}
        />

        <div className="grid min-h-0 flex-1 grid-cols-[190px_minmax(0,1fr)_344px]">
          <LoopEditorPalette onAddNode={editor.addNode} disabled={editor.busy} />

          <section className="relative flex min-h-0 flex-col bg-canvas">
            {editor.view === "graph" ? (
              <div className="relative min-h-0 flex-1">
                <LoopEditorStartSummary start={definition.start ?? []} />
                <LoopEditorCanvas
                  nodes={editor.nodes}
                  edges={editor.edges}
                  selectedNodeId={editor.selectedNode?.id ?? null}
                  onNodesChange={editor.onNodesChange}
                  onEdgesChange={editor.onEdgesChange}
                  onConnect={editor.onConnect}
                  onSelectNode={editor.selectNode}
                />
              </div>
            ) : (
              <div className="min-h-0 flex-1 overflow-auto">
                <LoopEditorDslView lines={editor.dslLines} />
              </div>
            )}

            {editor.publishError ? (
              <p
                className="flex items-center gap-2 border-t border-danger/40 bg-danger-tint px-3.5 py-2 text-form-label text-danger"
                data-testid="loop-editor-publish-error"
                role="alert"
              >
                <AlertCircle aria-hidden="true" className="size-3.5" />
                {editor.publishError}
              </p>
            ) : null}

            <LoopLinterDock
              lint={editor.lint}
              validateFailed={editor.validateFailed}
              onReveal={editor.revealNode}
            />
          </section>

          <LoopEditorSidebar
            contract={definition.contract}
            contractDisabled={editor.busy || editor.loop.source !== "workspace"}
            onChangeContract={editor.changeContract}
            node={editor.selectedNode}
            fields={editor.selectedFields}
            nodes={editor.nodes}
            edges={editor.edges}
            selectionKey={editor.selectionSeq}
            definition={definition}
            inspectorDisabled={editor.busy}
            onChangeField={editor.changeField}
            sidebarTab={editor.sidebarTab}
            onSidebarTabChange={editor.setSidebarTab}
          />
        </div>
      </div>
    </ReactFlowProvider>
  );
}

function LoopEditorSkeleton() {
  return (
    <div
      aria-busy="true"
      className="flex min-h-0 flex-1 flex-col"
      data-testid="loop-editor-loading"
      role="status"
    >
      <div className="flex h-10.5 items-center gap-2 border-b border-line px-3.5">
        <Skeleton className="h-7 w-40" />
        <Skeleton className="h-7 w-28" />
        <Skeleton className="h-7 w-32" />
      </div>
      <div className="grid min-h-0 flex-1 grid-cols-[190px_minmax(0,1fr)_344px]">
        <div className="space-y-3 border-r border-line p-3">
          <Skeleton className="h-3 w-20" />
          {[0, 1, 2, 3, 4].map(index => (
            <Skeleton className="h-9 w-full" key={index} />
          ))}
        </div>
        <div className="relative p-8">
          <Skeleton className="absolute top-16 left-16 h-24 w-44" />
          <Skeleton className="absolute top-44 left-72 h-24 w-44" />
          <Skeleton className="absolute top-72 left-28 h-24 w-44" />
        </div>
        <div className="space-y-4 border-l border-line p-4">
          <div className="flex gap-2">
            <Skeleton className="h-7 w-20" />
            <Skeleton className="h-7 w-16" />
          </div>
          <Skeleton className="h-3 w-48" />
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-20 w-full" />
        </div>
      </div>
      <span className="sr-only">Loading Loop editor</span>
    </div>
  );
}

function CenteredState({ testId, children }: { testId: string; children: React.ReactNode }) {
  return (
    <div
      className="flex min-h-0 flex-1 items-center justify-center px-6 py-10"
      data-testid={testId}
    >
      {children}
    </div>
  );
}
