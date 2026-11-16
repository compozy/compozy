import { FolderOpen, Globe, KeyRound, Plus, UserRound } from "@compozy/ui/icons";
import { type ReactNode, useState } from "react";

import {
  Alert,
  AlertDescription,
  Button,
  Field,
  FieldError,
  FieldLabel,
  Input,
  KindIcon,
  PopoverPanel,
  RadioCardGroup,
  Spinner,
} from "@compozy/ui";

import type { MCPCopyFailure } from "../../lib/mcp-runtime-view-model";
import type { RuntimeMCPImportRequest, RuntimeMCPImportScope, RuntimeMCPServer } from "../../types";

type CopyScope = Exclude<RuntimeMCPImportScope, "agent">;

export interface MCPCopyPanelProps {
  server: RuntimeMCPServer;
  /** The coding agent's display name ("Claude Code"). */
  runtimeName: string;
  /** The coding agent's mark, set on the header's identity well. */
  mark?: ReactNode;
  /** The coding agent can turn its own servers off for CompozyOS sessions. */
  canTurnOff: boolean;
  /** Named profile of the shell's lens; null on the default profile (no "This profile"). */
  profile: string | null;
  /** Project in view; null disables "Only this project". */
  workspaceId: string | null;
  submitting: boolean;
  failure: MCPCopyFailure | null;
  onSubmit: (request: RuntimeMCPImportRequest) => void;
  onClose: () => void;
}

const NAME_TAKEN = "mcp_import_name_taken";
const SOURCE_GONE = "mcp_server_not_found";

function copyRequest(
  scope: CopyScope,
  name: string,
  server: RuntimeMCPServer,
  profile: string | null,
  workspaceId: string | null
): RuntimeMCPImportRequest {
  return {
    scope,
    ...(name !== server.name ? { name } : {}),
    ...(scope === "profile" && profile ? { profile } : {}),
    // The project in view is where the inventory found the source, whatever the destination: the
    // daemon resolves the source in it and only files a Project copy there.
    ...(workspaceId ? { workspace_id: workspaceId } : {}),
  };
}

function CopyPanelFooter({
  submitting,
  blocked,
  onCancel,
  onSubmit,
}: {
  submitting: boolean;
  blocked: boolean;
  onCancel: () => void;
  onSubmit: () => void;
}) {
  return (
    <>
      <Button
        type="button"
        size="sm"
        variant="ghost"
        className="ml-auto"
        disabled={submitting}
        onClick={onCancel}
      >
        Cancel
      </Button>
      <Button
        type="button"
        size="sm"
        disabled={blocked}
        onClick={onSubmit}
        data-testid="mcp-copy-panel-submit"
      >
        {submitting ? <Spinner className="size-3.5" /> : null}
        {submitting ? "Adding…" : "Add to CompozyOS"}
      </Button>
    </>
  );
}

function CopyScopePicker({
  scope,
  profile,
  workspaceId,
  submitting,
  onChange,
}: {
  scope: CopyScope;
  profile: string | null;
  workspaceId: string | null;
  submitting: boolean;
  onChange: (scope: CopyScope) => void;
}) {
  return (
    <RadioCardGroup<CopyScope>
      aria-label="Use it in"
      className="flex flex-col gap-1"
      disabled={submitting}
      value={scope}
      onValueChange={onChange}
      options={[
        { value: "user", icon: Globe, title: "All projects", description: "Any session" },
        ...(profile
          ? [
              {
                value: "profile" as const,
                icon: UserRound,
                title: "This profile",
                description: `Sessions in ${profile}`,
              },
            ]
          : []),
        {
          value: "workspace",
          icon: FolderOpen,
          title: "Only this project",
          description: workspaceId ? "Sessions in this project" : "Pick a project first",
          disabled: !workspaceId,
        },
      ]}
    />
  );
}

function CopyNameField({
  name,
  submitting,
  takenMessage,
  onChange,
}: {
  name: string;
  submitting: boolean;
  /** The daemon's "name taken" message; set while the name must change. */
  takenMessage: string | undefined;
  onChange: (name: string) => void;
}) {
  const taken = takenMessage !== undefined;
  return (
    <Field data-invalid={taken ? "true" : undefined}>
      <FieldLabel htmlFor="mcp-copy-panel-name">Name in CompozyOS</FieldLabel>
      <Input
        aria-invalid={taken || undefined}
        className="font-mono"
        disabled={submitting}
        id="mcp-copy-panel-name"
        onChange={event => onChange(event.target.value)}
        value={name}
      />
      {taken ? <FieldError>{takenMessage}. Pick another name.</FieldError> : null}
    </Field>
  );
}

function CopyKeysNote({ keys }: { keys: string[] }) {
  if (keys.length === 0) return null;
  return (
    <p className="flex items-start gap-2 rounded-md bg-sunken px-2.5 py-2 text-meta text-muted-foreground shadow-hairline-inset">
      <KeyRound aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-faint" />
      <span>
        {keys.length === 1 ? "Its key " : "Its keys "}
        <span className="font-mono text-fg">{keys.join(", ")}</span>{" "}
        {keys.length === 1 ? "moves" : "move"} into your vault. Nobody sees the value.
      </span>
    </p>
  );
}

/**
 * Add a coding agent's server to CompozyOS: where every session can use it, and its keys moving
 * into the vault. The agent's own file is never touched. A taken name keeps the panel open with
 * the name to change; a server gone from its file says so and blocks the copy.
 */
export function MCPCopyPanel({
  server,
  runtimeName,
  mark,
  canTurnOff,
  profile,
  workspaceId,
  submitting,
  failure,
  onSubmit,
  onClose,
}: MCPCopyPanelProps) {
  const [scope, setScope] = useState<CopyScope>("user");
  const [name, setName] = useState(server.name);
  const nameTaken = failure?.code === NAME_TAKEN;
  // The name stays editable once a taken name asked for another one.
  const showName = nameTaken || name !== server.name;
  const sourceGone = failure?.code === SOURCE_GONE;
  const trimmed = name.trim();
  const blocked = submitting || sourceGone || trimmed === "";
  const keys = [...server.env_keys, ...server.header_keys];
  const remote = server.transport !== "stdio";
  const close = () => {
    if (!submitting) onClose();
  };
  return (
    <PopoverPanel
      data-testid="mcp-copy-panel"
      {...(mark
        ? { mark: <KindIcon aria-hidden="true" tone="well" size="sm" glyph={mark} /> }
        : { icon: Plus })}
      title={`Add ${server.name} to CompozyOS`}
      subtitle={`From ${runtimeName} · ${remote ? "Remote" : "Runs on this computer"}`}
      onClose={close}
      footer={
        <CopyPanelFooter
          blocked={blocked}
          onCancel={close}
          onSubmit={() => onSubmit(copyRequest(scope, trimmed, server, profile, workspaceId))}
          submitting={submitting}
        />
      }
    >
      {failure && !nameTaken ? (
        <Alert role="alert" variant={sourceGone ? "warning" : "destructive"}>
          <AlertDescription>{failure.message}</AlertDescription>
        </Alert>
      ) : null}
      <p className="text-meta text-muted-foreground">
        Every CompozyOS session can use it, whichever agent runs it. {runtimeName} keeps its own
        copy.
        {canTurnOff ? " You can turn its servers off for CompozyOS sessions." : ""}
      </p>
      <CopyScopePicker
        onChange={setScope}
        profile={profile}
        scope={scope}
        submitting={submitting}
        workspaceId={workspaceId}
      />
      {showName ? (
        <CopyNameField
          name={name}
          onChange={setName}
          submitting={submitting}
          takenMessage={nameTaken ? failure?.message : undefined}
        />
      ) : null}
      <CopyKeysNote keys={keys} />
    </PopoverPanel>
  );
}
