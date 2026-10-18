import type { ReactNode } from "react";
import { FileText } from "lucide-react";

import { Button, Empty, Panel, Pill, Skeleton, Spinner } from "@compozy/ui";

import type {
  AuthoredFileKind,
  useAgentAuthoredFileEditor,
} from "../hooks/use-agent-authored-file-editor";
import {
  authoredFileRevisionIdentity,
  authoredFileStatusTone,
} from "../lib/agent-authored-file-view";

type AuthoredFileEditorModel = ReturnType<typeof useAgentAuthoredFileEditor>;
type AuthoredFileDiagnostic = AuthoredFileEditorModel["diagnostics"][number];
type AuthoredFileRevision = AuthoredFileEditorModel["revisions"][number];

export function AuthoredFileWriteRecovery({
  kind,
  saveError,
  conflict,
  onReload,
}: {
  kind: AuthoredFileKind;
  saveError: string | null;
  conflict: boolean;
  onReload: () => void;
}) {
  if (!saveError && !conflict) return null;
  return (
    <div className="mt-3 flex flex-wrap items-center gap-2" role="alert">
      {saveError ? <p className="text-small-body text-danger">{saveError}</p> : null}
      {conflict ? (
        <Button
          type="button"
          size="sm"
          variant="ghost"
          onClick={onReload}
          data-testid={`agent-${kind}-reload`}
        >
          Reload
        </Button>
      ) : null}
    </div>
  );
}

export function AuthoredFileLoading({ kind }: { kind: AuthoredFileKind }) {
  return (
    <div className="flex flex-col gap-3" data-testid={`agent-${kind}-loading`}>
      <Skeleton className="h-8 w-48" />
      <Skeleton className="h-64 rounded-md" />
    </div>
  );
}

export function AuthoredFileLoadError({
  kind,
  fileLabel,
  onRetry,
}: {
  kind: AuthoredFileKind;
  fileLabel: string;
  onRetry: () => void;
}) {
  return (
    <Empty
      icon={FileText}
      title={`Couldn't load ${fileLabel}`}
      description="Retry to fetch the authored file."
      action={
        <Button type="button" size="sm" variant="ghost" onClick={onRetry}>
          Retry
        </Button>
      }
      data-testid={`agent-${kind}-error`}
      fill={false}
    />
  );
}

const MISSING_DESCRIPTIONS: Record<AuthoredFileKind, string> = {
  soul: "This agent has no soul file. Create one to define persona and constraints.",
  heartbeat: "This agent has no heartbeat policy. Create one to schedule wake checks.",
};

export function AuthoredFileMissing({
  kind,
  fileLabel,
  saving,
  onCreate,
  headerSlot,
  recovery,
}: {
  kind: AuthoredFileKind;
  fileLabel: string;
  saving: boolean;
  onCreate: () => void;
  headerSlot: ReactNode;
  recovery: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-4" data-testid={`agent-${kind}-missing`}>
      {headerSlot}
      <Panel bodyClassName="p-0">
        <Empty
          icon={FileText}
          title={`No ${fileLabel}`}
          description={MISSING_DESCRIPTIONS[kind]}
          action={
            <Button
              type="button"
              size="sm"
              onClick={onCreate}
              disabled={saving}
              data-testid={`agent-${kind}-create`}
            >
              {saving ? <Spinner className="size-3" /> : null}
              Create {fileLabel}
            </Button>
          }
          fill={false}
          className="px-4 py-8"
        />
      </Panel>
      {recovery}
    </div>
  );
}

export function AuthoredFileMeta({
  status,
  payload,
}: Pick<AuthoredFileEditorModel, "status" | "payload">) {
  return (
    <div className="flex flex-wrap items-center gap-2 border-b border-line-soft px-4 py-2.5">
      {/* Lifecycle states, not tags: dot + word, no plate. */}
      <Pill form="plain" tone={authoredFileStatusTone(status)}>
        <Pill.Dot />
        {status}
      </Pill>
      {payload && "enabled" in payload ? (
        <Pill form="plain" tone={payload.enabled ? "success" : "neutral"}>
          <Pill.Dot />
          {payload.enabled ? "enabled" : "disabled"}
        </Pill>
      ) : null}
      {payload?.source_path ? (
        <code className="font-mono text-badge tracking-mono text-muted">{payload.source_path}</code>
      ) : null}
    </div>
  );
}

function AuthoredFileDiagnosticItem({ item }: { item: AuthoredFileDiagnostic }) {
  return (
    <li className="text-small-body text-danger">
      {item.message}
      {item.line != null ? (
        <span className="ml-2 font-mono text-muted">line {item.line}</span>
      ) : null}
      {item.source_path ? (
        <code className="ml-2 font-mono text-muted">{item.source_path}</code>
      ) : null}
    </li>
  );
}

export function AuthoredFileDiagnostics({
  kind,
  diagnostics,
}: {
  kind: AuthoredFileKind;
  diagnostics: AuthoredFileDiagnostic[];
}) {
  if (diagnostics.length === 0) return null;
  return (
    <ul className="flex flex-col gap-1 px-4 py-3" data-testid={`agent-${kind}-diagnostics`}>
      {diagnostics.map(item => (
        <AuthoredFileDiagnosticItem
          key={`${item.source_path ?? "unknown"}:${item.line ?? "unknown"}:${item.message}`}
          item={item}
        />
      ))}
    </ul>
  );
}

export function AuthoredFileActions({
  kind,
  editor,
  saving,
  validating,
}: {
  kind: AuthoredFileKind;
  editor: AuthoredFileEditorModel;
  saving: boolean;
  validating: boolean;
}) {
  return (
    <div className="flex flex-wrap items-center gap-2 px-4 py-3">
      <Button
        type="button"
        size="sm"
        variant="ghost"
        onClick={() => void editor.handleValidate()}
        disabled={validating || saving}
        data-testid={`agent-${kind}-validate`}
      >
        {validating ? <Spinner className="size-3" /> : null}
        {validating ? "Validating…" : "Validate"}
      </Button>
      <span className="min-w-0 flex-1" />
      <Button
        type="button"
        size="sm"
        variant="ghost"
        onClick={() => editor.setShowHistory(current => !current)}
        data-testid={`agent-${kind}-history-toggle`}
      >
        History
      </Button>
      <Button
        type="button"
        size="sm"
        onClick={() => void editor.handleSave()}
        disabled={!editor.dirty || saving}
        data-testid={`agent-${kind}-save`}
      >
        {saving ? <Spinner className="size-3" /> : null}
        {saving ? "Saving…" : "Save"}
      </Button>
    </div>
  );
}

function AuthoredFileRevisionRow({
  kind,
  revision,
  saving,
  onRestore,
}: {
  kind: AuthoredFileKind;
  revision: AuthoredFileRevision;
  saving: boolean;
  onRestore: (revisionId: string) => void;
}) {
  const { id, created } = authoredFileRevisionIdentity(revision);
  return (
    <li
      className="flex items-center justify-between gap-3"
      data-testid={`agent-${kind}-revision-${id}`}
    >
      <span className="font-mono text-badge tracking-mono text-muted">
        {id || "revision"} {created ? `· ${created}` : ""}
      </span>
      <Button
        type="button"
        size="sm"
        variant="ghost"
        onClick={() => onRestore(id)}
        disabled={!id || saving}
        data-testid={`agent-${kind}-restore-${id}`}
      >
        Restore
      </Button>
    </li>
  );
}

export function AuthoredFileHistory({
  kind,
  revisions,
  saving,
  onRestore,
}: {
  kind: AuthoredFileKind;
  revisions: AuthoredFileRevision[];
  saving: boolean;
  onRestore: (revisionId: string) => void;
}) {
  return (
    <div className="border-t border-line-soft px-4 py-3" data-testid={`agent-${kind}-history`}>
      {revisions.length === 0 ? (
        <p className="text-small-body text-muted">No revisions yet.</p>
      ) : (
        <ul className="flex flex-col gap-2">
          {revisions.map(revision => {
            const { id, created } = authoredFileRevisionIdentity(revision);
            return (
              <AuthoredFileRevisionRow
                key={id || created}
                kind={kind}
                revision={revision}
                saving={saving}
                onRestore={onRestore}
              />
            );
          })}
        </ul>
      )}
    </div>
  );
}

export function AuthoredFileLiveStatus({
  fileLabel,
  saving,
  validating,
}: {
  fileLabel: string;
  saving: boolean;
  validating: boolean;
}) {
  let message = "";
  if (saving) message = `Saving ${fileLabel}`;
  else if (validating) message = `Validating ${fileLabel}`;
  return (
    <span className="sr-only" aria-live="polite">
      {message}
    </span>
  );
}
