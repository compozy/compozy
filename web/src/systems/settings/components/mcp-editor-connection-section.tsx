import { useState } from "react";

import {
  Button,
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
  FormSection,
  Input,
  KindIcon,
  Pill,
  RadioCardGroup,
} from "@compozy/ui";
import { ClipboardPaste, Globe, KeyRound, Plug, Terminal } from "@compozy/ui/icons";

import type { MCPDraft, MCPDraftErrors, MCPTransport } from "../lib/mcp-editor-model";
import {
  applyParsedServer,
  applySourceLine,
  mcpSourceLine,
  parseMCPServersJSON,
  suggestMCPName,
} from "../lib/mcp-editor-paste";

import { MCPEditorJSONPaste } from "./mcp-editor-json-paste";

/** Required fields whose errors wait until the operator has touched them. */
export type MCPRequiredField = "name" | "command" | "url";

export interface MCPEditorConnectionSectionProps {
  draft: MCPDraft;
  errors: MCPDraftErrors;
  isCreate: boolean;
  /** The transport the person picked; `null` lets the source line decide. */
  picked: MCPTransport | null;
  /** The name follows the source until the person types one. */
  autoName: boolean;
  onChange: (updater: (draft: MCPDraft) => MCPDraft) => void;
  /** A card click: picks the transport and replaces the launch fields. */
  onPick: (transport: MCPTransport) => void;
  /** Pasted JSON already set the draft's transport; record it as picked. */
  onAdopt: (transport: MCPTransport) => void;
  onTouch: (field: MCPRequiredField) => void;
}

function hasSource(draft: MCPDraft): boolean {
  return mcpSourceLine(draft).trim() !== "";
}

/**
 * "How it connects": Remote server or Local program, then one source field. Until a card is
 * picked the field takes either a URL or a command and the line decides; "Paste JSON instead"
 * reads the `mcpServers` block from a server's docs. Switching transport replaces the launch
 * configuration — `toRequest` omits the other branch's fields entirely.
 */
export function MCPEditorConnectionSection({
  draft,
  errors,
  isCreate,
  picked,
  autoName,
  onChange,
  onPick,
  onAdopt,
  onTouch,
}: MCPEditorConnectionSectionProps) {
  const line = mcpSourceLine(draft);
  // The field keeps what was typed ("npx " mid-word); it follows the draft only when another
  // control (args, JSON paste, a transport switch) changes the launch fields.
  const [text, setText] = useState(line);
  const [syncedLine, setSyncedLine] = useState(line);
  if (line !== syncedLine) {
    setSyncedLine(line);
    if (mcpSourceLine(applySourceLine(draft, text, draft.transport)) !== line) setText(line);
  }
  // The JSON being pasted, or `null` while the one-line source field shows.
  const [pasting, setPasting] = useState<string | null>(null);

  const shown: MCPTransport | null = picked ?? (hasSource(draft) ? draft.transport : null);
  const isRemote = draft.transport === "http";
  const field: MCPRequiredField = isRemote ? "url" : "command";
  const error = isRemote ? errors.url : errors.command;

  const writeLine = (next: string) => {
    setText(next);
    onTouch(field);
    onChange(current => {
      const applied = applySourceLine(current, next, picked);
      return autoName ? { ...applied, name: suggestMCPName(applied) } : applied;
    });
  };

  return (
    <FormSection data-testid="settings-mcp-editor-connection" icon={Plug} title="How it connects">
      <RadioCardGroup
        aria-label="How it connects"
        className="grid-cols-2"
        data-testid="settings-mcp-servers-editor-transport"
        iconWellSize="lg"
        onValueChange={value => onPick(value)}
        options={[
          {
            value: "http",
            icon: Globe,
            title: "Remote server",
            "data-testid": "settings-mcp-servers-editor-transport-remote",
          },
          {
            value: "stdio",
            icon: Terminal,
            title: "Local program",
            "data-testid": "settings-mcp-servers-editor-transport-stdio",
          },
        ]}
        value={shown}
      />

      {pasting !== null ? (
        <MCPEditorJSONPaste
          autoName={autoName}
          initialText={pasting}
          onAdopt={onAdopt}
          onChange={onChange}
          onDone={() => setPasting(null)}
        />
      ) : (
        <Field data-invalid={error ? "true" : undefined}>
          <div className="flex items-center justify-between gap-2">
            <FieldLabel htmlFor="mcp-editor-source">
              {shown === null ? "URL or command" : shown === "http" ? "Server URL" : "Command"}
            </FieldLabel>
            {isCreate ? (
              <Button
                className="h-auto px-0"
                data-testid="settings-mcp-servers-editor-paste-json"
                onClick={() => setPasting("")}
                size="xs"
                type="button"
                variant="link"
              >
                <ClipboardPaste data-icon="inline-start" />
                Paste JSON instead
              </Button>
            ) : null}
          </div>
          <Input
            aria-invalid={error ? true : undefined}
            className="font-mono"
            data-testid={
              isRemote
                ? "settings-mcp-servers-editor-url-input"
                : "settings-mcp-servers-editor-command-input"
            }
            id="mcp-editor-source"
            onBlur={() => onTouch(field)}
            onChange={event => writeLine(event.target.value)}
            onPaste={event => {
              const pasted = event.clipboardData.getData("text").trim();
              if (isCreate && pasted.startsWith("{")) {
                event.preventDefault();
                const parsed = parseMCPServersJSON(pasted);
                const only = parsed.ok && parsed.servers.length === 1 ? parsed.servers[0] : null;
                if (only) {
                  onAdopt(only.transport);
                  onChange(current => applyParsedServer(current, only, !autoName));
                }
                setPasting(pasted);
              }
            }}
            placeholder={
              shown === null ? "https://… or npx …" : shown === "http" ? "https://…" : "npx …"
            }
            value={text}
          />
          {error ? <FieldError>{error}</FieldError> : null}
          {errors.remoteFields ? (
            <FieldError data-testid="settings-mcp-editor-remote-error">
              {errors.remoteFields}
            </FieldError>
          ) : null}
          {shown === null ? (
            <FieldDescription>Copy it from the server's docs or README.</FieldDescription>
          ) : null}
        </Field>
      )}

      {hasSource(draft) ? <MCPServerPreview draft={draft} /> : null}
    </FormSection>
  );
}

/** What is about to be added, as the list will show it. */
function MCPServerPreview({ draft }: { draft: MCPDraft }) {
  const remote = draft.transport === "http";
  const keys = draft.secretEnv.filter(entry => entry.key.trim()).length;
  return (
    <div
      className="flex items-start gap-3 rounded-lg bg-sunken p-3 shadow-hairline-inset"
      data-testid="settings-mcp-servers-editor-preview"
    >
      <KindIcon aria-hidden="true" icon={remote ? Globe : Terminal} size="sm" tone="well" />
      <div className="flex min-w-0 flex-1 flex-col gap-1.5">
        <div className="flex min-w-0 flex-col">
          <span className="truncate text-small-body font-medium text-fg-strong">
            {draft.name.trim() || "Unnamed server"}
          </span>
          <span className="truncate font-mono text-mono-id text-faint">{mcpSourceLine(draft)}</span>
        </div>
        <div className="flex flex-wrap gap-1">
          {remote ? (
            <Pill size="xs" tone="neutral">
              <KeyRound aria-hidden="true" className="size-3 text-faint" />
              Sign in after adding, if it asks
            </Pill>
          ) : keys > 0 ? (
            <Pill size="xs" tone="neutral">
              <KeyRound aria-hidden="true" className="size-3 text-faint" />
              {keys === 1 ? "Needs 1 key" : `Needs ${keys} keys`}
            </Pill>
          ) : null}
          <Pill size="xs" tone="neutral">
            <Plug aria-hidden="true" className="size-3 text-faint" />
            Tools appear once it connects
          </Pill>
        </div>
      </div>
    </div>
  );
}
