import { useEffect, useState } from "react";

import { useQuery } from "@tanstack/react-query";
import { Blocks } from "lucide-react";

import { useActiveWorkspace } from "@/systems/workspace";

import { PaletteDetailView } from "../components/palette-detail-view";
import { PaletteFormView } from "../components/palette-form-view";
import { PaletteGridView } from "../components/palette-grid-view";
import { PaletteListRow } from "../components/palette-list-row";
import { PaletteViewChips } from "../components/palette-view-chips";
import { OsPaletteViewNote } from "../components/os-palette-view-note";
import type { CmdPaletteDispatch } from "./use-cmd-palette-dispatch";
import type {
  CmdPaletteViewAction,
  CmdPaletteViewEnvelope,
  CmdPaletteViewPayload,
  CmdPaletteViewRow,
  ResolvedPaletteCommand,
} from "../lib/cmd-palette-types";
import { cmdPaletteViewOptions } from "../lib/cmd-palette-query-options";
import type { PaletteViewContent, PaletteViewDefinition } from "../lib/palette-view-registry";

const VIEW_TIMEOUT_MS = 3_000;

export interface CmdPaletteDeclarativeViewModel {
  readonly content: PaletteViewContent;
  readonly definition: PaletteViewDefinition;
  readonly error: string | null;
  readonly loading: boolean;
  readonly timedOut: boolean;
  retry(): void;
}

export function useCmdPaletteDeclarativeView({
  dispatch,
  onDismiss,
  query,
  viewId,
  enabled = true,
}: {
  dispatch: CmdPaletteDispatch;
  onDismiss: () => void;
  query: string;
  viewId: string;
  enabled?: boolean;
}): CmdPaletteDeclarativeViewModel {
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const request = useQuery(cmdPaletteViewOptions(runtimeWorkspaceId, viewId, enabled));
  const [timedOut, setTimedOut] = useState(false);
  const [selectedRow, setSelectedRow] = useState("");
  const [activeChip, setActiveChip] = useState("all");

  useEffect(() => {
    if (!request.isPending) return;
    const timer = window.setTimeout(() => setTimedOut(true), VIEW_TIMEOUT_MS);
    return () => window.clearTimeout(timer);
  }, [request.isPending]);

  const envelope = request.data;
  const definition = viewDefinition(viewId, envelope);
  const runAction = async (
    action: CmdPaletteViewAction | undefined,
    values: Readonly<Record<string, unknown>> = {}
  ) => {
    if (!action?.action) throw new Error("This view action requires a live view program.");
    const command = commandForViewAction(viewId, action);
    const outcome = await dispatch.run(command, { args: values });
    if (outcome.status === "refused") throw new Error(outcome.reason);
    if (outcome.status === "ran" || outcome.status === "invoked") onDismiss();
  };

  return {
    definition,
    content: envelope
      ? contentForEnvelope({
          activeChip,
          envelope,
          query,
          runAction,
          selectedRow,
          setActiveChip,
          setSelectedRow,
        })
      : emptyFrame(request.isPending ? "Loading view…" : "View unavailable"),
    error: request.error instanceof Error ? request.error.message : null,
    loading: request.isPending,
    timedOut: request.isPending && timedOut,
    retry: () => {
      setTimedOut(false);
      void request.refetch();
    },
  };
}

export function contentForEnvelope(input: {
  activeChip: string;
  envelope: CmdPaletteViewEnvelope;
  query: string;
  runAction: (
    action: CmdPaletteViewAction | undefined,
    values?: Readonly<Record<string, unknown>>
  ) => Promise<void>;
  selectedRow: string;
  setActiveChip: (id: string) => void;
  setSelectedRow: (id: string) => void;
  filterLocally?: boolean;
}): PaletteViewContent {
  const { envelope, runAction } = input;
  switch (envelope.kind) {
    case "list":
      return listContent(input);
    case "detail":
      return bodyContent(
        "detail",
        <PaletteDetailView detail={envelope.payload.detail ?? null} onAction={runAction} />
      );
    case "form":
      return envelope.payload.form
        ? bodyContent(
            "form",
            <PaletteFormView
              form={envelope.payload.form}
              onSubmit={(action, values) => runAction(action, values)}
            />
          )
        : emptyFrame("Form unavailable");
    case "grid":
      return envelope.payload.grid
        ? bodyContent(
            "grid",
            <PaletteGridView
              columns={envelope.payload.chrome?.columns}
              empty={envelope.payload.empty}
              grid={envelope.payload.grid}
              onAction={action => void runAction(action)}
            />
          )
        : emptyFrame("Grid unavailable");
    default:
      return emptyFrame(`This host cannot render the “${envelope.kind}” view kind.`);
  }
}

function listContent(input: {
  activeChip: string;
  envelope: CmdPaletteViewEnvelope;
  query: string;
  runAction: (action: CmdPaletteViewAction | undefined) => Promise<void>;
  selectedRow: string;
  setActiveChip: (id: string) => void;
  setSelectedRow: (id: string) => void;
  filterLocally?: boolean;
}): PaletteViewContent {
  const payload = input.envelope.payload;
  const rows = (payload.sections ?? []).flatMap(section => section.rows);
  const filtered =
    input.filterLocally === false
      ? rows
      : rows.filter(row => rowMatches(row, input.query, input.activeChip));
  const selected = filtered.find(row => row.id === input.selectedRow) ?? filtered[0] ?? null;
  return {
    kind: "list",
    rows: filtered.map(row => ({
      value: row.id,
      testId: `palette-view-row-${row.id}`,
      node: <PaletteListRow row={row} />,
      onSelect: () => void input.runAction(primaryAction(row)),
    })),
    header:
      payload.chips && payload.chips.length > 0 ? (
        <PaletteViewChips
          active={input.activeChip}
          chips={[{ id: "all", label: "All", count: rows.length }, ...payload.chips]}
          onChange={input.setActiveChip}
        />
      ) : null,
    empty: (
      <OsPaletteViewNote placement="empty">
        {payload.empty?.title ?? "No items yet"}
        {payload.empty?.hint ? ` — ${payload.empty.hint}` : ""}
      </OsPaletteViewNote>
    ),
    note: overflowNote(payload),
    aside:
      rows.some(row => row.detail) || payload.detail ? (
        <PaletteDetailView
          detail={selected?.detail ?? payload.detail ?? null}
          onAction={action => void input.runAction(action)}
        />
      ) : undefined,
    backHint: input.activeChip === "all" ? "back" : "clear filter",
    resetKey: input.activeChip,
    onSelectionChange: input.setSelectedRow,
    onEmptyQueryBackspace: () => {
      if (input.activeChip === "all") return false;
      input.setActiveChip("all");
      return true;
    },
  };
}

function bodyContent(kind: "detail" | "form" | "grid", body: React.ReactNode): PaletteViewContent {
  return {
    kind,
    rows: [],
    body,
    header: null,
    empty: null,
    note: null,
    backHint: "back",
    resetKey: kind,
    onEmptyQueryBackspace: () => false,
  };
}

export function emptyFrame(message: string): PaletteViewContent {
  return {
    kind: "list",
    rows: [],
    header: null,
    empty: <OsPaletteViewNote placement="empty">{message}</OsPaletteViewNote>,
    note: null,
    backHint: "back",
    resetKey: message,
    onEmptyQueryBackspace: () => false,
  };
}

export function viewDefinition(
  viewId: string,
  envelope: CmdPaletteViewEnvelope | undefined
): PaletteViewDefinition {
  const source = extensionName(viewId);
  return {
    id: viewId,
    title: envelope?.title ?? viewId,
    icon: Blocks,
    placeholder: envelope?.payload.chrome?.search_placeholder ?? "Search view…",
    enterHint: "open",
    description: source
      ? `${envelope?.title ?? viewId} from ${source}`
      : (envelope?.title ?? viewId),
  };
}

export function extensionName(viewId: string): string | null {
  const match = /^ext\.([^.]+)\./.exec(viewId);
  return match?.[1] ?? null;
}

function rowMatches(row: CmdPaletteViewRow, query: string, activeChip: string): boolean {
  const normalized = query.trim().toLocaleLowerCase();
  const chipMatches = activeChip === "all" || row.keywords?.includes(activeChip);
  if (!chipMatches) return false;
  if (normalized === "") return true;
  return [row.title, row.subtitle, ...(row.keywords ?? [])].some(value =>
    value?.toLocaleLowerCase().includes(normalized)
  );
}

function primaryAction(row: CmdPaletteViewRow): CmdPaletteViewAction | undefined {
  return row.actions?.find(action => action.primary) ?? row.actions?.[0];
}

function overflowNote(payload: CmdPaletteViewPayload): React.ReactNode {
  const pagination = payload.chrome?.pagination;
  if (!pagination?.has_more || !payload.empty?.hint) return null;
  return <OsPaletteViewNote>{payload.empty.hint}</OsPaletteViewNote>;
}

export function commandForViewAction(
  viewId: string,
  action: CmdPaletteViewAction
): ResolvedPaletteCommand {
  const target = action.action;
  if (!target) throw new Error("This view action requires a live view program.");
  const id = viewActionCommandID(viewId, action);
  return {
    id,
    title: action.title,
    section: action.section ?? "View",
    icon: action.icon ?? "command",
    source: extensionName(viewId) ? `ext.${extensionName(viewId)}` : "core",
    available: true,
    reason: "",
    bindings: action.shortcut ? [action.shortcut] : [],
    alias: null,
    destructive: action.destructive ?? false,
    confirmation: action.confirmation ?? null,
    arguments: [],
    action: target,
    execution: { retry_safe: false, single_flight: false },
    availability_exempt: false,
    visible: true,
    chords: action.shortcut ? [action.shortcut] : [],
  };
}

/** Maps an extension-local action tool to its daemon-canonical command id. */
export function viewActionCommandID(viewId: string, action: CmdPaletteViewAction): string {
  const target = action.action;
  if (target?.kind !== "tool" || !target.tool) return `view-action.${viewId}`;
  if (target.tool.startsWith("ext.")) return target.tool;
  const extension = extensionName(viewId);
  return extension ? `ext.${extension}.${target.tool}` : target.tool;
}
