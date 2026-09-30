import { createContext, use, type ReactNode } from "react";
import { AlertTriangle } from "lucide-react";
import { Handle, Position, type NodeProps } from "@xyflow/react";

import { cn, KindIcon, Pill, PropertyRow, type KindIconRegistry } from "@compozy/ui";

import type { EditorNode, EditorNodeData } from "../../lib/codec";
import { EDITOR_ROUTE_ROW_HEIGHT } from "../../lib/loop-editor-layout";
import {
  editorNodeClassLabel,
  environmentReadout,
  fanOutChips,
  type EnvironmentReadout,
} from "../../lib/loop-editor-node-readout";
import { routeCardRows } from "../../lib/loop-editor-route-edges";
import type { LoopEditorNodeActions } from "../../lib/loop-editor-types";
import { loopNodeCardRows, type LoopNodeCardRow } from "../../lib/loop-node-card-rows";
import {
  LOOP_CALL_TOOL_ICON,
  LOOP_NODE_KIND_ICONS,
  loopNodeClassIcon,
} from "../../lib/loop-node-kind-icons";
import { LoopEditorNodeMenu } from "./loop-editor-node-menu";

const LoopEditorNodeActionsContext = createContext<LoopEditorNodeActions | null>(null);

export function LoopEditorNodeActionsProvider({
  actions,
  children,
}: {
  actions: LoopEditorNodeActions;
  children: ReactNode;
}) {
  return <LoopEditorNodeActionsContext value={actions}>{children}</LoopEditorNodeActionsContext>;
}

const LOOP_EDITOR_KIND_ICON_REGISTRY = {
  ...LOOP_NODE_KIND_ICONS,
  "": LOOP_CALL_TOOL_ICON,
} satisfies KindIconRegistry;

const HANDLE_NUB =
  "!h-5 !min-h-0 !min-w-0 !w-2 !border-line !bg-line transition-[width,left,right] duration-150 group-hover:!w-2.5";

const ORIGIN = { x: 0, y: 0 };

const ROUTE_SUMMARY_ROW_KEYS = new Set(["routes", "default"]);

type RouteCardRow = ReturnType<typeof routeCardRows>[number];

function nodeFrameClassName(hasError: boolean, focused: boolean, selected: boolean): string {
  if (hasError) return cn("border-danger", focused && "ring-2 ring-danger");
  if (focused)
    return "border-line hover:border-line-strong border-accent-dim ring-2 ring-accent-dim";
  return cn(
    "border-line hover:border-line-strong",
    selected && "border-accent-dim/50 ring-1 ring-accent-dim/40"
  );
}

export function LoopEditorNode({ id, data, selected }: NodeProps<EditorNode>) {
  const { raw, nodeClass, kind, hasError } = data;
  const actions = use(LoopEditorNodeActionsContext);

  const focused = data.focused === true;
  const chips = fanOutChips(raw);
  const environment = environmentReadout(raw, data.loopDefaultEnvironment);
  const routeRows = routeCardRows({ id, type: "loopNode", position: ORIGIN, data });
  const rows = loopNodeCardRows(raw).filter(
    row => routeRows.length === 0 || !ROUTE_SUMMARY_ROW_KEYS.has(row.key)
  );
  const hasBody = rows.length > 0 || environment !== undefined || chips.length > 0;
  const connectable = data.readOnly !== true;
  const card = (
    <div
      className={cn(
        "group relative flex w-47 flex-col rounded-md border bg-canvas shadow-card transition-colors",
        nodeFrameClassName(hasError, focused, selected)
      )}
      data-testid="loop-editor-node"
      data-node-id={raw.id}
      data-node-error={hasError ? "true" : "false"}
      data-node-focused={focused ? "true" : "false"}
      data-node-selected={selected ? "true" : "false"}
    >
      <Handle
        className={cn(HANDLE_NUB, "!-left-2 !rounded-l-xxs !rounded-r-none group-hover:!-left-2.5")}
        isConnectable={connectable}
        position={Position.Left}
        type="target"
      />
      <LoopEditorNodeHeader
        focused={focused}
        hasBody={hasBody}
        kind={kind}
        nodeClass={nodeClass}
        nodeId={String(raw.id)}
      />
      {hasBody ? <LoopEditorNodeBody chips={chips} environment={environment} rows={rows} /> : null}
      {routeRows.length > 0 ? (
        <LoopEditorNodeRoutes connectable={connectable} routeRows={routeRows} />
      ) : null}
      {hasError ? <LoopEditorNodeErrorBadge /> : null}
      {routeRows.length === 0 ? (
        <Handle
          className={cn(
            HANDLE_NUB,
            "!-right-2 !rounded-l-none !rounded-r-xxs group-hover:!-right-2.5"
          )}
          isConnectable={connectable}
          position={Position.Right}
          type="source"
        />
      ) : null}
    </div>
  );

  if (!actions) return card;
  return (
    <LoopEditorNodeMenu
      canPaste={actions.canPaste}
      nodeId={raw.id}
      onCopy={actions.onCopy}
      onDelete={actions.onDelete}
      onDuplicate={actions.onDuplicate}
      onPaste={actions.onPaste}
      onRename={actions.onRename}
      readOnly={actions.readOnly}
    >
      {card}
    </LoopEditorNodeMenu>
  );
}

function LoopEditorNodeHeader({
  nodeId,
  nodeClass,
  kind,
  focused,
  hasBody,
}: {
  nodeId: string;
  nodeClass: EditorNodeData["nodeClass"];
  kind: string;
  focused: boolean;
  hasBody: boolean;
}) {
  const classGlyph = loopNodeClassIcon({
    nodeClass: nodeClass ?? "action",
    isFanOut: kind === "fan-out",
    isGate: kind === "gate",
  });
  return (
    <div
      className={cn(
        "flex min-h-10 items-center gap-2 px-2.5 py-2",
        hasBody && "border-b border-line-soft"
      )}
    >
      <span
        className={cn(
          "grid size-6 shrink-0 place-items-center rounded bg-surface-2",
          focused ? "text-accent-strong" : "text-muted"
        )}
      >
        <KindIcon
          className="size-3.5"
          fallback={classGlyph}
          kind={kind}
          registry={LOOP_EDITOR_KIND_ICON_REGISTRY}
          size="xs"
          tone={focused ? "accent" : "muted"}
        />
      </span>
      <span
        className="min-w-0 flex-1 truncate text-small-body font-medium leading-tight text-fg-strong"
        title={nodeId}
      >
        {nodeId}
      </span>
      <Pill size="xs" tone={focused ? "accent" : "neutral"} mono>
        {editorNodeClassLabel(nodeClass, kind)}
      </Pill>
    </div>
  );
}

function LoopEditorNodeBody({
  rows,
  environment,
  chips,
}: {
  rows: readonly LoopNodeCardRow[];
  environment: EnvironmentReadout | undefined;
  chips: readonly string[];
}) {
  return (
    <div className="flex flex-col gap-1 px-2.5 py-2">
      {rows.map(row => (
        <PropertyRow
          className="min-h-0 gap-2 py-0"
          key={row.key}
          label={row.danger ? <span className="text-danger/70">{row.label}</span> : row.label}
          mono
        >
          {row.value}
        </PropertyRow>
      ))}
      {environment ? (
        <PropertyRow className="min-h-0 gap-2 py-0" data-slot="loop-node-card-env" label="env" mono>
          <span
            className={cn("min-w-0 truncate", environment.inherited ? "text-faint" : "text-subtle")}
            data-source={environment.inherited ? "loop-default" : "node"}
            title={environment.label}
          >
            {environment.label}
          </span>
        </PropertyRow>
      ) : null}
      {chips.length > 0 ? (
        <div className="mt-0.5 flex flex-wrap gap-1" data-testid="loop-editor-node-branches">
          {chips.map(chip => (
            <span
              key={chip}
              className="rounded-xs bg-surface-2 px-1 py-px font-mono text-pill-group-badge text-subtle"
            >
              {chip}
            </span>
          ))}
        </div>
      ) : null}
    </div>
  );
}

function LoopEditorNodeRoutes({
  routeRows,
  connectable,
}: {
  routeRows: readonly RouteCardRow[];
  connectable: boolean;
}) {
  return (
    <div className="flex flex-col border-t border-line-soft" data-testid="loop-editor-node-routes">
      {routeRows.map(row => (
        <div
          className="relative flex items-center gap-1.5 px-2.5"
          data-route-handle={row.handle}
          key={row.handle}

          style={{ height: EDITOR_ROUTE_ROW_HEIGHT }}
        >
          <span
            className="min-w-0 flex-1 truncate font-mono text-pill-group-badge text-subtle"
            title={row.label}
          >
            {row.label}
          </span>
          <span className="shrink-0 font-mono text-pill-group-badge text-faint">
            {row.to || "—"}
          </span>
          <Handle
            className={cn(
              HANDLE_NUB,
              "!-right-2 !h-3.5 !rounded-l-none !rounded-r-xxs group-hover:!-right-2.5"
            )}
            id={row.handle}
            isConnectable={connectable}
            position={Position.Right}
            style={{ top: "50%" }}
            type="source"
          />
        </div>
      ))}
    </div>
  );
}

function LoopEditorNodeErrorBadge() {
  return (
    <span
      className="absolute -right-2 -top-2 grid size-5 place-items-center rounded-full border-2 border-canvas bg-danger text-accent-ink"
      data-testid="loop-editor-node-badge"
      aria-label="This step has a problem"
      role="img"
      title="This step has a problem"
    >
      <AlertTriangle aria-hidden="true" className="size-2.5" />
    </span>
  );
}
