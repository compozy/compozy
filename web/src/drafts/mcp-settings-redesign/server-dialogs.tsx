/**
 * Proposed MCP dialogs, built on the entity-dialog kit (identity well header, ruled sections,
 * consequence footer). Add/Edit: pick how it connects, paste the URL/command (or the JSON from the
 * docs), see a preview of what you're adding, then name, where, and a key only when needed.
 * Remove reads as a plain consequence. Extension servers ask only for what's missing.
 */
import {
  ClipboardPaste,
  FileText,
  FolderOpen,
  Globe,
  KeyRound,
  Plug,
  Plus,
  Puzzle,
  Terminal,
  Trash2,
} from "@compozy/ui/icons";
import { useState, type ComponentType, type ReactNode } from "react";

import {
  Button,
  ConfirmDialog,
  Dialog,
  DialogContent,
  dialogShellClass,
  Disclosure,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  Field,
  FieldDescription,
  FieldLabel,
  FormSection,
  Input,
  KindIcon,
  RadioCardGroup,
  SecretField,
  Textarea,
} from "@compozy/ui";

export type AddServerCase = "empty" | "remote" | "local" | "json" | "edit";

const SOURCE: Record<Exclude<AddServerCase, "empty">, string> = {
  remote: "https://mcp.linear.app/mcp",
  local: "npx -y @modelcontextprotocol/server-github",
  json: '{\n  "mcpServers": {\n    "github": {\n      "command": "npx",\n      "args": ["-y", "@modelcontextprotocol/server-github"],\n      "env": { "GITHUB_TOKEN": "" }\n    }\n  }\n}',
  edit: "npx -y @modelcontextprotocol/server-github",
};

function Chip({
  icon: Icon,
  children,
}: {
  icon: ComponentType<{ className?: string }>;
  children: ReactNode;
}) {
  return (
    <span className="inline-flex items-center gap-1 rounded-sm bg-surface-2 px-1.5 py-0.5 text-meta text-muted-foreground">
      <Icon aria-hidden="true" className="size-3 text-faint" />
      {children}
    </span>
  );
}

/** What the person is about to add, as it will appear in the list. */
function Preview({ remote, name, endpoint }: { remote: boolean; name: string; endpoint: string }) {
  return (
    <div className="flex items-start gap-3 rounded-lg bg-sunken p-3 shadow-hairline-inset">
      <KindIcon aria-hidden="true" icon={remote ? Globe : Terminal} tone="well" size="sm" />
      <div className="flex min-w-0 flex-1 flex-col gap-1.5">
        <div className="flex min-w-0 flex-col">
          <span className="truncate text-small-body font-medium text-fg-strong">{name}</span>
          <span className="truncate font-mono text-mono-id text-faint">{endpoint}</span>
        </div>
        <div className="flex flex-wrap gap-1">
          {remote ? (
            <Chip icon={KeyRound}>You'll sign in after adding</Chip>
          ) : (
            <Chip icon={KeyRound}>Needs 1 key</Chip>
          )}
          <Chip icon={Plug}>Tools appear once it connects</Chip>
        </div>
      </div>
    </div>
  );
}

function ConnectionSection({ kind }: { kind: AddServerCase }) {
  const remote = kind === "remote";
  const filled = kind !== "empty";
  return (
    <FormSection title="How it connects" icon={Plug}>
      <div className="flex flex-col gap-4">
        <RadioCardGroup
          aria-label="How it connects"
          className="grid-cols-2"
          iconWellSize="lg"
          value={kind === "empty" ? null : remote ? "remote" : "local"}
          onValueChange={() => undefined}
          options={[
            {
              value: "remote",
              icon: Globe,
              title: "Remote server",
            },
            {
              value: "local",
              icon: Terminal,
              title: "Local program",
            },
          ]}
        />
        <Field>
          <div className="flex items-center justify-between gap-2">
            <FieldLabel htmlFor="mcp-source">
              {kind === "json"
                ? "Config from the docs"
                : kind === "empty"
                  ? "URL or command"
                  : remote
                    ? "Server URL"
                    : "Command"}
            </FieldLabel>
            {kind === "json" ? null : (
              <Button type="button" size="xs" variant="link" className="h-auto px-0">
                <ClipboardPaste data-icon="inline-start" />
                Paste JSON instead
              </Button>
            )}
          </div>
          <Textarea
            id="mcp-source"
            variant="mono"
            rows={kind === "json" ? 8 : 1}
            defaultValue={filled ? SOURCE[kind] : ""}
            placeholder={kind === "empty" ? "https://… or npx …" : remote ? "https://…" : "npx …"}
            className="min-h-0 resize-none"
          />
          {kind === "json" ? (
            <FieldDescription className="flex items-center gap-1.5">
              <FileText aria-hidden="true" className="size-3.5" />
              Found 1 server in the JSON and filled in the rest.
            </FieldDescription>
          ) : kind === "empty" ? (
            <FieldDescription>Copy it from the server's docs or README.</FieldDescription>
          ) : null}
        </Field>
        {filled ? (
          <Preview
            remote={remote}
            name={remote ? "linear" : "github"}
            endpoint={remote ? SOURCE.remote : SOURCE.local}
          />
        ) : null}
      </div>
    </FormSection>
  );
}

function NameSection({ kind }: { kind: AddServerCase }) {
  const edit = kind === "edit";
  const name = kind === "remote" ? "linear" : kind === "empty" ? "" : "github";
  return (
    <FormSection title="Name and where to use it" icon={FolderOpen}>
      <div className="flex flex-col gap-4">
        <Field>
          <Input
            id="mcp-name"
            aria-label="Name"
            defaultValue={name}
            placeholder="Name, like linear"
            disabled={edit}
          />
          <FieldDescription>
            {edit
              ? "Agents know the server by this name, so it can't change."
              : "Agents see this name next to its tools."}
          </FieldDescription>
        </Field>
        <RadioCardGroup
          aria-label="Use it in"
          className="grid-cols-2"
          value="all"
          onValueChange={() => undefined}
          options={[
            { value: "all", icon: Globe, title: "All projects" },
            { value: "project", icon: FolderOpen, title: "Only compozy" },
          ]}
        />
      </div>
    </FormSection>
  );
}

function KeySection({ edit }: { edit: boolean }) {
  const [value, setValue] = useState("");
  return (
    <FormSection
      title="Key it needs"
      icon={KeyRound}
      rightLabel="1 required"
      description="Saved in your vault. Nobody sees it again, not even agents."
    >
      <SecretField
        id="mcp-key-github-token"
        label={<span className="font-mono">GITHUB_TOKEN</span>}
        description="A GitHub personal access token with repo access."
        required
        value={value}
        onValueChange={setValue}
        present={edit}
        presenceLabel={edit ? "Saved in your vault" : undefined}
        placeholder="ghp_…"
      />
    </FormSection>
  );
}

export function AddServerDialog({ kind }: { kind: AddServerCase }) {
  const edit = kind === "edit";
  const filled = kind !== "empty";
  const local = kind === "local" || kind === "json" || edit;
  return (
    <Dialog defaultOpen>
      <DialogContent
        unframed
        showCloseButton={false}
        className={`grid-rows-[auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("sm")}`}
      >
        <EntityDialogHeader
          icon={Plug}
          title={edit ? "Edit github" : "Add an MCP server"}
          description={
            edit
              ? "Changes reach every session on its next message."
              : "Give your agents new tools, like reading Linear issues or querying a database."
          }
          onClose={() => undefined}
        />
        <EntityDialogBody className="flex flex-col">
          <NameSection kind={kind} />
          <ConnectionSection kind={kind} />
          {local && filled ? <KeySection edit={edit} /> : null}
          {filled ? (
            <Disclosure variant="framed" label="More options" className="mt-6">
              <div className="flex flex-col gap-3 p-4">
                {local ? (
                  <Field>
                    <FieldLabel htmlFor="mcp-env">Other settings it reads</FieldLabel>
                    <Input id="mcp-env" placeholder="NAME=value" className="font-mono" />
                    <FieldDescription>Values that aren't secret, one per line.</FieldDescription>
                  </Field>
                ) : (
                  <Field>
                    <FieldLabel htmlFor="mcp-header">Custom header</FieldLabel>
                    <Input id="mcp-header" placeholder="X-Team: platform" className="font-mono" />
                    <FieldDescription>Only if the server's docs ask for one.</FieldDescription>
                  </Field>
                )}
              </div>
            </Disclosure>
          ) : null}
        </EntityDialogBody>
        <EntityDialogFooter
          hint={
            edit ? (
              <Button type="button" size="sm" variant="ghost" className="-ml-2 text-destructive">
                <Trash2 data-icon="inline-start" />
                Remove server
              </Button>
            ) : (
              "Agents get its tools on their next message."
            )
          }
          onCancel={() => undefined}
          primaryLabel={edit ? "Save changes" : "Add server"}
          primaryIcon={edit ? undefined : Plus}
          primaryDisabled={!filled}
        />
      </DialogContent>
    </Dialog>
  );
}

export function RemoveServerDialog({ shadowed = false }: { shadowed?: boolean }) {
  return (
    <ConfirmDialog
      defaultOpen
      tone="destructive"
      icon={Trash2}
      title="Remove sentry?"
      description="Sessions stop using it from their next message, and its saved sign-in is deleted. You can add it again anytime."
      note={
        shadowed
          ? "This project's .mcp.json also has a sentry server. That one takes over after you remove this one."
          : undefined
      }
      noteTone="info"
      cancelLabel="Cancel"
      confirmLabel="Remove server"
      onConfirm={() => undefined}
    />
  );
}

export function ExtensionServerDialog() {
  const [key, setKey] = useState("");
  return (
    <Dialog defaultOpen>
      <DialogContent
        unframed
        showCloseButton={false}
        className={`grid-rows-[auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("sm")}`}
      >
        <EntityDialogHeader
          icon={Puzzle}
          title="Set up Linear"
          description="The Linear extension added this server. It needs one thing from you before sessions can use it."
          onClose={() => undefined}
        />
        <EntityDialogBody className="flex flex-col">
          <FormSection title="What it needs" icon={KeyRound} rightLabel="1 missing">
            <div className="flex flex-col gap-4">
              <Field>
                <FieldLabel htmlFor="linear-workspace">Workspace</FieldLabel>
                <Input id="linear-workspace" defaultValue="compozy" />
                <FieldDescription>Already filled in by the extension.</FieldDescription>
              </Field>
              <SecretField
                id="linear-api-key"
                label="API key"
                description="Create one in Linear › Settings › API."
                required
                value={key}
                onValueChange={setKey}
                placeholder="lin_api_…"
              />
            </div>
          </FormSection>
          <Disclosure variant="framed" label="Technical details" className="mt-6">
            <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 p-4 text-meta">
              <dt className="text-faint">Address</dt>
              <dd className="truncate font-mono text-muted-foreground">
                https://mcp.linear.app/mcp
              </dd>
              <dt className="text-faint">Agents call it</dt>
              <dd className="truncate font-mono text-muted-foreground">mcp__linear__*</dd>
              <dt className="text-faint">Version</dt>
              <dd className="truncate font-mono text-muted-foreground">linear@1.4.0</dd>
            </dl>
          </Disclosure>
        </EntityDialogBody>
        <EntityDialogFooter
          hint={
            <Button type="button" size="sm" variant="ghost" className="-ml-2">
              Reset to extension defaults
            </Button>
          }
          onCancel={() => undefined}
          primaryLabel="Save"
          primaryDisabled={!key}
        />
      </DialogContent>
    </Dialog>
  );
}
