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

export function AgentMCPAddDialog(props: Parameters<typeof useAgentMCPAddDialog>[0]) {
  return <AgentMCPAddDialogView {...useAgentMCPAddDialog(props)} />;
}

export function AgentMCPAddDialogView({
  mode,
  setMode,
  draft,
  setDraft,
  selected,
  setSelected,
  headers,
  setHeaders,
  servers,
  vault,
  available,
  validation,
  nameConflict,
  errors,
  validHeaders,
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
              : { status: "ready", refs: (vault.data ?? []).map(secret => secret.ref) }
        }
        onChange={setDraft}
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
          <div className="flex flex-col gap-3">
            <Label>Header credentials</Label>
            {headers.map((header, index) => (
              <div className="flex gap-2" key={header.id}>
                <Input
                  aria-label={`Header name ${index + 1}`}
                  value={header.key}
                  onChange={event =>
                    setHeaders(
                      headers.map((row, at) =>
                        at === index ? { ...row, key: event.target.value } : row
                      )
                    )
                  }
                />
                <Input
                  aria-label={`Header secret ${index + 1}`}
                  type="password"
                  autoComplete="new-password"
                  value={header.value}
                  onChange={event =>
                    setHeaders(
                      headers.map((row, at) =>
                        at === index ? { ...row, value: event.target.value } : row
                      )
                    )
                  }
                />
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => setHeaders(headers.filter((_, at) => at !== index))}
                >
                  Remove
                </Button>
              </div>
            ))}
            <Button
              type="button"
              variant="secondary"
              onClick={() =>
                setHeaders([
                  ...headers,
                  { id: crypto.randomUUID(), key: "Authorization", value: "" },
                ])
              }
            >
              Add header credential
            </Button>
            <p className="text-caption text-muted-foreground">
              Credentials are saved in Vault when you save this agent.
            </p>
          </div>
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
          onChange={event => setSelected(event.target.value)}
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
        <Button type="button" variant="secondary" onClick={() => setMode("new")}>
          Add new server
        </Button>
      </DialogContent>
    </Dialog>
  );
}
