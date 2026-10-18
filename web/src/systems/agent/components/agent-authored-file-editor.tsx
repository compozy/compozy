import type { ReactNode } from "react";

import { Panel, Textarea } from "@compozy/ui";

import {
  useAgentAuthoredFileEditor,
  type AuthoredFileKind,
} from "../hooks/use-agent-authored-file-editor";
import {
  AuthoredFileActions,
  AuthoredFileDiagnostics,
  AuthoredFileHistory,
  AuthoredFileLiveStatus,
  AuthoredFileLoadError,
  AuthoredFileLoading,
  AuthoredFileMeta,
  AuthoredFileMissing,
  AuthoredFileWriteRecovery,
} from "./agent-authored-file-parts";
import type {
  AgentHeartbeatHistoryResponse,
  AgentHeartbeatPayload,
  AgentSoulHistoryResponse,
  AgentSoulPayload,
} from "../types";

export type { AuthoredFileKind };

interface AgentAuthoredFileEditorBaseProps {
  resourceKey: string;
  isLoading: boolean;
  isError: boolean;
  onValidate: (body: string) => Promise<{
    diagnostics?: Array<{ message: string; line?: number; source_path?: string }>;
    validation_status?: string;
  }>;
  onRetry: () => void;
  /** Extra content rendered above the editor (heartbeat status/wake). */
  headerSlot?: ReactNode;
}

export type AgentAuthoredFileEditorProps =
  | (AgentAuthoredFileEditorBaseProps & {
      kind: "soul";
      payload: AgentSoulPayload | undefined;
      history: AgentSoulHistoryResponse | undefined;
      onSave: (body: string, expectedDigest: string) => Promise<AgentSoulPayload>;
      onRestore: (revisionId: string, expectedDigest: string) => Promise<AgentSoulPayload>;
    })
  | (AgentAuthoredFileEditorBaseProps & {
      kind: "heartbeat";
      payload: AgentHeartbeatPayload | undefined;
      history: AgentHeartbeatHistoryResponse | undefined;
      onSave: (body: string, expectedDigest: string) => Promise<AgentHeartbeatPayload>;
      onRestore: (revisionId: string, expectedDigest: string) => Promise<AgentHeartbeatPayload>;
    });

export function AgentAuthoredFileEditor(props: AgentAuthoredFileEditorProps) {
  const { kind, isLoading, isError, onRetry, headerSlot } = props;
  const editor = useAgentAuthoredFileEditor(props);
  const saving = editor.phase === "saving";
  const validating = editor.phase === "validating";
  const conflict = editor.phase === "conflict";
  const saveError = conflict || editor.phase === "failed" ? editor.error : null;
  const handleReload = () => {
    editor.handleReload();
    onRetry();
  };
  const recovery = (
    <AuthoredFileWriteRecovery
      kind={kind}
      saveError={saveError}
      conflict={conflict}
      onReload={handleReload}
    />
  );

  if (isLoading) {
    return <AuthoredFileLoading kind={kind} />;
  }

  if (isError) {
    return <AuthoredFileLoadError kind={kind} fileLabel={editor.fileLabel} onRetry={onRetry} />;
  }

  if (editor.isMissing) {
    return (
      <AuthoredFileMissing
        kind={kind}
        fileLabel={editor.fileLabel}
        saving={saving}
        onCreate={() => void editor.handleCreate()}
        headerSlot={headerSlot}
        recovery={recovery}
      />
    );
  }

  return (
    <div className="flex flex-col gap-4" data-testid={`agent-${kind}-editor`}>
      {editor.guardDialog}
      {headerSlot}
      <Panel bodyClassName="p-0">
        <AuthoredFileMeta status={editor.status} payload={editor.payload} />

        <Textarea
          variant="mono"
          value={editor.draft}
          onChange={event => editor.setDraft(event.target.value)}
          disabled={saving}
          className="min-h-64 rounded-none border-0 border-b border-line-soft"
          data-testid={`agent-${kind}-textarea`}
          aria-label={`${editor.fileLabel} body`}
        />

        <AuthoredFileDiagnostics kind={kind} diagnostics={editor.diagnostics} />

        {recovery}

        <AuthoredFileActions kind={kind} editor={editor} saving={saving} validating={validating} />

        {editor.showHistory ? (
          <AuthoredFileHistory
            kind={kind}
            revisions={editor.revisions}
            saving={saving}
            onRestore={revisionId => void editor.handleRestore(revisionId)}
          />
        ) : null}
      </Panel>
      <AuthoredFileLiveStatus
        fileLabel={editor.fileLabel}
        saving={saving}
        validating={validating}
      />
    </div>
  );
}
