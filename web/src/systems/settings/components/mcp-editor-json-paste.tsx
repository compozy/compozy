import { useState } from "react";

import {
  Button,
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
  RadioCardGroup,
  Textarea,
} from "@compozy/ui";
import { FileText, Globe, Terminal } from "@compozy/ui/icons";

import type { MCPDraft, MCPTransport } from "../lib/mcp-editor-model";
import {
  applyParsedServer,
  parseMCPServersJSON,
  type MCPJSONParseResult,
  type ParsedMCPServer,
} from "../lib/mcp-editor-paste";

export interface MCPEditorJSONPasteProps {
  /** JSON already pasted into the source field, applied before this view opened. */
  initialText: string;
  autoName: boolean;
  onChange: (updater: (draft: MCPDraft) => MCPDraft) => void;
  /** Records the pasted server's transport as the picked one, without touching the draft. */
  onAdopt: (transport: MCPTransport) => void;
  onDone: () => void;
}

/** Fill the draft from one pasted server; the caller's single draft update for this event. */
function adoptPastedServer(
  server: ParsedMCPServer,
  autoName: boolean,
  onChange: MCPEditorJSONPasteProps["onChange"],
  onAdopt: MCPEditorJSONPasteProps["onAdopt"]
) {
  onAdopt(server.transport);
  onChange(current => applyParsedServer(current, server, !autoName));
}

/** Parse pasted JSON and fill the draft when it holds exactly one server. */
function adoptPastedJSON(
  text: string,
  autoName: boolean,
  onChange: MCPEditorJSONPasteProps["onChange"],
  onAdopt: MCPEditorJSONPasteProps["onAdopt"]
): MCPJSONParseResult | null {
  if (!text.trim()) return null;
  const result = parseMCPServersJSON(text);
  if (result.ok && result.servers.length === 1) {
    adoptPastedServer(result.servers[0]!, autoName, onChange, onAdopt);
  }
  return result;
}

function feedback(result: MCPJSONParseResult | null, chosen: ParsedMCPServer | null): string {
  if (!result) return "Paste the mcpServers block from the server's docs or README.";
  if (!result.ok) return "";
  const count = result.servers.length;
  const ignored = chosen?.ignored.includes("headers") ? " Its headers weren't copied." : "";
  if (count === 1) return `Found 1 server in the JSON and filled in the rest.${ignored}`;
  if (!chosen) return `Found ${count} servers. Pick the one to add.`;
  return `Found ${count} servers and filled in ${chosen.name}.${ignored}`;
}

/**
 * "Config from the docs": accepts the `mcpServers` JSON people copy from a server's README. One
 * server fills the form at once; several let the person pick which one this dialog adds.
 */
export function MCPEditorJSONPaste({
  initialText,
  autoName,
  onChange,
  onAdopt,
  onDone,
}: MCPEditorJSONPasteProps) {
  const [text, setText] = useState(initialText);
  const result = text.trim() ? parseMCPServersJSON(text) : null;
  const servers = result?.ok ? result.servers : [];
  const [chosenIndex, setChosenIndex] = useState<number | null>(null);
  const chosen = servers.length === 1 ? servers[0]! : (servers[chosenIndex ?? -1] ?? null);
  const error =
    result && !result.ok
      ? result.reason === "invalid-json"
        ? "This isn't valid JSON yet."
        : "No MCP servers in this JSON."
      : null;

  return (
    <div className="flex flex-col gap-4" data-testid="settings-mcp-servers-editor-json">
      <Field data-invalid={error ? "true" : undefined}>
        <div className="flex items-center justify-between gap-2">
          <FieldLabel htmlFor="mcp-editor-json">Config from the docs</FieldLabel>
          <Button
            className="h-auto px-0"
            data-testid="settings-mcp-servers-editor-paste-json-cancel"
            onClick={onDone}
            size="xs"
            type="button"
            variant="link"
          >
            Type a URL or command instead
          </Button>
        </div>
        <Textarea
          aria-invalid={error ? true : undefined}
          className="min-h-0 resize-none"
          data-testid="settings-mcp-servers-editor-json-input"
          id="mcp-editor-json"
          onChange={event => {
            const next = event.target.value;
            setText(next);
            setChosenIndex(null);
            adoptPastedJSON(next, autoName, onChange, onAdopt);
          }}
          placeholder={'{\n  "mcpServers": { … }\n}'}
          rows={8}
          spellCheck={false}
          value={text}
          variant="mono"
        />
        {error ? (
          <FieldError>{error}</FieldError>
        ) : (
          <FieldDescription
            className="flex items-center gap-1.5"
            data-testid="settings-mcp-servers-editor-json-feedback"
          >
            {result ? <FileText aria-hidden="true" className="size-3.5 shrink-0" /> : null}
            {feedback(result, chosen)}
          </FieldDescription>
        )}
      </Field>
      {servers.length > 1 ? (
        <RadioCardGroup
          aria-label="Server to add"
          className="grid-cols-2"
          data-testid="settings-mcp-servers-editor-json-servers"
          onValueChange={value => {
            const index = Number(value);
            const server = servers[index];
            if (!server) return;
            setChosenIndex(index);
            adoptPastedServer(server, autoName, onChange, onAdopt);
          }}
          options={servers.map((server, index) => ({
            value: String(index),
            icon: server.transport === "http" ? Globe : Terminal,
            title: server.name || `Server ${index + 1}`,
            titleClassName: "min-w-0 flex-1 truncate font-mono",
          }))}
          value={chosenIndex === null ? null : String(chosenIndex)}
        />
      ) : null}
    </div>
  );
}
