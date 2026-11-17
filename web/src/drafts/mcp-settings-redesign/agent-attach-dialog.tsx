/**
 * Proposed "give an agent MCP servers" picker (agent profile › Tools). Replaces the native select
 * of `name · scope` strings and the nested Settings editor with one searchable checklist of the
 * servers you already have, each with its identity and state, plus "Add a new server".
 */
import { Bot, Globe, Plus, Search, Terminal } from "@compozy/ui/icons";

import {
  Checkbox,
  cn,
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  Input,
  KindIcon,
  StatusDot,
  Button,
} from "@compozy/ui";

import { SERVERS, sourceLabel } from "./settings-fixtures";

const CHECKED = new Set(["linear", "sentry"]);

export function AgentAttachDialog() {
  return (
    <Dialog defaultOpen>
      <DialogContent
        unframed
        showCloseButton={false}
        className={`grid-rows-[auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("sm")}`}
      >
        <EntityDialogHeader
          icon={Bot}
          title="Tools for Release bot"
          description="Pick the MCP servers this agent can use. It gets them on its next message."
          onClose={() => undefined}
        />
        <EntityDialogBody className="flex flex-col gap-3">
          <div className="relative">
            <Search
              aria-hidden="true"
              className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-faint"
            />
            <Input aria-label="Search servers" placeholder="Search your servers" className="pl-8" />
          </div>
          <ul className="flex flex-col overflow-hidden rounded-lg bg-sunken shadow-hairline-inset">
            {SERVERS.map(server => {
              const checked = CHECKED.has(server.name);
              return (
                <li key={server.name} className="border-b border-line-soft last:border-b-0">
                  <label
                    className={cn(
                      "flex min-h-14 cursor-default items-center gap-3 px-3 py-2 hover:bg-surface-2/60",
                      checked && "bg-surface-2/40"
                    )}
                  >
                    <Checkbox defaultChecked={checked} />
                    <KindIcon
                      aria-hidden="true"
                      icon={server.transport === "remote" ? Globe : Terminal}
                      tone="well"
                      size="sm"
                    />
                    <span className="flex min-w-0 flex-1 flex-col">
                      <span className="truncate text-small-body font-medium text-fg">
                        {server.name}
                      </span>
                      <span className="truncate text-meta text-faint">{sourceLabel(server)}</span>
                    </span>
                    {server.status === "sign-in" ? (
                      <span className="flex shrink-0 items-center gap-1.5 text-meta text-muted-foreground">
                        <StatusDot tone="warning" size="sm" />
                        Needs sign-in
                      </span>
                    ) : server.tools.length > 0 ? (
                      <span className="shrink-0 text-meta text-faint tabular-nums">
                        {server.tools.length} tools
                      </span>
                    ) : null}
                  </label>
                </li>
              );
            })}
          </ul>
        </EntityDialogBody>
        <EntityDialogFooter
          hint={
            <Button type="button" size="sm" variant="ghost" className="-ml-2">
              <Plus data-icon="inline-start" />
              Add a new server
            </Button>
          }
          onCancel={() => undefined}
          primaryLabel="Save · 2 servers"
        />
      </DialogContent>
    </Dialog>
  );
}
