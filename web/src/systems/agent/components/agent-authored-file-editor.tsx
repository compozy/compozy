import type { ReactNode } from "react";
import { FileText } from "lucide-react";

import { Button, Empty, Pill, Skeleton, Spinner, Textarea } from "@agh/ui";

import {
  useAgentAuthoredFileEditor,
  type AuthoredFileKind,
  type AuthoredFilePayload,
} from "../hooks/use-agent-authored-file-editor";
import type {
  AgentHeartbeatHistoryResponse,
  AgentHeartbeatPayload,
  AgentSoulHistoryResponse,
  AgentSoulPayload,
} from "../types";
import { AgentPanelBox } from "./agent-panel-box";

export type { AuthoredFileKind };

export interface AgentAuthoredFileEditorProps {
  resourceKey: string;
  kind: AuthoredFileKind;
  payload: AgentSoulPayload | AgentHeartbeatPayload | undefined;
  isLoading: boolean;
  isError: boolean;
  history: AgentSoulHistoryResponse | AgentHeartbeatHistoryResponse | undefined;
  onValidate: (body: string) => Promise<{
    diagnostics?: Array<{ message: string; line?: number; source_path?: string }>;
    validation_status?: string;
  }>;
  onSave: (body: string, expectedDigest: string) => Promise<AuthoredFilePayload>;
  onRestore: (revisionId: string, expectedDigest: string) => Promise<AuthoredFilePayload>;
  onRetry: () => void;
  /** Extra content rendered above the editor (heartbeat status/wake). */
  headerSlot?: ReactNode;
}

function AuthoredFileWriteRecovery({
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

export function AgentAuthoredFileEditor({
  resourceKey,
  kind,
  payload,
  isLoading,
  isError,
  history,
  onValidate,
  onSave,
  onRestore,
  onRetry,
  headerSlot,
}: AgentAuthoredFileEditorProps) {
  const editor = useAgentAuthoredFileEditor({
    resourceKey,
    kind,
    payload,
    history,
    onValidate,
    onSave,
    onRestore,
  });
  const handleReload = () => {
    editor.handleReload();
    onRetry();
  };

  if (isLoading) {
    return (
      <div className="flex flex-col gap-3" data-testid={`agent-${kind}-loading`}>
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-64 rounded-md" />
      </div>
    );
  }

  if (isError) {
    return (
      <Empty
        icon={FileText}
        title={`Couldn't load ${editor.fileLabel}`}
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

  if (editor.isMissing) {
    return (
      <div className="flex flex-col gap-4" data-testid={`agent-${kind}-missing`}>
        {headerSlot}
        <AgentPanelBox>
          <Empty
            icon={FileText}
            title={`No ${editor.fileLabel}`}
            description={
              kind === "soul"
                ? "This agent has no soul file. Create one to define persona and constraints."
                : "This agent has no heartbeat policy. Create one to schedule wake checks."
            }
            action={
              <Button
                type="button"
                size="sm"
                onClick={() => void editor.handleCreate()}
                disabled={editor.saving}
                data-testid={`agent-${kind}-create`}
              >
                {editor.saving ? <Spinner className="size-3" /> : null}
                Create {editor.fileLabel}
              </Button>
            }
            fill={false}
            className="px-4 py-8"
          />
        </AgentPanelBox>
        <AuthoredFileWriteRecovery
          kind={kind}
          saveError={editor.saveError}
          conflict={editor.conflict}
          onReload={handleReload}
        />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4" data-testid={`agent-${kind}-editor`}>
      {editor.guardDialog}
      {headerSlot}
      <AgentPanelBox>
        <div className="flex flex-wrap items-center gap-2 border-b border-line-soft px-4 py-2.5">
          <Pill
            mono
            tone={
              editor.status === "valid"
                ? "success"
                : editor.status === "invalid"
                  ? "danger"
                  : "neutral"
            }
          >
            {editor.status}
          </Pill>
          {editor.payload && "enabled" in editor.payload ? (
            <Pill mono tone={editor.payload.enabled ? "success" : "neutral"}>
              {editor.payload.enabled ? "enabled" : "disabled"}
            </Pill>
          ) : null}
          {editor.payload?.source_path ? (
            <code className="font-mono text-badge tracking-mono text-muted">
              {editor.payload.source_path}
            </code>
          ) : null}
        </div>

        <Textarea
          variant="mono"
          value={editor.draft}
          onChange={event => editor.setDraft(event.target.value)}
          disabled={editor.saving}
          className="min-h-64 rounded-none border-0 border-b border-line-soft"
          data-testid={`agent-${kind}-textarea`}
          aria-label={`${editor.fileLabel} body`}
        />

        {editor.diagnostics.length > 0 ? (
          <ul className="flex flex-col gap-1 px-4 py-3" data-testid={`agent-${kind}-diagnostics`}>
            {editor.diagnostics.map(item => (
              <li
                key={`${item.source_path ?? "unknown"}:${item.line ?? "unknown"}:${item.message}`}
                className="text-small-body text-danger"
              >
                {item.message}
                {item.line != null ? (
                  <span className="ml-2 font-mono text-muted">line {item.line}</span>
                ) : null}
                {item.source_path ? (
                  <code className="ml-2 font-mono text-muted">{item.source_path}</code>
                ) : null}
              </li>
            ))}
          </ul>
        ) : null}

        <AuthoredFileWriteRecovery
          kind={kind}
          saveError={editor.saveError}
          conflict={editor.conflict}
          onReload={handleReload}
        />

        <div className="flex flex-wrap items-center gap-2 px-4 py-3">
          <Button
            type="button"
            size="sm"
            variant="ghost"
            onClick={() => void editor.handleValidate()}
            disabled={editor.validating || editor.saving}
            data-testid={`agent-${kind}-validate`}
          >
            {editor.validating ? <Spinner className="size-3" /> : null}
            {editor.validating ? "Validating…" : "Validate"}
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
            disabled={!editor.dirty || editor.saving}
            data-testid={`agent-${kind}-save`}
          >
            {editor.saving ? <Spinner className="size-3" /> : null}
            {editor.saving ? "Saving…" : "Save"}
          </Button>
        </div>

        {editor.showHistory ? (
          <div
            className="border-t border-line-soft px-4 py-3"
            data-testid={`agent-${kind}-history`}
          >
            {editor.revisions.length === 0 ? (
              <p className="text-small-body text-muted">No revisions yet.</p>
            ) : (
              <ul className="flex flex-col gap-2">
                {editor.revisions.map(revision => {
                  const id = "id" in revision ? String(revision.id) : "";
                  const created =
                    "created_at" in revision && typeof revision.created_at === "string"
                      ? revision.created_at
                      : "";
                  return (
                    <li
                      key={id || created}
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
                        onClick={() => void editor.handleRestore(id)}
                        disabled={!id || editor.saving}
                        data-testid={`agent-${kind}-restore-${id}`}
                      >
                        Restore
                      </Button>
                    </li>
                  );
                })}
              </ul>
            )}
          </div>
        ) : null}
      </AgentPanelBox>
      <span className="sr-only" aria-live="polite">
        {editor.saving
          ? `Saving ${editor.fileLabel}`
          : editor.validating
            ? `Validating ${editor.fileLabel}`
            : ""}
      </span>
    </div>
  );
}
