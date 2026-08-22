import { useEffect, useState } from "react";

import { useQuery } from "@tanstack/react-query";
import { Blocks } from "lucide-react";

import { Button } from "@compozy/ui";

import { useActiveWorkspace } from "@/systems/workspace";

import { PaletteConfirmation } from "../components/os-palette-confirmation";
import { PaletteDetailView } from "../components/palette-detail-view";
import { PaletteFormView } from "../components/palette-form-view";
import { PaletteGridView } from "../components/palette-grid-view";
import { PaletteListRow } from "../components/palette-list-row";
import { PaletteViewChips } from "../components/palette-view-chips";
import { OsPaletteViewNote } from "../components/os-palette-view-note";
import { OsPaletteProgramBand } from "../components/os-palette-program-status";
import type { CmdPaletteDispatch } from "./use-cmd-palette-dispatch";
import { usePaletteRegistry } from "./use-palette-registry";
import type {
  CmdPaletteViewAction,
  CmdPaletteViewDetail,
  CmdPaletteViewEnvelope,
  CmdPaletteViewPayload,
  CmdPaletteViewRow,
  PaletteRegistry,
  ResolvedPaletteCommand,
} from "../lib/cmd-palette-types";
import { cmdPaletteViewOptions } from "../lib/cmd-palette-query-options";
import {
  parseExtensionName,
  type PaletteViewContent,
  type PaletteViewDefinition,
} from "../lib/palette-view-registry";
import { useProfileReadScope } from "@/systems/profiles";

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
  const registry = usePaletteRegistry();
  const { key: profileKey } = useProfileReadScope();
  const request = useQuery(cmdPaletteViewOptions(runtimeWorkspaceId, profileKey, viewId, enabled));
  const [attempt, setAttempt] = useState(0);
  const requestKey = `${viewId}\u0000${attempt}`;
  const [timedOutKey, setTimedOutKey] = useState("");
  const [selectedRowState, setSelectedRowState] = useState({ value: "", viewId });
  const [activeChipState, setActiveChipState] = useState<{
    revision: string;
    value: string | null;
    viewId: string;
  } | null>(null);
  const [pendingConfirm, setPendingConfirm] = useState<{
    action: CmdPaletteViewAction;
    values: Readonly<Record<string, unknown>>;
    viewId: string;
  } | null>(null);

  useEffect(() => {
    if (!request.isPending) return;
    const timer = window.setTimeout(() => setTimedOutKey(requestKey), VIEW_TIMEOUT_MS);
    return () => window.clearTimeout(timer);
  }, [request.isPending, requestKey]);

  const envelope = request.data;
  const selectedRow = selectedRowState.viewId === viewId ? selectedRowState.value : "";
  const activeChip =
    activeChipState?.viewId === viewId && activeChipState.revision === envelope?.revision
      ? activeChipState.value
      : (envelope?.payload.chrome?.active_chip ?? null);
  const currentPending = pendingConfirm?.viewId === viewId ? pendingConfirm : null;
  const definition = viewDefinition(viewId, envelope);
  const executeAction = async (
    action: CmdPaletteViewAction,
    values: Readonly<Record<string, unknown>>,
    confirmed = false
  ) => {
    if (!action.action) throw new Error("This view action requires a live view program.");
    const command = commandForViewAction(viewId, action, registry);
    const outcome = await dispatch.run(command, { args: values, confirmed });
    if (outcome.status === "refused") throw new Error(outcome.reason);
    if (
      (outcome.status === "ran" || outcome.status === "invoked") &&
      action.action.kind !== "view"
    ) {
      onDismiss();
    }
  };
  const runAction = async (
    action: CmdPaletteViewAction | undefined,
    values: Readonly<Record<string, unknown>> = {}
  ) => {
    if (!action) return;
    const cataloged = registry.byId.has(viewActionCommandID(viewId, action));
    if (action.confirmation && (action.handler || !cataloged)) {
      setPendingConfirm({ action, values, viewId });
      return;
    }
    await executeAction(action, values);
  };

  const content = envelope
    ? contentForEnvelope({
        activeChip,
        envelope,
        query,
        runAction,
        selectedRow,
        setActiveChip: value => setActiveChipState({ revision: envelope.revision, value, viewId }),
        setSelectedRow: value => setSelectedRowState({ value, viewId }),
        filterLocally: hostFiltersLocally(envelope.payload.chrome),
      })
    : emptyFrame(request.isPending ? "Loading view…" : "View unavailable");

  return {
    definition,
    content: withViewConfirm(content, currentPending, {
      onCancel: () => setPendingConfirm(null),
      onConfirm: () => {
        const pending = currentPending;
        setPendingConfirm(null);
        if (pending) void executeAction(pending.action, pending.values, true);
      },
    }),
    error: request.error instanceof Error ? request.error.message : null,
    loading: request.isPending,
    timedOut: request.isPending && timedOutKey === requestKey,
    retry: () => {
      setAttempt(current => current + 1);
      void request.refetch();
    },
  };
}

export function contentForEnvelope(input: {
  activeChip: string | null;
  envelope: CmdPaletteViewEnvelope;
  query: string;
  runAction: (
    action: CmdPaletteViewAction | undefined,
    values?: Readonly<Record<string, unknown>>
  ) => Promise<void>;
  selectedRow: string;
  setActiveChip: (id: string | null) => void;
  setSelectedRow: (id: string) => void;
  filterLocally?: boolean;
  runHandler?: (handler: string, args: readonly unknown[], controlled: boolean) => void;
}): PaletteViewContent {
  const { envelope, runAction } = input;
  switch (envelope.kind) {
    case "list":
      return listContent(input);
    case "detail":
      return bodyContent(
        "detail",
        <PaletteDetailView detail={envelope.payload.detail ?? null} onAction={runAction} />,
        chromeHeader(envelope.payload)
      );
    case "form":
      return envelope.payload.form
        ? bodyContent(
            "form",
            <PaletteFormView
              form={envelope.payload.form}
              onEvent={input.runHandler}
              onSubmit={(action, values) => {
                const handler = envelope.payload.form?.on_submit;
                if (handler && input.runHandler) {
                  input.runHandler(handler, [values], false);
                  return Promise.resolve();
                }
                return runAction(action, values);
              }}
            />,
            chromeHeader(envelope.payload)
          )
        : emptyFrame("Form unavailable");
    case "grid":
      return envelope.payload.grid
        ? bodyContent(
            "grid",
            <PaletteGridView
              columns={envelope.payload.chrome?.columns}
              empty={envelope.payload.empty}
              filterLocally={input.filterLocally ?? hostFiltersLocally(envelope.payload.chrome)}
              grid={envelope.payload.grid}
              loading={envelope.payload.chrome?.is_loading}
              query={input.query}
              onAction={action => void runAction(action)}
              onSelectionChange={input.setSelectedRow}
            />,
            chromeHeader(envelope.payload, paginationControl(envelope.payload, input.runHandler))
          )
        : emptyFrame("Grid unavailable");
    default:
      return emptyFrame(`This host cannot render the “${envelope.kind}” view kind.`);
  }
}

function listContent(input: {
  activeChip: string | null;
  envelope: CmdPaletteViewEnvelope;
  query: string;
  runAction: (action: CmdPaletteViewAction | undefined) => Promise<void>;
  selectedRow: string;
  setActiveChip: (id: string | null) => void;
  setSelectedRow: (id: string) => void;
  filterLocally?: boolean;
  runHandler?: (handler: string, args: readonly unknown[], controlled: boolean) => void;
}): PaletteViewContent {
  const payload = input.envelope.payload;
  const rows = (payload.sections ?? []).flatMap(section => section.rows);
  const filterLocally = input.filterLocally ?? hostFiltersLocally(payload.chrome);
  const filtered = filterLocally
    ? rows.filter(row => rowMatches(row, input.query, input.activeChip))
    : rows;
  const selected = filtered.find(row => row.id === input.selectedRow) ?? filtered[0] ?? null;
  return {
    kind: "list",
    rows: filtered.map(row => {
      const action = primaryAction(row);
      return {
        value: row.id,
        testId: `palette-view-row-${row.id}`,
        twoLine: Boolean(row.subtitle),
        node: <PaletteListRow row={row} />,
        disabled: action === undefined,
        onSelect: () => {
          if (action) void input.runAction(action);
        },
      };
    }),
    header: chromeHeader(
      payload,
      payload.chips && payload.chips.length > 0 ? (
        <PaletteViewChips
          active={input.activeChip}
          allCount={rows.length}
          chips={payload.chips}
          onChange={input.setActiveChip}
        />
      ) : null,
      paginationControl(payload, input.runHandler)
    ),
    empty: payload.chrome?.is_loading ? null : (
      <OsPaletteViewNote placement="empty">
        {payload.empty?.title ?? "No items yet"}
        {payload.empty?.hint ? ` — ${payload.empty.hint}` : ""}
      </OsPaletteViewNote>
    ),
    note: null,
    aside: listAside(rows, payload, selected) ? (
      <PaletteDetailView
        detail={rowDetail(selected, payload)}
        onAction={action => void input.runAction(action)}
      />
    ) : undefined,
    backHint: input.activeChip === null ? "back" : "clear filter",
    resetKey: input.activeChip ?? "all",
    onSelectionChange: input.setSelectedRow,
    onEmptyQueryBackspace: () => {
      if (input.activeChip === null) return false;
      input.setActiveChip(null);
      return true;
    },
  };
}

function bodyContent(
  kind: "detail" | "form" | "grid",
  body: React.ReactNode,
  header: React.ReactNode = null
): PaletteViewContent {
  return {
    kind,
    rows: [],
    body,
    header,
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
  return parseExtensionName(viewId);
}

function rowMatches(row: CmdPaletteViewRow, query: string, activeChip: string | null): boolean {
  const normalized = query.trim().toLocaleLowerCase();
  const chipMatches = activeChip === null || row.keywords?.includes(activeChip);
  if (!chipMatches) return false;
  if (normalized === "") return true;
  return [row.title, row.subtitle, ...(row.keywords ?? [])].some(value =>
    value?.toLocaleLowerCase().includes(normalized)
  );
}

function primaryAction(row: CmdPaletteViewRow): CmdPaletteViewAction | undefined {
  return row.actions?.find(action => action.primary) ?? row.actions?.[0];
}

function paginationControl(
  payload: CmdPaletteViewPayload,
  runHandler: ((handler: string, args: readonly unknown[], controlled: boolean) => void) | undefined
): React.ReactNode {
  const handler = payload.chrome?.on_load_more;
  if (!payload.chrome?.pagination?.has_more || !handler || !runHandler) return null;
  return (
    <div className="flex justify-center border-b border-line px-3 py-2">
      <Button size="sm" variant="outline" onClick={() => runHandler(handler, [], false)}>
        Load more
      </Button>
    </div>
  );
}

function chromeHeader(
  payload: CmdPaletteViewPayload,
  primary?: React.ReactNode,
  secondary?: React.ReactNode
): React.ReactNode {
  const loading = payload.chrome?.is_loading === true;
  if (!primary && !secondary && !loading) return null;
  return (
    <>
      {primary}
      {secondary}
      {loading ? <OsPaletteProgramBand phase="busy" onRetry={() => undefined} /> : null}
    </>
  );
}

export function hostFiltersLocally(chrome: CmdPaletteViewPayload["chrome"] | undefined): boolean {
  if (typeof chrome?.filtering === "boolean") return chrome.filtering;
  return chrome?.complete === true;
}

export function commandForViewAction(
  viewId: string,
  action: CmdPaletteViewAction,
  registry?: PaletteRegistry
): ResolvedPaletteCommand {
  const target = action.action;
  if (!target) throw new Error("This view action requires a live view program.");
  if (target.kind === "client_op") {
    throw new Error("View actions cannot run client operations.");
  }
  const shortcuts = action.shortcut ? [action.shortcut] : [];
  const id = viewActionCommandID(viewId, action);
  const catalog = registry?.byId.get(id);
  if (catalog) {
    return {
      ...catalog,
      confirmation: action.confirmation ?? catalog.confirmation,
      destructive: action.destructive ?? catalog.destructive,
    };
  }
  const available = target.kind !== "tool";
  return {
    id,
    title: action.title,
    section: action.section ?? "View",
    icon: action.icon ?? "command",
    source: extensionName(viewId) ? `ext.${extensionName(viewId)}` : "core",
    available,
    reason: available ? "" : "this command is not in the catalog",
    bindings: shortcuts,
    alias: null,
    destructive: action.destructive ?? false,
    confirmation: action.confirmation ?? null,
    arguments: [],
    action: target,
    execution: { retry_safe: false, single_flight: false },
    availability_exempt: false,
    visible: true,
    chords: shortcuts,
  };
}

/** Maps an extension-local action tool to its daemon-canonical command id. */
export function viewActionCommandID(viewId: string, action: CmdPaletteViewAction): string {
  const target = action.action;
  if (target?.kind !== "tool" || !target.tool) return `view-action.${viewId}`;
  const tool = target.tool.trim();
  if (tool.startsWith("ext__")) return tool;
  if (tool.startsWith("ext.")) return tool.replaceAll(".", "__");
  const extension = extensionName(viewId);
  return extension ? `ext__${extension}__${tool}` : tool;
}

function listAside(
  rows: readonly CmdPaletteViewRow[],
  payload: CmdPaletteViewPayload,
  selected: CmdPaletteViewRow | null
): boolean {
  return (
    payload.detail != null ||
    rows.some(row => row.detail != null || (row.actions?.length ?? 0) > 0) ||
    (selected?.actions?.length ?? 0) > 0
  );
}

function rowDetail(
  row: CmdPaletteViewRow | null,
  payload: CmdPaletteViewPayload
): CmdPaletteViewDetail | null {
  if (row?.detail) return row.detail;
  if (payload.detail) return payload.detail;
  if (row?.actions && row.actions.length > 0) return { actions: [...row.actions] };
  return null;
}

function withViewConfirm(
  content: PaletteViewContent,
  pending: { action: CmdPaletteViewAction; values: Readonly<Record<string, unknown>> } | null,
  handlers: { onCancel: () => void; onConfirm: () => void }
): PaletteViewContent {
  const confirmation = pending?.action.confirmation;
  if (!confirmation) return content;
  return {
    ...content,
    header: (
      <>
        <PaletteConfirmation
          confirmation={confirmation}
          destructive={pending.action.destructive ?? false}
          invalidatedReason=""
          onCancel={handlers.onCancel}
          onConfirm={handlers.onConfirm}
        />
        {content.header}
      </>
    ),
  };
}
