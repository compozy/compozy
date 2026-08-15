import { AlertCircle } from "lucide-react";
import { ReactFlowProvider } from "@xyflow/react";

import { Empty, Skeleton, useTopbarSlot, type TopbarSlotValue } from "@compozy/ui";

import { useWorktrees, type WorktreePayload } from "@/systems/workspace";

import { useLoopEditor, type UseLoopEditorResult } from "../../hooks/use-loop-editor";
import { useLoopFork } from "../../hooks/use-loop-fork";
import { useLoopConfig } from "../../hooks/use-loops";
import type { LoopDefinition, LoopDetail, LoopEnvironmentSpec } from "../../types";
import { LoopEditorCanvas } from "./loop-editor-canvas";
import { LoopEditorDslView } from "./loop-editor-dsl-view";
import { LoopEditorPalette } from "./loop-editor-palette";
import { LoopEditorPublishRejectedStrip } from "./loop-editor-publish-rejected-strip";
import { LoopEditorSidebar } from "./loop-editor-sidebar";
import { LoopEditorSourceStrip } from "./loop-editor-source-strip";
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
  liveDataEnabled?: boolean;
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
export function LoopEditor({
  workspaceId,
  name,
  topbarIdentity,
  onPublished,
  liveDataEnabled = true,
}: LoopEditorProps) {
  const editor = useLoopEditor(workspaceId, name, onPublished, liveDataEnabled);
  const config = useLoopConfig(workspaceId, name, Boolean(workspaceId) && liveDataEnabled);
  const forkState = useLoopFork(workspaceId, name);
  const worktrees = useWorktrees(workspaceId, {
    enabled: Boolean(workspaceId) && liveDataEnabled,
  });
  const readyEditor: ReadyEditor | null =
    editor.status === "ready" && editor.loop && editor.definition
      ? { ...editor, loop: editor.loop, definition: editor.definition }
      : null;

  useTopbarSlot({
    ...topbarIdentity,
    status: readyEditor ? (
      <LoopEditorTopbarStatus
        version={readyEditor.version}
        isDirty={readyEditor.isDirty}
        positionsDirty={readyEditor.positionsDirty}
        source={readyEditor.loop.source}
      />
    ) : undefined,
    actions: readyEditor ? (
      <LoopEditorTopbarActions
        busy={readyEditor.busy}
        publishDisabled={readyEditor.publishDisabled}
        onValidate={() => void readyEditor.validate()}
        onPublish={readyEditor.publish}
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

  return (
    <LoopEditorReady
      editor={readyEditor}
      fork={forkState}
      gitBacked={worktrees.data?.repo.git_backed !== false}
      loopDefaultEnvironment={config.data?.environment ?? undefined}
      worktrees={worktrees.data?.worktrees ?? []}
    />
  );
}

function LoopEditorReady({
  editor,
  fork,
  loopDefaultEnvironment,
  worktrees,
  gitBacked,
}: {
  editor: ReadyEditor;
  fork: { fork: () => void; forking: boolean };
  loopDefaultEnvironment?: LoopEnvironmentSpec;
  worktrees: WorktreePayload[];
  gitBacked: boolean;
}) {
  const definition = editor.definition;
  const readOnly = !editor.definitionEditable;

  return (
    <ReactFlowProvider>
      <div className="flex min-h-0 flex-1 flex-col gap-0" data-testid="loop-editor">
        <LoopEditorToolbar
          addNodeDisabled={editor.busy || readOnly}
          busy={editor.busy}
          onAddNode={editor.addNode}
          positionsDirty={editor.positionsDirty}
          view={editor.view}
          onViewChange={editor.selectView}
          onAutoLayout={editor.autoLayout}
          onSaveLayout={() => void editor.savePositions()}
        />

        {readOnly ? (
          <LoopEditorSourceStrip
            source={editor.loop.source}
            forking={fork.forking}
            onFork={fork.fork}
          />
        ) : null}

        <div className="grid min-h-0 flex-1 grid-cols-[minmax(0,1fr)_320px] lg:grid-cols-[170px_minmax(0,1fr)_320px] xl:grid-cols-[190px_minmax(0,1fr)_344px]">
          <LoopEditorPalette onAddNode={editor.addNode} disabled={editor.busy || readOnly} />

          <section className="relative flex min-h-0 flex-col bg-canvas">
            {editor.view === "graph" ? (
              <div className="relative min-h-0 flex-1">
                <LoopEditorStartSummary start={definition.start ?? []} />
                <LoopEditorCanvas
                  nodes={editor.nodes}
                  edges={editor.edges}
                  selectedNodeId={editor.selectedNode?.id ?? null}
                  revealSeq={editor.revealSeq}
                  onNodesChange={editor.onNodesChange}
                  onEdgesChange={editor.onEdgesChange}
                  onConnect={editor.onConnect}
                  onSelectNode={editor.selectNode}
                  readOnly={readOnly}
                  loopDefaultEnvironment={loopDefaultEnvironment}
                />
              </div>
            ) : (
              <div className="min-h-0 flex-1 overflow-auto">
                <LoopEditorDslView lines={editor.dslLines} />
              </div>
            )}

            {editor.publishError ? (
              <LoopEditorPublishRejectedStrip
                message={editor.publishError}
                issues={editor.publishRejectedIssues}
                version={editor.version}
                dockStale={editor.publishRejectedDockStale}
                failureKind={editor.publishFailureKind}
              />
            ) : null}

            <LoopLinterDock
              lint={editor.lint}
              validateFailed={editor.validateFailed}
              onReveal={editor.revealNode}
            />
          </section>

          <LoopEditorSidebar
            contract={definition.contract}
            contractDisabled={editor.busy || readOnly}
            gitBacked={gitBacked}
            inspectorDisabled={editor.busy || readOnly}
            lintByNode={editor.lint.byNode}
            loopDefaultEnvironment={loopDefaultEnvironment}
            node={editor.selectedNode}
            fields={editor.selectedFields}
            nodes={editor.nodes}
            edges={editor.edges}
            onChangeContract={editor.changeContract}
            onChangeContractPath={editor.changeContractPath}
            onChangeField={editor.changeField}
            onChangeFields={editor.changeFields}
            onSidebarTabChange={editor.selectSidebarTab}
            selectionKey={editor.selectionSeq}
            definition={definition}
            sidebarTab={editor.sidebarTab}
            worktrees={worktrees}
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
      <div className="grid min-h-0 flex-1 grid-cols-[minmax(0,1fr)_320px] lg:grid-cols-[170px_minmax(0,1fr)_320px] xl:grid-cols-[190px_minmax(0,1fr)_344px]">
        <div className="hidden space-y-3 border-r border-line p-3 lg:block">
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
