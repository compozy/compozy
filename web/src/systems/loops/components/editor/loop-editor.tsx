import { AlertCircle } from "@compozy/ui/icons";
import { ReactFlowProvider } from "@xyflow/react";

import { cn, Empty, useTopbarSlot, type TopbarSlotValue } from "@compozy/ui";

import { useWorktrees, type WorktreePayload } from "@/systems/workspace";

import { useLoopEditor, type UseLoopEditorResult } from "../../hooks/use-loop-editor";
import { useLoopEditorReadyActions } from "../../hooks/use-loop-editor-ready-actions";
import { useLoopFork } from "../../hooks/use-loop-fork";
import { useLoopConfig } from "../../hooks/use-loops";
import type { LoopDefinition, LoopDetail, LoopEnvironmentSpec } from "../../types";
import { LoopEditorCanvas } from "./loop-editor-canvas";
import { LoopEditorConnectionPicker } from "./loop-editor-connection-picker";
import { LoopEditorDslView } from "./loop-editor-dsl-view";
import { editorGridClass } from "./loop-editor-grid";
import { LoopEditorNodeActionsProvider } from "./loop-editor-node";
import { LoopEditorPalette } from "./loop-editor-palette";
import { LoopEditorPublishRejectedStrip } from "./loop-editor-publish-rejected-strip";
import { LoopEditorQuickAdd } from "./loop-editor-quick-add";
import { LoopEditorSidebar } from "./loop-editor-sidebar";
import { LoopEditorSkeleton } from "./loop-editor-skeleton";
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
          <p className="text-small-body text-muted">
            {editor.errorMessage || `Loop ${name} not found.`}
          </p>
        </div>
      </CenteredState>
    );
  }

  return (
    <ReactFlowProvider>
      <LoopEditorReady
        editor={readyEditor}
        fork={forkState}
        gitBacked={worktrees.data?.repo.git_backed !== false}
        loopDefaultEnvironment={config.data?.environment ?? undefined}
        worktrees={worktrees.data?.worktrees ?? []}
      />
    </ReactFlowProvider>
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
  const { addNode, chrome, editorRoot, nodeActions, overlays, readOnly, revealNode, selectNode } =
    useLoopEditorReadyActions(editor);

  return (
    <LoopEditorNodeActionsProvider actions={nodeActions}>
      <div
        className="flex min-h-0 flex-1 flex-col gap-0"
        data-testid="loop-editor"
        ref={editorRoot}
      >
        <LoopEditorToolbar
          addNodeDisabled={editor.saving || readOnly}
          busy={editor.busy}
          onAddNode={addNode}
          positionsDirty={editor.positionsDirty}
          view={editor.view}
          onViewChange={editor.selectView}
          onAutoLayout={editor.autoLayout}
          onSaveLayout={() => void editor.savePositions()}
          paletteMode={chrome.paletteMode}
          onTogglePalette={chrome.togglePalette}
          inspectorOpen={chrome.inspectorOpen}
          onToggleInspector={chrome.toggleInspector}
        />

        {readOnly ? (
          <LoopEditorSourceStrip
            source={editor.loop.source}
            forking={fork.forking}
            onFork={fork.fork}
          />
        ) : null}

        <div
          className={cn(
            "grid min-h-0 flex-1",
            editorGridClass(chrome.paletteMode, chrome.inspectorOpen)
          )}
        >
          {chrome.paletteMode === "expanded" ? (
            <LoopEditorPalette onAddNode={addNode} disabled={editor.saving || readOnly} />
          ) : null}

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
                  onNodesDelete={editor.onNodesDelete}
                  onConnectDropped={overlays.openConnectionDrop}
                  onQuickAdd={overlays.openQuickAdd}
                  onSelectNode={selectNode}
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
              onReveal={revealNode}
              collapsed={chrome.dockCollapsed}
              onToggleCollapsed={chrome.toggleDock}
            />
          </section>

          {chrome.inspectorOpen ? (
            <LoopEditorSidebar
              contract={definition.contract}
              contractDisabled={editor.saving || readOnly}
              gitBacked={gitBacked}
              inspectorDisabled={editor.saving || readOnly}
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
          ) : null}
        </div>
      </div>

      <LoopEditorQuickAdd
        nodes={editor.nodes.map(node => ({
          id: node.id,
          kind: node.data.kind,
          label: node.id,
        }))}
        onAddNode={item => addNode(item, overlays.quickAdd?.position ?? undefined)}
        onOpenChange={overlays.setQuickAddOpen}
        onRevealNode={revealNode}
        open={overlays.quickAdd !== null}
        readOnly={readOnly}
      />

      <LoopEditorConnectionPicker
        onOpenChange={open => {
          if (!open) overlays.closeConnectionDrop();
        }}
        onPick={item => {
          const drop = overlays.connectionDrop;
          if (drop) {
            editor.addNodeWithEdge(item, drop.position, drop.source);
            chrome.openInspector();
          }
          overlays.closeConnectionDrop();
        }}
        open={overlays.connectionDrop !== null}
        point={overlays.connectionDrop?.point ?? null}
        sourceNodeId={overlays.connectionDrop?.source ?? ""}
      />
    </LoopEditorNodeActionsProvider>
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
