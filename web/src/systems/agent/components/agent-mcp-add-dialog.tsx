import {
  Alert,
  AlertDescription,
  Button,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  Input,
  Label,
  NativeSelect,
  NativeSelectOption,
} from "@compozy/ui";
import { MCPServerEditor, toRequest } from "@/systems/settings";
import { useAgentMCPAddDialog } from "../hooks/use-agent-mcp-add-dialog";

/** Creates the per-mount controller for staging an agent MCP attachment. */
export function AgentMCPAddDialog(props: Parameters<typeof useAgentMCPAddDialog>[0]) {
  return <AgentMCPAddDialogView {...useAgentMCPAddDialog(props)} />;
}

/** Stages configured name references or new connections through the shared editor. */
export function AgentMCPAddDialogView({
  mode,
  onNewServer,
  draft,
  onDraftChange,
  selected,
  onSelectServer,
  headers,
  onHeaderAdd,
  onHeaderChange,
  onHeaderRemove,
  servers,
  vault,
  available,
  validation,
  nameConflict,
  errors,
  validHeaders,
  headerError,
  headerErrors,
  agent,
  onAdd,
  onClose,
}: ReturnType<typeof useAgentMCPAddDialog>) {
  if (mode === "new")
    return (
      <MCPServerEditor
        open
        mode="create"
        draft={draft}
        scope={agent.origin === "workspace" ? "workspace" : "user"}
        target="sidecar"
        availableTargets={["sidecar"]}
        entry={null}
        errors={errors}
        isValid={validation.valid && !nameConflict && validHeaders}
        isSaving={false}
        saveError={null}
        vaultInventory={
          vault.isPending
            ? { status: "loading" }
            : vault.error
              ? { status: "error", message: vault.error.message, retry: () => void vault.refetch() }
              : {
                  status: "ready",
                  refs: (vault.data ?? []).flatMap(secret =>
                    secret.ref.startsWith("vault:mcp/shared/") ? [secret.ref] : []
                  ),
                }
        }
        onChange={onDraftChange}
        onTargetChange={() => {}}
        onClose={onClose}
        onSave={() => {
          const request = toRequest(draft);
          onAdd({
            ...request.server,
            ...(request.secret_values?.secret_env
              ? { secret_env_values: request.secret_values.secret_env }
              : {}),
            ...(draft.transport === "http" && headers.length
              ? {
                  secret_header_values: Object.fromEntries(
                    headers.map(header => [header.key.trim(), header.value])
                  ),
                }
              : {}),
          });
        }}
      >
        {draft.transport === "http" ? (
          <AgentMCPHeaderFields
            headers={headers}
            onHeaderAdd={onHeaderAdd}
            onHeaderChange={onHeaderChange}
            onHeaderRemove={onHeaderRemove}
            headerError={headerError}
            headerErrors={headerErrors}
          />
        ) : null}
      </MCPServerEditor>
    );

  return (
    <Dialog
      open
      onOpenChange={open => {
        if (!open) onClose();
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add MCP server</DialogTitle>
        </DialogHeader>
        {servers.error ? (
          <Alert variant="destructive">
            <AlertDescription>{servers.error.message}</AlertDescription>
          </Alert>
        ) : null}
        <Label htmlFor="agent-mcp-existing">Configured server</Label>
        <NativeSelect
          id="agent-mcp-existing"
          value={selected}
          disabled={servers.isPending}
          onChange={event => onSelectServer(event.target.value)}
        >
          <NativeSelectOption value="">Choose a server</NativeSelectOption>
          {available.map(server => (
            <NativeSelectOption key={server.name} value={server.name}>
              {server.name} · {server.scope}
            </NativeSelectOption>
          ))}
        </NativeSelect>
        <Button
          type="button"
          disabled={!available.some(server => server.name === selected)}
          onClick={() => onAdd({ name: selected, reference: true })}
        >
          Attach server
        </Button>
        <Button type="button" variant="secondary" onClick={onNewServer}>
          Add new server
        </Button>
      </DialogContent>
    </Dialog>
  );
}

/** Edits write-only header rows and displays the owning policy errors. */
function AgentMCPHeaderFields({
  headers,
  onHeaderAdd,
  onHeaderChange,
  onHeaderRemove,
  headerError,
  headerErrors,
}: Pick<
  ReturnType<typeof useAgentMCPAddDialog>,
  "headers" | "onHeaderAdd" | "onHeaderChange" | "onHeaderRemove" | "headerError" | "headerErrors"
>) {
  return (
    <div className="flex flex-col gap-3">
      <Label>Header credentials</Label>
      {headerError ? (
        <p role="alert" className="text-caption text-destructive">
          {headerError}
        </p>
      ) : null}
      {headers.map((header, index) => (
        <div className="flex flex-col gap-2" key={header.id}>
          <div className="flex gap-2">
            <Input
              aria-label={`Header name ${index + 1}`}
              value={header.key}
              onChange={event => onHeaderChange(header.id, { key: event.target.value })}
            />
            <Input
              aria-label={`Header secret ${index + 1}`}
              type="password"
              autoComplete="new-password"
              value={header.value}
              onChange={event => onHeaderChange(header.id, { value: event.target.value })}
            />
            <Button type="button" variant="ghost" onClick={() => onHeaderRemove(header.id)}>
              Remove
            </Button>
          </div>
          {headerErrors[index] ? (
            <p role="alert" className="text-caption text-destructive">
              {headerErrors[index]}
            </p>
          ) : null}
        </div>
      ))}
      <Button type="button" variant="secondary" onClick={onHeaderAdd}>
        Add header credential
      </Button>
      <p className="text-caption text-muted-foreground">
        Credentials are saved in Vault when you save this agent.
      </p>
    </div>
  );
}
