import { createContext, use, type ReactNode } from "react";
import { AlertTriangle } from "lucide-react";
import { Handle, Position, type NodeProps } from "@xyflow/react";

import { cn, KindIcon, PropertyRow, type KindIconRegistry } from "@compozy/ui";

import type { EditorNode } from "../../lib/codec";
import { EDITOR_ROUTE_ROW_HEIGHT } from "../../lib/loop-editor-layout";
import { routeCardRows } from "../../lib/loop-editor-route-edges";
import type { LoopEditorNodeActions } from "../../lib/loop-editor-types";
import { loopNodeCardRows } from "../../lib/loop-node-card-rows";
import {
  LOOP_CALL_TOOL_ICON,
  LOOP_NODE_KIND_ICONS,
  loopNodeClassIcon,
} from "../../lib/loop-node-kind-icons";
import { LOOP_ENVIRONMENT_MODE_LABELS } from "../../lib/loop-node-schema-types";
import type { LoopEnvironmentMode, LoopEnvironmentSpec } from "../../types";
import { MonoTag } from "../mono-tag";
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

const KIND_IN_LABEL = new Set(["gate", "fan-out", "collect", "branch", "sub-loop", "route", "ask"]);

const LOOP_EDITOR_KIND_ICON_REGISTRY = {
  ...LOOP_NODE_KIND_ICONS,
  "": LOOP_CALL_TOOL_ICON,
} satisfies KindIconRegistry;

const HANDLE_NUB =
  "!h-5 !min-h-0 !min-w-0 !w-[7px] !border-line !bg-line transition-[width,left,right] duration-150 group-hover:!w-[9px]";

const ORIGIN = { x: 0, y: 0 };

function classLabel(nodeClass: string | null, kind: string): string {
  const base = nodeClass ?? "node";
  return KIND_IN_LABEL.has(kind) ? `${base} · ${kind}` : base;
}

function num(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

/** The fan-out knob chips (batch / parallelism / author bound) — the author-time truthful
 *  "branch" summary; runtime fanned items only exist on the run page, never here. */
function fanOutChips(raw: EditorNode["data"]["raw"]): string[] {
  if (raw.kind !== "fan-out") return [];
  const chips: string[] = [];
  const batch = num(raw.batch_size);
  const parallel = num(raw.max_parallel);
  const authorBound = num(raw.max_fan_out);
  if (batch !== undefined) chips.push(`batch ${batch}`);
  if (parallel !== undefined) chips.push(parallel <= 1 ? "seq" : `×${parallel}`);
  if (authorBound !== undefined) chips.push(`≤${authorBound}`);
  return chips;
}

/**
 * The node's own environment declaration, as a short readout.
 *
 * Only what the node itself declares is rendered: the loop-level default is not
 * part of the canvas node model, so naming it here would be a guess rather than
 * a readback.
 */
interface EnvironmentReadout {
  label: string;
  inherited: boolean;
}

function specLabel(spec: {
  mode?: unknown;
  worktree_ref?: unknown;
  directory?: unknown;
}): string | undefined {
  if (typeof spec.mode !== "string") return undefined;
  if (spec.mode === "worktree" && typeof spec.worktree_ref === "string" && spec.worktree_ref) {
    return `${LOOP_ENVIRONMENT_MODE_LABELS.worktree} · ${spec.worktree_ref}`;
  }
  if (spec.mode === "directory" && typeof spec.directory === "string" && spec.directory) {
    return `${LOOP_ENVIRONMENT_MODE_LABELS.directory} · ${spec.directory}`;
  }
  if (spec.mode in LOOP_ENVIRONMENT_MODE_LABELS) {
    return LOOP_ENVIRONMENT_MODE_LABELS[spec.mode as LoopEnvironmentMode];
  }
  return undefined;
}

function environmentReadout(
  raw: EditorNode["data"]["raw"],
  loopDefault?: LoopEnvironmentSpec
): EnvironmentReadout | undefined {
  if (raw.kind !== "run-agent" && raw.kind !== "goal") return undefined;
  const params = raw.params;
  if (typeof params === "object" && params !== null) {
    const environment = (params as Record<string, unknown>).environment;
    if (typeof environment === "object" && environment !== null) {
      const label = specLabel(environment as Record<string, unknown>);
      if (label) return { label, inherited: false };
    }
  }
  const defaultLabel = loopDefault ? specLabel(loopDefault) : undefined;
  return defaultLabel ? { label: `${defaultLabel} · loop default`, inherited: true } : undefined;
}

const ROUTE_SUMMARY_ROW_KEYS = new Set(["routes", "default"]);

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
  const classGlyph = loopNodeClassIcon({
    nodeClass: nodeClass ?? "action",
    isFanOut: kind === "fan-out",
    isGate: kind === "gate",
  });
  const card = (
    <div
      className={cn(
        "group relative flex w-[188px] flex-col rounded-md border bg-canvas-tint transition-colors",
        hasError ? "border-danger" : "border-line hover:border-line-strong",
        focused && !hasError && "border-accent-dim ring-[1.75px] ring-accent-dim",
        focused && hasError && "ring-[1.75px] ring-danger",
        !focused && selected && !hasError && "border-accent-dim/50 ring-1 ring-accent-dim/40"
      )}
      data-testid="loop-editor-node"
      data-node-id={raw.id}
      data-node-error={hasError ? "true" : "false"}
      data-node-focused={focused ? "true" : "false"}
      data-node-selected={selected ? "true" : "false"}
    >
      <Handle
        className={cn(
          HANDLE_NUB,
          "!-left-2 !rounded-l-[2px] !rounded-r-none group-hover:!-left-2.5"
        )}
        isConnectable={connectable}
        position={Position.Left}
        type="target"
      />
      <div
        className={cn(
          "flex min-h-9.5 items-center gap-2 px-2.5 py-2",
          hasBody && "border-b border-line-soft"
        )}
      >
        <span
          className={cn(
            "grid size-6 shrink-0 place-items-center rounded border border-line-strong bg-elevated",
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
          title={String(raw.id)}
        >
          {String(raw.id)}
        </span>
        <MonoTag
          className={cn(
            "shrink-0 text-pill-group-badge tracking-[0.07em]",
            focused ? "text-accent-strong" : "text-faint"
          )}
        >
          {classLabel(nodeClass, kind)}
        </MonoTag>
      </div>
      {hasBody ? (
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
            <PropertyRow
              className="min-h-0 gap-2 py-0"
              data-slot="loop-node-card-env"
              label="env"
              mono
            >
              <span
                className={cn(
                  "min-w-0 truncate",
                  environment.inherited ? "text-faint" : "text-subtle"
                )}
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
                  className="rounded-xs bg-badge-fill px-1 py-px font-mono text-pill-group-badge text-subtle"
                >
                  {chip}
                </span>
              ))}
            </div>
          ) : null}
        </div>
      ) : null}
      {routeRows.length > 0 ? (
        <div
          className="flex flex-col border-t border-line-soft"
          data-testid="loop-editor-node-routes"
        >
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
                  "!-right-2 !h-3.5 !rounded-l-none !rounded-r-[2px] group-hover:!-right-2.5"
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
      ) : null}
      {hasError ? (
        <span
          className="absolute -right-2 -top-2 grid size-4.5 place-items-center rounded-full border-2 border-canvas bg-danger text-accent-ink"
          data-testid="loop-editor-node-badge"
          title="This node has a validation error"
        >
          <AlertTriangle aria-hidden="true" className="size-2.5" />
        </span>
      ) : null}
      {routeRows.length === 0 ? (
        <Handle
          className={cn(
            HANDLE_NUB,
            "!-right-2 !rounded-l-none !rounded-r-[2px] group-hover:!-right-2.5"
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
